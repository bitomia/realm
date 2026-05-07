package volumes

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// DirectoryVolumeManager implements VolumeManager using simple directories
type DirectoryVolumeManager struct{}

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

	info, err := os.Stat(volumePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("volume path does not exist: %s", volumePath)
		}
		return "", fmt.Errorf("failed to stat volume path: %v", err)
	}

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

	if _, err := os.Stat(volumePath); os.IsNotExist(err) {
		return fmt.Errorf("volume does not exist: %s", volumePath)
	}

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

	if _, err := os.Stat(volumePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("volume does not exist: %s", volumePath)
	}

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

	return &VolumeInfo{
		Name:  volumePath,
		Quota: "none",
		Used:  fmt.Sprintf("%d", size),
	}, nil
}
