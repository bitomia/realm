package vm

import (
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"

	"github.com/bitomia/realm/common"
)

// dialLibvirt connects to a local libvirtd over its unix socket. A nil socket
// leaves the path to the dialer's own default (/var/run/libvirt/libvirt-sock).
func dialLibvirt(socket *string) (*libvirt.Libvirt, error) {
	var opts []dialers.LocalOption
	if socket != nil {
		opts = append(opts, dialers.WithSocket(*socket))
	}
	l := libvirt.NewWithDialer(dialers.NewLocal(opts...))
	if err := l.Connect(); err != nil {
		return nil, fmt.Errorf("libvirt: dial failed: %w", err)
	}
	return l, nil
}

func withLibvirt(socket *string, fn func(*libvirt.Libvirt) error) error {
	l, err := dialLibvirt(socket)
	if err != nil {
		return err
	}
	defer func() { _ = l.Disconnect() }()
	return fn(l)
}

func lookupDomain(l *libvirt.Libvirt, name string) (libvirt.Domain, bool, error) {
	d, err := l.DomainLookupByName(name)
	if err != nil {
		if libvirt.IsNotFound(err) {
			return libvirt.Domain{}, false, nil
		}
		return libvirt.Domain{}, false, err
	}
	return d, true, nil
}

// qemuCommandlineNS is the libvirt QEMU namespace required to pass arbitrary
const qemuCommandlineNS = "http://libvirt.org/schemas/domain/qemu/1.0"

// hostfwdBaseSlot is the pcie.0 slot the first passthrough NIC is pinned to.
const hostfwdBaseSlot = 0x1e

type xDomain struct {
	XMLName     xml.Name      `xml:"domain"`
	Type        string        `xml:"type,attr"`
	QemuXMLNS   string        `xml:"xmlns:qemu,attr,omitempty"`
	Name        string        `xml:"name"`
	Memory      *xMemory      `xml:"memory,omitempty"`
	VCPU        *xVCPU        `xml:"vcpu,omitempty"`
	OS          xOS           `xml:"os"`
	CPU         *xCPU         `xml:"cpu,omitempty"`
	SysInfo     *xSysInfo     `xml:"sysinfo,omitempty"`
	Features    *xFeatures    `xml:"features,omitempty"`
	Devices     xDevices      `xml:"devices"`
	QemuCmdline *xQemuCmdline `xml:"qemu:commandline,omitempty"`
}

type xQemuCmdline struct {
	Args []xQemuArg `xml:"qemu:arg"`
}

type xQemuArg struct {
	Value string `xml:"value,attr"`
}

type xMemory struct {
	Unit  string `xml:"unit,attr"`
	Value int    `xml:",chardata"`
}

type xVCPU struct {
	Value int `xml:",chardata"`
}

type xOS struct {
	Type   xOSType    `xml:"type"`
	Loader *xOSLoader `xml:"loader,omitempty"`
	NVRam  *xOSNVRam  `xml:"nvram,omitempty"`
	SMBIOS *xOSSMBIOS `xml:"smbios,omitempty"`
}

type xOSLoader struct {
	ReadOnly string `xml:"readonly,attr,omitempty"`
	Secure   string `xml:"secure,attr,omitempty"`
	Type     string `xml:"type,attr,omitempty"`
	Path     string `xml:",chardata"`
}

type xOSNVRam struct {
	Template string `xml:"template,attr,omitempty"`
	Path     string `xml:",chardata"`
}

type xOSType struct {
	Arch    string `xml:"arch,attr,omitempty"`
	Machine string `xml:"machine,attr,omitempty"`
	Value   string `xml:",chardata"`
}

type xOSSMBIOS struct {
	Mode string `xml:"mode,attr"`
}

type xCPU struct {
	Mode  string `xml:"mode,attr,omitempty"`
	Model string `xml:"model,omitempty"`
}

type xSysInfo struct {
	Type   string         `xml:"type,attr"`
	System xSysInfoSystem `xml:"system"`
}

