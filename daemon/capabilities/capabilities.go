package capabilities

import (
	"log/slog"

	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/network"
	"github.com/bitomia/realm/daemon/volumes"
)

var globalCaps *HostDaemonCapabilities = nil

type HostDaemonCapabilities struct {
	// Can use github.com/bitomia/realm/daemon/containers
	HasContainersEngine bool `json:"containers_engine"`
	// Can use github.com/bitomia/realm/daemon/network
	HasContainersNetworking bool `json:"networking"`
	// Can use github.com/bitomia/realm/daemon/volumes
	HasVolumes bool `json:"volumes"`
	// Can use github.com/bitomia/realm/daemon/volumes zfs
	HasVolumesZFS bool `json:"volumes_zfs"`
}

func Get() *HostDaemonCapabilities {
	return globalCaps
}

func Initialize(cfg *config.Config) {
	if globalCaps != nil {
		slog.Error("capabilitis.Initialize", "error", "capabilities already initialized")
		return
	}

	globalCaps = &HostDaemonCapabilities{false, false, false, false}
	globalCaps.evalContainersEngine()
	globalCaps.evalContainersNetworking()
	globalCaps.evalVolumes(cfg)
}

func (c HostDaemonCapabilities) Print() {
	slog.Info("Capability", "type", "containers engine", "value", c.HasContainersEngine)
	slog.Info("Capability", "type", "container network interfaces", "value", c.HasContainersNetworking)
	slog.Info("Capability", "type", "volumes", "value", c.HasVolumes)
	slog.Info("Capability", "type", "ZFS volumes", "value", c.HasVolumesZFS)
}

func (c HostDaemonCapabilities) evalContainersEngine() {
	containerdVersion, err := containers.GetContainerdVersion()
	globalCaps.HasContainersEngine = err == nil

	if err != nil {
		slog.Warn("Cannot get containerd version", "error", err.Error())
	} else {
		slog.Info("Containerd version", "version", containerdVersion)
	}
}

func (c HostDaemonCapabilities) evalContainersNetworking() {
	slog.Info("Checking containers networking availability")

	if err := network.IsCNIAvailable(); err != nil {
		slog.Warn("CNI validation failed", "error", err.Error())
		return
	}

	slog.Info("CNI plugins validated successfully")
	globalCaps.HasContainersNetworking = true
}

func (c HostDaemonCapabilities) evalVolumes(cfg *config.Config) {
	if err := volumes.InitializeManager(cfg.Daemon.ZFS); err != nil {
		slog.Warn("Cannot initialize volumes", "error", err.Error())
		return
	}

	volumesPath, err := volumes.GetVolumesPath()
	if err != nil {
		slog.Warn("Cannot get volumes path", "error", err.Error())
		return
	}

	if cfg.Daemon.ZFS {
		slog.Info("Volumes ready (ZFS)", "path", volumesPath)
	} else {
		slog.Info("Volumes ready (directory-based)", "path", volumesPath)
	}

	globalCaps.HasVolumes = true
	globalCaps.HasVolumesZFS = cfg.Daemon.ZFS
}

func (c HostDaemonCapabilities) ContainersEngine() bool {
	return c.HasContainersEngine
}

func (c HostDaemonCapabilities) ContainersNetworking() bool {
	return c.HasContainersNetworking
}

func (c HostDaemonCapabilities) Volumes() bool {
	return c.HasVolumes
}

func (c HostDaemonCapabilities) VolumesZFS() bool {
	return c.HasVolumesZFS
}

func (c HostDaemonCapabilities) VMM() bool {
	// TODO
	return true
}
