package nodes

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net"
	"reflect"

	"github.com/digitalocean/go-libvirt"
	"github.com/go-viper/mapstructure/v2"

	"github.com/bitomia/realm/agent/config"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/cloudinit"
	commonConfig "github.com/bitomia/realm/common/config"
)

const VMDriverID common.NodeDriverID = "vm"

type VMDrive struct {
	File   string `json:"file,omitempty"`
	Format string `json:"format,omitempty"`
	If     string `json:"if,omitempty"`
	Media  string `json:"media,omitempty"`
	Index  string `json:"index,omitempty"`
	Resize string `json:"resize,omitempty"`
}

type VMNetdev struct {
	Type       string `json:"type,omitempty"`
	ID         string `json:"id,omitempty"`
	Ifname     string `json:"ifname,omitempty"`
	Script     string `json:"script,omitempty"`
	Downscript string `json:"downscript,omitempty"`
	BR         string `json:"br,omitempty"`
	Helper     string `json:"helper,omitempty"`
	Net        string `json:"net,omitempty"`
	DHCPStart  string `json:"dhcpstart,omitempty"`
	Hostfwd    string `json:"hostfwd,omitempty"`
}

type VMConfig struct {
	Machine string     `json:"machine,omitempty"`
	Accel   []string   `json:"accel,omitempty"`
	CPU     string     `json:"cpu,omitempty"`
	Memory  int        `json:"memory,omitempty"`
	SMP     string     `json:"smp,omitempty"`
	Serial  string     `json:"serial,omitempty"`
	Drives  []VMDrive  `json:"drives,omitempty"`
	Netdev  []VMNetdev `json:"netdev,omitempty"`
}

// VMNodeMetadata is persisted in the nodes repository and used to
// rehydrate the libvirt domain XML across agent restarts.
type VMNodeMetadata struct {
	DomainName string               `json:"domain_name"`
	DomainXML  string               `json:"domain_xml"`
	Drives     map[int]OverlayImage `json:"overlay_drives"`
}

type VMDriver struct {
	Config VMConfig
}

func stringToSliceHook(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
	if from.Kind() == reflect.String && to == reflect.TypeOf([]string{}) {
		return []string{data.(string)}, nil
	}
	return data, nil
}

func NewVMDriverFromConfig(c *any) (common.NodeDriver, error) {
	var cfg VMConfig
	if c != nil {
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			TagName:    "json",
			Result:     &cfg,
			DecodeHook: stringToSliceHook,
		})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(*c); err != nil {
			return nil, err
		}
	}

	return &VMDriver{Config: cfg}, nil
}

func (q *VMDriver) DriverInfo() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(
		VMDriverID,
		NewVMDriverFromConfig,
		true,
		common.WithStartMode(common.AgentMode),
	)
}

func (q *VMDriver) GetNodeDriverID() common.NodeDriverID {
	return VMDriverID
}

func (q *VMDriver) Provision(nodeName string, cloudInit *cloudinit.CloudInit, repository common.NodesRepository) error {
	slog.Info("VMDriver.Provision", "msg", "preparing libvirt domain", "node", nodeName)

	var err error
	overlayDrives, err := resolveDrives(q.Config.Drives, nodeName)
	if err != nil {
		return fmt.Errorf("vm: failed to resolve drive images: %w", err)
	}

	domainXML, err := q.buildDomainXML(nodeName, overlayDrives, cloudInit)
	if err != nil {
		cleanupOverlays(nodeName)
		return fmt.Errorf("vm: failed to build domain XML: %w", err)
	}

	metadata := VMNodeMetadata{
		DomainName: nodeName,
		DomainXML:  domainXML,
		Drives:     overlayDrives,
	}

	if err := repository.SetGuestNode(nodeName, q, cloudInit, metadata); err != nil {
		slog.Error("VMDriver.Provision", "msg", "failed to persist guest node", "error", err)
		for _, d := range overlayDrives {
			d.Cleanup()
		}
		return err
	}

	slog.Info("VMDriver.Provision", "msg", "domain prepared (not yet started)", "node", nodeName)
	return nil
}

