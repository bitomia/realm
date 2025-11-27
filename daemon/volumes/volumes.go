package volumes

import (
	"errors"
	"log"
	"path/filepath"

	"github.com/bitomia/realm/config"
)

type VolumeInfo struct {
	Name  string
	Quota string
	Used  string
}

func GetVolumesPath() (string, error) {
	volumesPath := config.Get().Daemon.VolumesPool
	if volumesPath == "" {
		return "", errors.New("REALM_VOLUMES_POOL not found")
	}
	return volumesPath, nil
}

func GetPathForVolume(volume string) (string, error) {
	volumesPath, err := GetVolumesPath()
	if err != nil {
		log.Printf("GetPathForVolume failed: %v\n", err)
		return "", err
	}
	return filepath.Join(volumesPath, volume), nil
}