type xSysInfoSystem struct {
	Entries []xSysInfoEntry `xml:"entry"`
}

type xSysInfoEntry struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

type xFeatures struct {
	ACPI *struct{}      `xml:"acpi,omitempty"`
	SMM  *xFeatureState `xml:"smm,omitempty"`
}

type xFeatureState struct {
	State string `xml:"state,attr"`
}

type xDevices struct {
	Disks      []xDisk      `xml:"disk"`
	Interfaces []xInterface `xml:"interface"`
	Serials    []xSerial    `xml:"serial,omitempty"`
	Consoles   []xSerial    `xml:"console,omitempty"`
	Memballoon *xMemballoon `xml:"memballoon,omitempty"`
	TPM        *xTPM        `xml:"tpm,omitempty"`
	Graphics   *xGraphics   `xml:"graphics,omitempty"`
	Video      *xVideo      `xml:"video,omitempty"`
}

type xGraphics struct {
	Type     string `xml:"type,attr"`
	Port     int    `xml:"port,attr,omitempty"`
	AutoPort string `xml:"autoport,attr,omitempty"`
	Listen   string `xml:"listen,attr,omitempty"`
}

type xVideo struct {
	Model xVideoModel `xml:"model"`
}

type xVideoModel struct {
	Type    string `xml:"type,attr"`
	Heads   int    `xml:"heads,attr,omitempty"`
	Primary string `xml:"primary,attr,omitempty"`
}

type xTPM struct {
	Model   string      `xml:"model,attr,omitempty"`
	Backend xTPMBackend `xml:"backend"`
}

type xTPMBackend struct {
	Type    string `xml:"type,attr"`
	Version string `xml:"version,attr,omitempty"`
}

type xDisk struct {
	Type     string      `xml:"type,attr"`
	Device   string      `xml:"device,attr"`
	Driver   xDiskDriver `xml:"driver"`
	Source   xDiskSource `xml:"source"`
	Target   xDiskTarget `xml:"target"`
	ReadOnly *struct{}   `xml:"readonly,omitempty"`
}

type xDiskDriver struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type xDiskSource struct {
	File string `xml:"file,attr"`
}

type xDiskTarget struct {
	Dev string `xml:"dev,attr"`
	Bus string `xml:"bus,attr"`
}

type xInterface struct {
	Type   string        `xml:"type,attr"`
	Mac    *xIfaceMac    `xml:"mac,omitempty"`
	Source *xIfaceSource `xml:"source,omitempty"`
	Model  *xIfaceModel  `xml:"model,omitempty"`
	Target *xIfaceTarget `xml:"target,omitempty"`
}

type xIfaceMac struct {
	Address string `xml:"address,attr"`
}

type xIfaceSource struct {
	Bridge  string `xml:"bridge,attr,omitempty"`
	Network string `xml:"network,attr,omitempty"`
}

type xIfaceModel struct {
	Type string `xml:"type,attr"`
}

type xIfaceTarget struct {
	Dev string `xml:"dev,attr,omitempty"`
}

type xSerial struct {
	Type     string        `xml:"type,attr"`
	Source   *xSerialSrc   `xml:"source,omitempty"`
	Protocol *xSerialProto `xml:"protocol,omitempty"`
	Target   *xSerialTgt   `xml:"target,omitempty"`
}

type xSerialSrc struct {
	Mode    string `xml:"mode,attr,omitempty"`
	Path    string `xml:"path,attr,omitempty"`
	Host    string `xml:"host,attr,omitempty"`
	Service string `xml:"service,attr,omitempty"`
}

type xSerialProto struct {
	Type string `xml:"type,attr"`
}

type xSerialTgt struct {
	Port string `xml:"port,attr,omitempty"`
}

type xMemballoon struct {
	Model string `xml:"model,attr"`
}