func (q *VMDriver) Deprovision(nodeName *string, repository common.NodesRepository) error {
	if nodeName == nil {
		return fmt.Errorf("vMDriver expects node name for guest node deprovision")
	}

	self, err := repository.GetGuestNode(*nodeName)
	if err == nil {
		metadata, mErr := common.CastMetadata[VMNodeMetadata](&self.Metadata)
		if mErr != nil {
			slog.Warn("VMDriver.Deprovision", "msg", "cannot cast metadata", "error", mErr)
		} else {
			if err := withLibvirt(func(l *libvirt.Libvirt) error {
				d, found, err := lookupDomain(l, metadata.DomainName)
				if err != nil {
					return err
				}
				if !found {
					return nil
				}
				if err := l.DomainDestroy(d); err != nil && !libvirt.IsNotFound(err) {
					return err
				}
				return nil
			}); err != nil {
				slog.Warn("VMDriver.Deprovision", "msg", "failed to destroy domain", "error", err)
			}
			for _, drive := range metadata.Drives {
				drive.Cleanup()
			}
		}
	}

	return repository.DeleteGuestNode(*nodeName, q, self.Metadata)
}

func (q *VMDriver) GetDriverConfig() common.NodeDriverConfig {
	var c any = q.Config
	return common.NodeDriverConfig{Driver: VMDriverID, DriverConfig: &c}
}

func (q *VMDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(q.GetDriverConfig())
}

func (q *VMDriver) UnmarshalJSON(data []byte) error {
	var cfgMap map[string]any
	if err := json.Unmarshal(data, &cfgMap); err != nil {
		return err
	}

	var nodeDriver common.NodeDriver
	var err error
	if len(cfgMap) > 0 {
		var a any = cfgMap
		nodeDriver, err = NewVMDriverFromConfig(&a)
	} else {
		nodeDriver, err = NewVMDriverFromConfig(nil)
	}
	if err != nil {
		return err
	}
	*q = *nodeDriver.(*VMDriver)
	return nil
}

