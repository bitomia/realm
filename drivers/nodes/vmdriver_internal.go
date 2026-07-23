//go:build ignore

package nodes

import (
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"

	"github.com/bitomia/realm/common"
)

func dialLibvirt(socket string) (*libvirt.Libvirt, error) {
	l := libvirt.NewWithDialer(dialers.NewLocal(dialers.WithSocket(socket)))
	if err := l.Connect(); err != nil {
		return nil, fmt.Errorf("libvirt: dial failed: %w", err)
	}
	return l, nil
}

func withLibvirt(socket string, fn func(*libvirt.Libvirt) error) error {
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
	SMBIOS *xOSSMBIOS `xml:"smbios,omitempty"`
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
	ACPI *struct{} `xml:"acpi,omitempty"`
}

type xDevices struct {
	Disks      []xDisk      `xml:"disk"`
	Interfaces []xInterface `xml:"interface"`
	Serials    []xSerial    `xml:"serial,omitempty"`
	Consoles   []xSerial    `xml:"console,omitempty"`
	Memballoon *xMemballoon `xml:"memballoon,omitempty"`
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
