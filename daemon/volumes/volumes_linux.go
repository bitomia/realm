//go:build linux
// +build linux

package volumes

/*
   #cgo LDFLAGS: -lzfs -lzpool -lnvpair -lzfs_core
*/
import "C"

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	zfs "github.com/bitomia/go-libzfs"
)

// ZFSVolumeManager implements VolumeManager using ZFS
type ZFSVolumeManager struct{}

// DirectoryVolumeManager implements VolumeManager using simple directories
type DirectoryVolumeManager struct{}

func datasetExists(volume string) bool {
	parentPath, err := GetVolumesPath()
	if err != nil {
		return false
	}

	parent, err := zfs.DatasetOpen(parentPath)
	if err != nil {
		return false
	}
	defer parent.Close()

	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return false
	}

	for _, child := range parent.Children {
		if name, ok := child.Properties[zfs.DatasetPropName]; ok {
			if name.Value == volumePath {
				return true
			}
		}
	}
	return false
}

func (m *ZFSVolumeManager) MountVolume(volume string) (string, error) {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return "", err
	}
	ds, err := zfs.DatasetOpen(volumePath)
	if err != nil {
		return "", err
	}
	defer ds.Close()

	mounted, mountPoint := ds.IsMounted()
	if mounted == true {
		slog.Info("Volume already mounted", "path", mountPoint)
		return mountPoint, nil
	} else {
		err = ds.Mount("", 0) // "" means default mountpoint, 0 means no special flags
		if err != nil {
			return "", err
		}
		mounted, mountPoint = ds.IsMounted()
	}
	if mounted {
		slog.Info("Volume mounted", "path", mountPoint)
		if err := os.Chown(mountPoint, 1001, 1001); err != nil {
			slog.Error("Failed to set permissions", "path", mountPoint, "error", err)
		}
		return mountPoint, nil
	} else {
		return "", fmt.Errorf("Could not mount %s", volume)
	}
}

func (m *ZFSVolumeManager) IsVolume(volume string) bool {
	return datasetExists(volume)
}

func (m *ZFSVolumeManager) CreateVolume(volume string) error {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		debug.PrintStack()
		return err
	}
	props := map[zfs.Prop]zfs.Property{
		zfs.DatasetPropCanmount: zfs.Property{Value: "on"},
		//		zfs.DatasetPropCompression: "lz4",
	}

	d, err := zfs.DatasetCreate(volumePath, zfs.DatasetTypeFilesystem, props)
	if err != nil {
		debug.PrintStack()
		return err
	}
	defer d.Close()

	slog.Info("Volume created", "path", volumePath)
	return nil
}

func (m *ZFSVolumeManager) DeleteVolume(volume string, deferred bool) error {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return err
	}

	// Check if dataset exists before opening to avoid SIGSEGV in go-libzfs
	if !datasetExists(volume) {
		slog.Info("Volume does not exist, skipping delete", "path", volumePath)
		return nil
	}

	ds, err := zfs.DatasetOpen(volumePath)
	if err != nil {
		return err
	}
	defer ds.Close()

	if mounted, _ := ds.IsMounted(); mounted {
		err = ds.Unmount(0)
		if err != nil {
			return err
		}
	}
	err = ds.Destroy(deferred)
	if err != nil {
		return err
	}

	slog.Info("Volume deleted", "path", volumePath)
	return nil
}

func (m *ZFSVolumeManager) SetVolumeQuota(volume string, quotaSize string) error {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return err
	}
	ds, err := zfs.DatasetOpen(volumePath)
	if err != nil {
		return err
	}
	err = ds.SetProperty(zfs.DatasetPropQuota, quotaSize)
	if err != nil {
		return err
	}
	return nil
}

func (m *ZFSVolumeManager) DisableVolumeQuota(volume string) error {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return err
	}

	ds, err := zfs.DatasetOpen(volumePath)
	if err != nil {
		return err
	}
	err = ds.SetProperty(zfs.DatasetPropQuota, "none")
	if err != nil {
		return err
	}
	return nil
}

func (m *ZFSVolumeManager) GetVolumeInfo(volume string) (*VolumeInfo, error) {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return nil, err
	}

	ds, err := zfs.DatasetOpen(volumePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open dataset %s: %v", volumePath, err)
	}
	defer ds.Close()

	quota, err := ds.GetProperty(zfs.DatasetPropQuota)
	if err != nil {
		return nil, fmt.Errorf("failed to get quota for dataset %s: %v", volumePath, err)
	}

	used, err := ds.GetProperty(zfs.DatasetPropUsed)
	if err != nil {
		return nil, fmt.Errorf("failed to get used space for dataset %s: %v", volumePath, err)
	}

	return &VolumeInfo{
		Name:  volumePath,
		Quota: quota.Value,
		Used:  used.Value,
	}, nil
}

func (m *ZFSVolumeManager) Init() error {
	return zfs.Init()
}

// DirectoryVolumeManager implementation for Linux
func (m *DirectoryVolumeManager) Init() error {
	path, err := GetVolumesPath()

	if err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}

	return nil
}

func (m *DirectoryVolumeManager) MountVolume(volume string) (string, error) {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return "", err
	}

	// Check if the path exists
	info, err := os.Stat(volumePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("volume path does not exist: %s", volumePath)
		}
		return "", fmt.Errorf("failed to stat volume path: %v", err)
	}

	// Check it is a directory
	if !info.IsDir() {
		return "", fmt.Errorf("volume path is not a directory: %s", volumePath)
	}

	slog.Info("Volume mounted", "path", volumePath)
	return volumePath, nil
}

func (m *DirectoryVolumeManager) IsVolume(volume string) bool {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return false
	}
	info, err := os.Stat(volumePath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (m *DirectoryVolumeManager) CreateVolume(volume string) error {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return err
	}

	// Create the directory with appropriate permissions
	err = os.MkdirAll(volumePath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create volume directory: %v", err)
	}

	slog.Info("Volume created", "path", volumePath)
	return nil
}

func (m *DirectoryVolumeManager) DeleteVolume(volume string, deferred bool) error {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return err
	}

	// Check if the path exists
	if _, err := os.Stat(volumePath); os.IsNotExist(err) {
		return fmt.Errorf("volume does not exist: %s", volumePath)
	}

	// Remove the directory and all its contents
	err = os.RemoveAll(volumePath)
	if err != nil {
		return fmt.Errorf("failed to delete volume: %v", err)
	}

	slog.Info("Volume deleted", "path", volumePath)
	return nil
}

func (m *DirectoryVolumeManager) SetVolumeQuota(volume string, quotaSize string) error {
	slog.Warn("SetVolumeQuota not supported with directory-based volumes", "volume", volume)
	return fmt.Errorf("quota management not supported with directory-based volumes")
}

func (m *DirectoryVolumeManager) DisableVolumeQuota(volume string) error {
	slog.Warn("DisableVolumeQuota not supported with directory-based volumes", "volume", volume)
	return fmt.Errorf("quota management not supported with directory-based volumes")
}

func (m *DirectoryVolumeManager) GetVolumeInfo(volume string) (*VolumeInfo, error) {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return nil, err
	}

	// For directory-based volumes, we can't get detailed quota information
	// Just return basic info
	return &VolumeInfo{
		Name:  volumePath,
		Quota: "none",
		Used:  "unknown",
	}, nil
}