func (q *VMDriver) buildDomainXML(nodeName string, overlayDrives map[int]OverlayImage, cloudInit *cloudinit.CloudInit) (string, error) {
	dom := xDomain{
		Type: domainTypeFromAccel(q.Config.Accel),
		Name: nodeName,
		OS: xOS{
			Type: xOSType{Machine: q.Config.Machine, Value: "hvm"},
		},
		Features: &xFeatures{ACPI: &struct{}{}},
		Devices: xDevices{
			Memballoon: &xMemballoon{Model: "virtio"},
		},
	}

	if q.Config.Memory > 0 {
		dom.Memory = &xMemory{Unit: "MiB", Value: q.Config.Memory}
	}
	if vcpus := parseSMP(q.Config.SMP); vcpus > 0 {
		dom.VCPU = &xVCPU{Value: vcpus}
	}
	if q.Config.CPU != "" {
		dom.CPU = &xCPU{Mode: "custom", Model: q.Config.CPU}
	}

	for idx, drive := range q.Config.Drives {
		ov, ok := overlayDrives[idx]
		if !ok {
			continue
		}
		bus, prefix := diskBusFromIf(drive.If)
		d := xDisk{
			Type:   "file",
			Device: diskDeviceFromMedia(drive.Media),
			Driver: xDiskDriver{Name: "qemu", Type: driverTypeFromFormat(drive.Format)},
			Source: xDiskSource{File: ov.FilePath},
			Target: xDiskTarget{Dev: fmt.Sprintf("%s%c", prefix, 'a'+idx), Bus: bus},
		}
		if d.Device == "cdrom" {
			d.ReadOnly = &struct{}{}
		}
		dom.Devices.Disks = append(dom.Devices.Disks, d)
	}

	for _, nd := range q.Config.Netdev {
		iface, err := buildInterface(nd)
		if err != nil {
			return "", err
		}
		dom.Devices.Interfaces = append(dom.Devices.Interfaces, iface)
	}

	if s := buildSerial(q.Config.Serial); s != nil {
		dom.Devices.Serials = append(dom.Devices.Serials, *s)
		dom.Devices.Consoles = append(dom.Devices.Consoles, *s)
	}

	if cloudInit != nil {
		cfg := config.Get()
		host := q.resolveCloudInitHost(cfg)
		serial := fmt.Sprintf("ds=nocloud-net;s=http://%s:%d/cloudinit/%s/", host, cfg.Agent.ListenPort, nodeName)
		dom.SysInfo = &xSysInfo{
			Type: "smbios",
			System: xSysInfoSystem{Entries: []xSysInfoEntry{
				{Name: "serial", Value: serial},
			}},
		}
		dom.OS.SMBIOS = &xOSSMBIOS{Mode: "sysinfo"}
	}

	out, err := xml.MarshalIndent(dom, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (q *VMDriver) Start(nodeName *string, repository common.NodesRepository) error {
	if nodeName == nil {
		return fmt.Errorf("nodeName cannot be nil")
	}
	metadata, err := q.getMetadata(*nodeName, repository)
	if err != nil {
		return fmt.Errorf("vm: failed to get metadata: %w", err)
	}

	return withLibvirt(func(l *libvirt.Libvirt) error {
		if d, found, err := lookupDomain(l, metadata.DomainName); err != nil {
			return err
		} else if found {
			state, _, err := l.DomainGetState(d, 0)
			if err != nil {
				return err
			}
			if libvirt.DomainState(state) == libvirt.DomainRunning {
				slog.Info("VMDriver.Start", "msg", "domain already running", "node", *nodeName)
				return nil
			}
			if err := l.DomainDestroy(d); err != nil && !libvirt.IsNotFound(err) {
				return err
			}
		}
		if _, err := l.DomainCreateXML(metadata.DomainXML, 0); err != nil {
			return fmt.Errorf("vm: DomainCreateXML failed: %w", err)
		}
		slog.Info("VMDriver.Start", "msg", "domain started", "node", *nodeName)
		return nil
	})
}

func (q *VMDriver) Stop(nodeName *string, _ string, _ uint32, repository common.NodesRepository, force bool) error {
	if nodeName == nil {
		return fmt.Errorf("nodeName cannot be nil")
	}
	metadata, err := q.getMetadata(*nodeName, repository)
	if err != nil {
		return fmt.Errorf("vm: failed to get metadata: %w", err)
	}

	return withLibvirt(func(l *libvirt.Libvirt) error {
		d, found, err := lookupDomain(l, metadata.DomainName)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		if force {
			return l.DomainDestroy(d)
		}
		return l.DomainShutdown(d)
	})
}

func (q *VMDriver) Restart(nodeName *string, _ string, _ uint32, repository common.NodesRepository) error {
	if nodeName == nil {
		return fmt.Errorf("nodeName cannot be nil")
	}
	metadata, err := q.getMetadata(*nodeName, repository)
	if err != nil {
		return fmt.Errorf("getMetadata on Restart failed: %s", err.Error())
	}

	return withLibvirt(func(l *libvirt.Libvirt) error {
		d, found, err := lookupDomain(l, metadata.DomainName)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("vm: domain %q is not running", metadata.DomainName)
		}
		return l.DomainReboot(d, 0)
	})
}

func (q *VMDriver) UpdateStatus(nodeName *string, repository common.NodesRepository) (common.NodeStatus, error) {
	if nodeName == nil {
		err := fmt.Errorf("getMetadata on UpdateStatus failed: nodeName cannot be nil")
		return common.NodeStatus{StatusCode: common.NodeStatusError, Reason: err.Error()}, err
	}
	metadata, err := q.getMetadata(*nodeName, repository)
	if err != nil {
		e := fmt.Errorf("getMetadata on UpdateStatus failed: %s", err.Error())
		return common.NodeStatus{StatusCode: common.NodeStatusError, Reason: e.Error()}, e
	}

	var status common.NodeStatus
	err = withLibvirt(func(l *libvirt.Libvirt) error {
		d, found, err := lookupDomain(l, metadata.DomainName)
		if err != nil {
			return err
		}
		if !found {
			status = common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: "not started"}
			return nil
		}
		state, _, err := l.DomainGetState(d, 0)
		if err != nil {
			return err
		}
		switch libvirt.DomainState(state) {
		case libvirt.DomainRunning, libvirt.DomainBlocked:
			status = common.NodeStatus{StatusCode: common.NodeStatusReady}
		case libvirt.DomainPaused, libvirt.DomainPmsuspended:
			status = common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: "paused"}
		case libvirt.DomainShutdown, libvirt.DomainShutoff:
			status = common.NodeStatus{StatusCode: common.NodeStatusOffline, Reason: "VM is not running"}
		case libvirt.DomainCrashed:
			status = common.NodeStatus{StatusCode: common.NodeStatusError, Reason: "VM crashed"}
		default:
			status = common.NodeStatus{StatusCode: common.NodeStatusOffline, Reason: "unknown state"}
		}
		return nil
	})
	if err != nil {
		return common.NodeStatus{StatusCode: common.NodeStatusError, Reason: err.Error()}, err
	}
	return status, nil
}