func parseSMP(smp string) int {
	if smp == "" {
		return 1
	}
	if n, err := strconv.Atoi(strings.TrimSpace(smp)); err == nil {
		return n
	}
	for part := range strings.SplitSeq(smp, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && (kv[0] == "cpus" || kv[0] == "vcpus") {
			if n, err := strconv.Atoi(kv[1]); err == nil {
				return n
			}
		}
	}
	if n, err := strconv.Atoi(strings.SplitN(smp, ",", 2)[0]); err == nil {
		return n
	}
	return 1
}

func domainTypeFromAccel(accel []string) string {
	for _, a := range accel {
		switch strings.ToLower(strings.SplitN(a, ",", 2)[0]) {
		case "kvm":
			return "kvm"
		case "hvf":
			return "hvf"
		case "xen":
			return "xen"
		}
	}
	return "qemu"
}

func diskBusFromIf(ifv string) (bus string, devPrefix string) {
	switch strings.ToLower(ifv) {
	case "virtio", "":
		return "virtio", "vd"
	case "scsi":
		return "scsi", "sd"
	case "ide":
		return "ide", "hd"
	case "sata":
		return "sata", "sd"
	}
	return "virtio", "vd"
}

func diskDeviceFromMedia(media string) string {
	if strings.EqualFold(media, "cdrom") {
		return "cdrom"
	}
	return "disk"
}

func driverTypeFromFormat(format string) string {
	if format == "" {
		return "raw"
	}
	return format
}

func buildInterface(nd VMNetdev) (xInterface, error) {
	var mac *xIfaceMac
	if nd.Mac != "" {
		mac = &xIfaceMac{Address: nd.Mac}
	}

	t := strings.ToLower(nd.Type)
	switch t {
	case "user", "":
		return xInterface{
			Type:  "user",
			Mac:   mac,
			Model: &xIfaceModel{Type: "virtio"},
		}, nil
	case "bridge", "tap":
		if nd.BR == "" {
			return xInterface{}, errors.New("vm: bridge netdev requires br=<bridge>")
		}
		i := xInterface{
			Type:   "bridge",
			Mac:    mac,
			Source: &xIfaceSource{Bridge: nd.BR},
			Model:  &xIfaceModel{Type: "virtio"},
		}
		if nd.Ifname != "" {
			i.Target = &xIfaceTarget{Dev: nd.Ifname}
		}
		return i, nil
	}
	return xInterface{}, fmt.Errorf("vm: unsupported netdev type %q", nd.Type)
}

func buildCPU(cpu string) *xCPU {
	switch strings.ToLower(strings.TrimSpace(cpu)) {
	case "":
		return nil
	case "host", "host-passthrough", "passthrough":
		return &xCPU{Mode: "host-passthrough"}
	case "host-model":
		return &xCPU{Mode: "host-model"}
	}
	return &xCPU{Mode: "custom", Model: cpu}
}

func buildBios(b *VMBios) (*xOSLoader, *xOSNVRam, error) {
	if b == nil || b.Loader == "" {
		if b != nil && (b.NVRam != "" || b.Secure) {
			return nil, nil, errors.New("vm: bios requires loader=<path> when nvram or secure is set")
		}
		return nil, nil, nil
	}

	loader := &xOSLoader{ReadOnly: "yes", Type: "pflash", Path: b.Loader}
	if b.Secure {
		loader.Secure = "yes"
	}

	if b.NVRam == "" {
		return nil, nil, errors.New("vm: bios requires nvram=<path> alongside loader")
	}
	nvram := &xOSNVRam{Path: b.NVRam, Template: b.NVRamTemplate}

	return loader, nvram, nil
}

func buildTPM(t *VMTPM) *xTPM {
	if t == nil {
		return nil
	}
	model := t.Model
	if model == "" {
		model = "tpm-crb"
	}
	version := t.Version
	if version == "" {
		version = "2.0"
	}
	return &xTPM{Model: model, Backend: xTPMBackend{Type: "emulator", Version: version}}
}

