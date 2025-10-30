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

func MountVolume(volume string) (string, error) {
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

func IsVolume(volume string) bool {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return false
	}
	ds, err := zfs.DatasetOpen(volumePath)
	if err != nil {
		return false
	}
	defer ds.Close()
	return true
}

func CreateVolume(volume string) error {
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

func DeleteVolume(volume string, deferred bool) error {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return err
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

func SetVolumeQuota(volume string, quotaSize string) error {
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

func DisableVolumeQuota(volume string) error {
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

func GetVolumeInfo(volume string) (*VolumeInfo, error) {
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
