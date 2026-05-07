package capabilities

import (
	"log/slog"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"

	"github.com/bitomia/realm/agent/containers"
	"github.com/bitomia/realm/agent/network"
	"github.com/bitomia/realm/agent/volumes"
	"github.com/bitomia/realm/common/config"
)

var systemCaps *SystemCapabilities = nil

type SystemCapabilities struct {
	// Can use github.com/bitomia/realm/agent/containers
	containersEngine bool
	// Can use github.com/bitomia/realm/agent/network
	containersNetworking bool
	// Can use github.com/bitomia/realm/agent/volumes
	volumes bool
	// Can use github.com/bitomia/realm/agent/volumes zfs
	volumesZFS bool
	// Can host virtual machines
	vmm bool
}

func Get() *SystemCapabilities {
	return systemCaps
}

func Initialize(cfg *config.Config) {
	if systemCaps != nil {
		slog.Error("capabilitis.Initialize", "error", "capabilities already initialized")
		return
	}

	systemCaps = &SystemCapabilities{false, false, false, false, false}
	if cfg.Agent.Containers {
		systemCaps.evalContainersEngine()
		systemCaps.evalContainersNetworking()
	} else {
		slog.Info("Containers support disabled")
	}
	systemCaps.evalVolumes(cfg)
	systemCaps.evalVMM()
}

func (c SystemCapabilities) Print() {
	slog.Info("Capability", "type", "containers engine", "value", c.containersEngine)
	slog.Info("Capability", "type", "container network interfaces", "value", c.containersNetworking)
	slog.Info("Capability", "type", "volumes", "value", c.volumes)
	slog.Info("Capability", "type", "ZFS volumes", "value", c.volumesZFS)
	slog.Info("Capability", "type", "VMM", "value", c.vmm)
}

func (c SystemCapabilities) evalContainersEngine() {
	containerdVersion, err := containers.GetContainerdVersion()
	systemCaps.containersEngine = err == nil

	if err != nil {
		slog.Warn("Cannot get containerd version", "error", err.Error())
	} else {
		slog.Info("Containerd version", "version", containerdVersion)
	}
}

func (c SystemCapabilities) evalContainersNetworking() {
	slog.Info("Checking containers networking availability")

	if err := network.IsCNIAvailable(); err != nil {
		slog.Warn("CNI validation failed", "error", err.Error())
		return
	}

	slog.Info("CNI plugins validated successfully")
	systemCaps.containersNetworking = true
}

func (c SystemCapabilities) evalVolumes(cfg *config.Config) {
	if err := volumes.InitializeManager(cfg.Agent.ZFS); err != nil {
		slog.Warn("Cannot initialize volumes", "error", err.Error())
		return
	}

	volumesPath, err := volumes.GetVolumesPath()
	if err != nil {
		slog.Warn("Cannot get volumes path", "error", err.Error())
		return
	}

	if cfg.Agent.ZFS {
		slog.Info("Volumes ready (ZFS)", "path", volumesPath)
	} else {
		slog.Info("Volumes ready (directory-based)", "path", volumesPath)
	}

	systemCaps.volumes = true
	systemCaps.volumesZFS = cfg.Agent.ZFS
}

func (c SystemCapabilities) evalVMM() {
	l := libvirt.NewWithDialer(dialers.NewLocal())
	if err := l.Connect(); err != nil {
		systemCaps.vmm = false
	}
	systemCaps.vmm = true
}

func (c SystemCapabilities) ContainersEngine() bool {
	return c.containersEngine
}

func (c SystemCapabilities) ContainersNetworking() bool {
	return c.containersNetworking
}

func (c SystemCapabilities) Volumes() bool {
	return c.volumes
}

func (c SystemCapabilities) VolumesZFS() bool {
	return c.volumesZFS
}

func (c SystemCapabilities) VMM() bool {
	return c.vmm
}