func buildGraphics(g *VMGraphics) (*xGraphics, *xVideo, error) {
	if g == nil {
		return nil, nil, nil
	}

	t := strings.ToLower(g.Type)
	switch t {
	case "vnc", "":
		t = "vnc"
	case "spice":
	default:
		return nil, nil, fmt.Errorf("vm: unsupported graphics type %q, want vnc or spice", g.Type)
	}

	listen := g.Listen
	if listen == "" {
		// Loopback by default
		listen = "127.0.0.1"
	}

	gfx := &xGraphics{Type: t, Listen: listen}
	if g.Port > 0 {
		gfx.Port = g.Port
		gfx.AutoPort = "no"
	} else {
		gfx.Port = -1
		gfx.AutoPort = "yes"
	}

	video := g.Video
	if video == "" {
		video = "virtio"
	}

	return gfx, &xVideo{Model: xVideoModel{Type: video, Heads: 1, Primary: "yes"}}, nil
}

// buildHostfwdArgs renders a user netdev with port forwards as raw QEMU
// arguments. libvirt's <portForward> element only works with the passt
// backend, but SLIRP forwards ports perfectly well through hostfwd=, so the
// netdev is passed through the qemu:commandline namespace instead.
//
// Each spec takes the QEMU form
//
//	[tcp|udp]:[hostaddr]:hostport-[guestaddr]:guestport
//
// Specs are validated rather than passed through blindly: QEMU exits at
// startup on a malformed one, which takes the serial console down with it.
func buildHostfwdArgs(nd VMNetdev, index int) ([]string, error) {
	if t := strings.ToLower(nd.Type); t != "user" && t != "" {
		return nil, fmt.Errorf("vm: hostfwd is only supported on user netdevs, not %q", nd.Type)
	}

	id := nd.ID
	if id == "" {
		id = fmt.Sprintf("hostnet%d", index)
	}

	netdev := "user,id=" + id
	if nd.Net != "" {
		netdev += ",net=" + nd.Net
	}
	if nd.DHCPStart != "" {
		netdev += ",dhcpstart=" + nd.DHCPStart
	}
	for _, spec := range nd.Hostfwd {
		fwd, err := normalizeHostfwd(spec)
		if err != nil {
			return nil, err
		}
		if fwd == "" {
			continue
		}
		netdev += ",hostfwd=" + fwd
	}

	device := "virtio-net-pci,netdev=" + id
	if nd.Mac != "" {
		device += ",mac=" + nd.Mac
	}
	// Without an explicit address QEMU claims the first free slot on pcie.0,
	// which is the one libvirt is about to hand its own pcie-root-port, and the
	// domain dies on a slot conflict. Count down from the top of the bus
	// instead: 0x1f is the q35 LPC bridge, and libvirt allocates upward from
	// 0x1, so the high slots stay clear.
	slot := hostfwdBaseSlot - index
	if slot <= 0 {
		return nil, fmt.Errorf("vm: too many hostfwd netdevs (%d), no free PCI slot", index+1)
	}
	device += fmt.Sprintf(",bus=pcie.0,addr=%#x", slot)

	return []string{"-netdev", netdev, "-device", device}, nil
}

// normalizeHostfwd validates one hostfwd spec and returns it in QEMU's
// canonical proto:hostaddr:hostport-guestaddr:guestport form.
func normalizeHostfwd(spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", nil
	}
	if strings.ContainsAny(spec, ", ") {
		return "", fmt.Errorf("vm: hostfwd %q: one forward per entry, use a list for several", spec)
	}

	proto, rest := "tcp", spec
	// The protocol is optional; without it the spec starts straight at the
	// (possibly empty) host address.
	if head, tail, ok := strings.Cut(spec, ":"); ok {
		if p := strings.ToLower(head); p == "tcp" || p == "udp" {
			proto, rest = p, tail
		}
	}

	hostPart, guestPart, ok := strings.Cut(rest, "-")
	if !ok {
		return "", fmt.Errorf("vm: hostfwd %q: expected <hostaddr>:<hostport>-<guestaddr>:<guestport>", spec)
	}

	hostAddr, hostPort, err := splitHostfwdAddr(hostPart, spec, "host")
	if err != nil {
		return "", err
	}
	guestAddr, guestPort, err := splitHostfwdAddr(guestPart, spec, "guest")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s:%d-%s:%d", proto, hostAddr, hostPort, guestAddr, guestPort), nil
}

