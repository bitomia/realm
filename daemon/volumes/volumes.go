package volumes

import (
	"errors"
	"log"
	"path/filepath"

	"github.com/bitomia/realm/daemon/config"
)

type VolumeInfo struct {
	Name  string
	Quota string
	Used  string
}

// VolumeManager defines the interface for volume operations
type VolumeManager interface {
	// Init initializes the volume manager
	Init() error
	// MountVolume mounts a volume and returns its path
	MountVolume(volume string) (string, error)
	// IsVolume checks if a volume exists
	IsVolume(volume string) bool
	// CreateVolume creates a new volume
	CreateVolume(volume string) error
	// DeleteVolume deletes a volume
	DeleteVolume(volume string, deferred bool) error
	// SetVolumeQuota sets a quota for a volume
	SetVolumeQuota(volume string, quotaSize string) error
	// DisableVolumeQuota disables quota for a volume
	DisableVolumeQuota(volume string) error
	// GetVolumeInfo retrieves information about a volume
	GetVolumeInfo(volume string) (*VolumeInfo, error)
}

var manager VolumeManager

// newZFSManager is registered by volumes_zfs.go on linux via init().
// On non-linux platforms it remains nil, and InitializeManager returns an error.
var newZFSManager func() VolumeManager

// InitializeManager initializes the appropriate volume manager based on configuration
func InitializeManager(useZFS bool) error {
	if useZFS {
		if newZFSManager == nil {
			return errors.New("ZFS is not supported on this platform")
		}
		manager = newZFSManager()
	} else {
		manager = &DirectoryVolumeManager{}
	}
	return manager.Init()
}

// Init initializes the volume manager (calls the manager's Init method)
func Init() error {
	if manager == nil {
		return errors.New("volume manager not initialized, call InitializeManager first")
	}
	return manager.Init()
}

// MountVolume mounts a volume using the configured manager
func MountVolume(volume string) (string, error) {
	if manager == nil {
		return "", errors.New("volume manager not initialized")
	}
	return manager.MountVolume(volume)
}

// IsVolume checks if a volume exists using the configured manager
func IsVolume(volume string) bool {
	if manager == nil {
		return false
	}
	return manager.IsVolume(volume)
}

// CreateVolume creates a volume using the configured manager
func CreateVolume(volume string) error {
	if manager == nil {
		return errors.New("volume manager not initialized")
	}
	return manager.CreateVolume(volume)
}

// DeleteVolume deletes a volume using the configured manager
func DeleteVolume(volume string, deferred bool) error {
	if manager == nil {
		return errors.New("volume manager not initialized")
	}
	return manager.DeleteVolume(volume, deferred)
}

// SetVolumeQuota sets a quota for a volume using the configured manager
func SetVolumeQuota(volume string, quotaSize string) error {
	if manager == nil {
		return errors.New("volume manager not initialized")
	}
	return manager.SetVolumeQuota(volume, quotaSize)
}

// DisableVolumeQuota disables quota for a volume using the configured manager
func DisableVolumeQuota(volume string) error {
	if manager == nil {
		return errors.New("volume manager not initialized")
	}
	return manager.DisableVolumeQuota(volume)
}

// GetVolumeInfo retrieves information about a volume using the configured manager
func GetVolumeInfo(volume string) (*VolumeInfo, error) {
	if manager == nil {
		return nil, errors.New("volume manager not initialized")
	}
	return manager.GetVolumeInfo(volume)
}

func GetVolumesPath() (string, error) {
	volumesPath := config.Get().Daemon.VolumesPool
	if volumesPath == "" {
		return "", errors.New("REALM_VOLUMES_POOL not found")
	}

	if config.Get().Daemon.ZFS {
		return volumesPath, nil
	} else {
		return filepath.Join(config.Get().Daemon.DataPath, volumesPath), nil
	}

}

func GetPathForVolume(volume string) (string, error) {
	volumesPath, err := GetVolumesPath()
	if err != nil {
		log.Printf("GetPathForVolume failed: %v\n", err)
		return "", err
	}
	return filepath.Join(volumesPath, volume), nil
}
