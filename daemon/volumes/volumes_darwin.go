//go:build darwin
// +build darwin

package volumes

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func (m *DirectoryVolumeManager) GetVolumeInfo(volume string) (*VolumeInfo, error) {
	volumePath, err := GetPathForVolume(volume)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(volumePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("volume does not exist: %s", volumePath)
	}

	// Calculate directory size
	var size int64
	err = filepath.Walk(volumePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to calculate volume size: %v", err)
	}

	// Get free space on the volume using statfs
	var stat unix.Statfs_t
	err = unix.Statfs(volumePath, &stat)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk space: %v", err)
	}

	return &VolumeInfo{
		Name:  volumePath,
		Quota: "none", // No quota support for file-based volumes
		Used:  fmt.Sprintf("%d", size),
	}, nil
}
