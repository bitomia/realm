//go:build windows
// +build windows

package volumes

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func MountVolume(volume string) (string, error) {
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

	// Verify it's a directory
	if !info.IsDir() {
		return "", fmt.Errorf("volume path is not a directory: %s", volumePath)
	}

	slog.Info("Volume mounted", "path", volumePath)
	return volumePath, nil
}

func IsVolume(volume string) bool {
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

func CreateVolume(volume string) error {
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

func DeleteVolume(volume string, deferred bool) error {
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

func SetVolumeQuota(volume string, quotaSize string) error {
	// For a simple file-based volume implementation, we cannot enforce quotas.
	slog.Warn("SetVolumeQuota is not supported for file-based volumes on Windows", "volume", volume)
	return errors.New("quota management is not supported for file-based volumes on Windows")
}

func DisableVolumeQuota(volume string) error {
	// Since we don't support quotas in file-based implementation, this is a no-op
	slog.Warn("DisableVolumeQuota is not supported for file-based volumes on Windows", "volume", volume)
	return nil
}

type VolumeInfo struct {
	Name  string
	Quota string
	Used  string
}

func GetVolumeInfo(volume string) (*VolumeInfo, error) {
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

	// Get free space on the volume
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	pathPtr, err := windows.UTF16PtrFromString(volumePath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert path: %v", err)
	}

	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk space: %v", err)
	}

	return &VolumeInfo{
		Name:  volumePath,
		Quota: "none", // No quota support for file-based volumes
		Used:  fmt.Sprintf("%d", size),
	}, nil
}
