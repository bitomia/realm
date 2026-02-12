package daemon

import (
	"log/slog"

	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/network"
	"github.com/bitomia/realm/daemon/volumes"
)

type Capabilities struct {
	ContainersEngine bool
	Volumes          bool
	VolumesZFS       bool
	CNI              bool
}

func (c Capabilities) Evaluate(cfg *config.Config) {
	c.reset()
	c.evalContainersEngine()
	c.evalVolumes(cfg)
	c.evalCNI()
}

func (c Capabilities) Print() {
	slog.Info("Capability", "type", "containers engine", "value", c.ContainersEngine)
	slog.Info("Capability", "type", "volumes", "value", c.Volumes)
	slog.Info("Capability", "type", "ZFS volumes", "value", c.VolumesZFS)
	slog.Info("Capability", "type", "container network interfaces", "value", c.CNI)
}

func (c Capabilities) reset() {
	c.ContainersEngine = false
	c.Volumes = false
	c.VolumesZFS = false
	c.CNI = false
}

func (c Capabilities) evalContainersEngine() {
	containerdVersion, err := containers.GetContainerdVersion()
	globalCapabilities.ContainersEngine = err == nil

	if err != nil {
		slog.Warn("Cannot get containerd version", "error", err.Error())
	} else {
		slog.Info("Containerd version", "version", containerdVersion)
	}
}

func (c Capabilities) evalVolumes(cfg *config.Config) {
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

	globalCapabilities.Volumes = true
	globalCapabilities.VolumesZFS = cfg.Daemon.ZFS
}

func (c Capabilities) evalCNI() {
	slog.Info("Checking CNI availability")

	if err := network.IsCNIAvailable(); err != nil {
		slog.Warn("CNI validation failed", "error", err.Error())
		return
	}

	slog.Info("CNI plugins validated successfully")
	globalCapabilities.CNI = true
}