func splitHostfwdAddr(s, spec, side string) (string, uint16, error) {
	addr, portStr := "", s
	if i := strings.LastIndex(s, ":"); i >= 0 {
		addr, portStr = s[:i], s[i+1:]
	}
	if addr != "" && net.ParseIP(addr) == nil {
		return "", 0, fmt.Errorf("vm: hostfwd %q: invalid %s address %q", spec, side, addr)
	}
	if portStr == "" {
		return "", 0, fmt.Errorf("vm: hostfwd %q: missing %s port", spec, side)
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || port == 0 {
		return "", 0, fmt.Errorf("vm: hostfwd %q: invalid %s port %q", spec, side, portStr)
	}
	return addr, uint16(port), nil
}

func queryGuestInterfaces(l *libvirt.Libvirt, d libvirt.Domain) []common.NetworkInterface {
	sources := []uint32{
		uint32(libvirt.DomainInterfaceAddressesSrcAgent),
		uint32(libvirt.DomainInterfaceAddressesSrcLease),
		uint32(libvirt.DomainInterfaceAddressesSrcArp),
	}
	for _, src := range sources {
		ifaces, err := l.DomainInterfaceAddresses(d, src, 0)
		if err != nil {
			slog.Debug("queryGuestInterfaces", "msg", "source unavailable", "source", src, "error", err)
			continue
		}
		if len(ifaces) == 0 {
			continue
		}
		out := make([]common.NetworkInterface, 0, len(ifaces))
		for _, i := range ifaces {
			ni := common.NetworkInterface{Name: i.Name}
			if len(i.Hwaddr) > 0 {
				ni.HWAddr = i.Hwaddr[0]
			}
			for _, a := range i.Addrs {
				if a.Addr != "" {
					ni.Addresses = append(ni.Addresses, a.Addr)
				}
			}
			out = append(out, ni)
		}
		return out
	}
	return nil
}

func buildSerial(serial string) *xSerial {
	if serial == "" || strings.EqualFold(serial, "none") {
		return nil
	}
	switch {
	case strings.EqualFold(serial, "stdio"), strings.EqualFold(serial, "pty"):
		return &xSerial{Type: "pty", Target: &xSerialTgt{Port: "0"}}
	case strings.HasPrefix(serial, "file:"):
		return &xSerial{
			Type:   "file",
			Source: &xSerialSrc{Path: strings.TrimPrefix(serial, "file:")},
			Target: &xSerialTgt{Port: "0"},
		}
	case strings.HasPrefix(serial, "telnet:"):
		return buildTCPSerial(strings.TrimPrefix(serial, "telnet:"), "telnet")
	case strings.HasPrefix(serial, "tcp:"):
		return buildTCPSerial(strings.TrimPrefix(serial, "tcp:"), "raw")
	}
	return &xSerial{Type: "pty", Target: &xSerialTgt{Port: "0"}}
}

// buildTCPSerial parses a qemu serial spec of the form
// "host:port,opt,opt" (e.g. "localhost:4444,server,nowait") into a libvirt
// <serial type='tcp'> device.
func buildTCPSerial(spec, protocol string) *xSerial {
	parts := strings.Split(spec, ",")
	hostPort := parts[0]
	mode := "connect"
	for _, opt := range parts[1:] {
		if strings.EqualFold(strings.TrimSpace(opt), "server") {
			mode = "bind"
		}
	}

	host, port := hostPort, ""
	if idx := strings.LastIndex(hostPort, ":"); idx >= 0 {
		host = hostPort[:idx]
		port = hostPort[idx+1:]
	}

	s := &xSerial{
		Type:     "tcp",
		Source:   &xSerialSrc{Mode: mode, Host: host, Service: port},
		Protocol: &xSerialProto{Type: protocol},
		Target:   &xSerialTgt{Port: "0"},
	}
	return s
}
