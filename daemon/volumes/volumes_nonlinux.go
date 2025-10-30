//go:build !linux
// +build !linux

package volumes

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	slog.Warn("SetVolumeQuota only supported on Linux", "volume", volume)
	return errors.New("quota management only supported on Linux")
}

func DisableVolumeQuota(volume string) error {
	slog.Warn("DisableVolumeQuota only supported on Linux", "volume", volume)
	return errors.New("quota management only supported on Linux")
}
