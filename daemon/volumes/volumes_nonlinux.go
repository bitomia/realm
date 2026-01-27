//go:build !linux
// +build !linux

package volumes

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
)

// ZFSVolumeManager is not available on non-Linux systems
type ZFSVolumeManager struct{}

// DirectoryVolumeManager implements VolumeManager using simple directories
type DirectoryVolumeManager struct{}

// ZFSVolumeManager methods - all return errors on non-Linux
func (m *ZFSVolumeManager) Init() error {
	return errors.New("ZFS not supported on this platform")
}

func (m *ZFSVolumeManager) MountVolume(volume string) (string, error) {
	return "", errors.New("ZFS not supported on this platform")
}

func (m *ZFSVolumeManager) IsVolume(volume string) bool {
	return false
}

func (m *ZFSVolumeManager) CreateVolume(volume string) error {
	return errors.New("ZFS not supported on this platform")
}

func (m *ZFSVolumeManager) DeleteVolume(volume string, deferred bool) error {
	return errors.New("ZFS not supported on this platform")
}

func (m *ZFSVolumeManager) SetVolumeQuota(volume string, quotaSize string) error {
	return errors.New("ZFS not supported on this platform")
}

func (m *ZFSVolumeManager) DisableVolumeQuota(volume string) error {
	return errors.New("ZFS not supported on this platform")
}

func (m *ZFSVolumeManager) GetVolumeInfo(volume string) (*VolumeInfo, error) {
	return nil, errors.New("ZFS not supported on this platform")
}

// DirectoryVolumeManager implementation for non-Linux systems

func (m *DirectoryVolumeManager) Init() error {
	// No initialization needed for directory-based volumes
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
	return errors.New("quota management not supported with directory-based volumes")
}

func (m *DirectoryVolumeManager) DisableVolumeQuota(volume string) error {
	slog.Warn("DisableVolumeQuota not supported with directory-based volumes", "volume", volume)
	return errors.New("quota management not supported with directory-based volumes")
}

// ValidateZFSAvailability is a no-op on non-Linux systems
func ValidateZFSAvailability() error {
	slog.Info("ZFS validation skipped on non-Linux systems")
	return nil
}