func (q *VMDriver) GetState(nodeName *string, repository common.NodesRepository) (common.NodeState, error) {
	state := common.NodeState{}

	self, err := repository.GetGuestNode(*nodeName)
	if err != nil {
		return state, err
	}
	metadata, err := common.CastMetadata[VMNodeMetadata](&self.Metadata)
	if err != nil {
		return state, fmt.Errorf("vMDriver.GetState cannot cast metadata: %s", err.Error())
	}

	err = withLibvirt(func(l *libvirt.Libvirt) error {
		d, found, err := lookupDomain(l, metadata.DomainName)
		if err != nil {
			return err
		}
		if !found {
			if q.Config.Memory > 0 {
				state.TotalMem = uint64(q.Config.Memory) * 1024 * 1024
			}
			return nil
		}
		_, maxMemKiB, _, nrCPU, _, err := l.DomainGetInfo(d)
		if err != nil {
			return err
		}
		state.NumCPU = int(nrCPU)
		state.TotalMem = maxMemKiB * 1024

		stats, sErr := l.DomainMemoryStats(d, uint32(libvirt.DomainMemoryStatNr), 0)
		if sErr != nil {
			slog.Debug("VMDriver.GetState", "msg", "memory stats unavailable", "error", sErr)
			return nil
		}
		var actual, unused, available uint64
		for _, s := range stats {
			switch libvirt.DomainMemoryStatTags(s.Tag) {
			case libvirt.DomainMemoryStatActualBalloon:
				actual = s.Val
			case libvirt.DomainMemoryStatUnused:
				unused = s.Val
			case libvirt.DomainMemoryStatAvailable:
				available = s.Val
			}
		}
		// libvirt memory stats are in KiB.
		if available > 0 {
			state.TotalMem = available * 1024
			state.FreeMem = unused * 1024
			state.UsedMem = state.TotalMem - state.FreeMem
		} else if actual > 0 {
			state.UsedMem = actual * 1024
			if state.TotalMem > state.UsedMem {
				state.FreeMem = state.TotalMem - state.UsedMem
			}
		}
		if state.TotalMem > 0 {
			state.FreeMemPercent = float64(state.FreeMem) / float64(state.TotalMem) * 100
		}

		state.NetworkInterfaces = queryGuestInterfaces(l, d)
		for _, ni := range state.NetworkInterfaces {
			slog.Info("VMDriver.GetState", "node", *nodeName, "iface", ni.Name, "hwaddr", ni.HWAddr, "addrs", ni.Addresses)
		}
		return nil
	})
	if err != nil {
		return state, err
	}
	return state, nil
}

func (q *VMDriver) resolveCloudInitHost(cfg *commonConfig.Config) string {
	// find first bridged netdev
	var bridgeName string
	hasBridgedNetdev := false
	for _, nd := range q.Config.Netdev {
		if nd.Type != "" && nd.Type != "user" {
			hasBridgedNetdev = true
			bridgeName = nd.BR
			break
		}
	}

	if !hasBridgedNetdev {
		return "10.0.2.2"
	}

	if bridgeName != "" {
		if iface, err := net.InterfaceByName(bridgeName); err == nil {
			if addrs, err := iface.Addrs(); err == nil {
				for _, addr := range addrs {
					if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
						return ipNet.IP.String()
					}
				}
			}
		}
		slog.Warn("VMDriver.resolveCloudInitHost", "msg", "could not get IP from bridge interface", "bridge", bridgeName)
	}

	if cfg.NetworkConfig.IPAddress != nil {
		return cfg.NetworkConfig.IPAddress.String()
	}
	if cfg.Agent.ListenAddress != "" && cfg.Agent.ListenAddress != "0.0.0.0" {
		return cfg.Agent.ListenAddress
	}

	slog.Warn("VMDriver.resolveCloudInitHost",
		"msg", "bridged networking detected but could not determine host IP; falling back to 10.0.2.2 which may not be reachable from the guest")
	return "10.0.2.2"
}

func (q *VMDriver) getMetadata(nodeName string, repository common.NodesRepository) (*VMNodeMetadata, error) {
	node, err := repository.GetGuestNode(nodeName)
	if err != nil {
		return nil, err
	}

	var metadata VMNodeMetadata
	tmp, err := json.Marshal(node.Metadata)
	if err != nil {
		slog.Error("VMDriver.getMetadata", "error", "error on marshaling metadata", "node", nodeName)
		return nil, err
	}
	if err := json.Unmarshal(tmp, &metadata); err != nil {
		slog.Error("VMDriver.getMetadata", "error", "error on unmarshalling metadata", "node", nodeName)
		return nil, err
	}
	return &metadata, nil
}
