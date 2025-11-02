package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bitomia/realm/internal/types"
)

type Container struct {
	ContainerName string               `json:"container_name"`
	Image         string               `json:"image"`
	LastState     types.ContainerState `json:"last_state"`
}

func (db *DaemonDB) GetAllContainers() ([]Container, error) {
	data, err := db.getKey(containerPrefix)
	if err != nil {
		slog.Error("Error on GetAllContainers", "error", err.Error())
		return nil, err
	}

	var containers []Container
	for _, value := range data {
		var container Container
		if err := json.Unmarshal([]byte(value), &container); err != nil {
			slog.Error("Error unmarshaling container", "error", err.Error())
			continue
		}
		containers = append(containers, container)
	}
	return containers, nil
}

func (db *DaemonDB) GetContainer(containerName string) (Container, error) {
	if containerName == "" {
		return Container{}, errors.New("container name cannot be empty")
	}

	containerKey, err := db.containerKey(containerName)
	if err != nil {
		slog.Error("Error getting container key", "error", err.Error())
		return Container{}, err
	}

	value, err := db.get(containerKey)
	if err != nil {
		slog.Error("Error on GetContainer", "error", err.Error())
		return Container{}, fmt.Errorf("Container %s not found", containerName)
	}

	var container Container
	if err := json.Unmarshal([]byte(value), &container); err != nil {
		slog.Error("Error unmarshaling container", "error", err.Error())
		return Container{}, err
	}
	return container, nil
}

func (db *DaemonDB) CreateContainer(containerName string, image string, owner string, state types.ContainerState) (Container, error) {
	container := Container{
		ContainerName: containerName,
		Image:         image,
		LastState:     state,
	}

	value, err := json.Marshal(container)
	if err != nil {
		slog.Error("Error marshaling container", "error", err.Error())
		return Container{}, err
	}

	containerKey, err := db.containerKey(containerName)
	if err != nil {
		slog.Error("Error getting container key", "error", err.Error())
		return Container{}, err
	}

	err = db.put(containerKey, string(value))
	if err != nil {
		slog.Error("Error on CreateContainer", "error", err.Error())
		return Container{}, err
	}

	return container, nil
}

func (db *DaemonDB) UpdateContainerState(containerName string, state types.ContainerState) (types.ContainerState, error) {
	slog.Info("db.UpdateContainerState", "container", containerName, "state", state)

	// Get existing container
	container, err := db.GetContainer(containerName)
	if err != nil {
		return "", err
	}

	// Update state
	container.LastState = state

	// Save back to etcd
	value, err := json.Marshal(container)
	if err != nil {
		slog.Error("Error marshaling container", "error", err.Error())
		return "", err
	}

	containerKey, err := db.containerKey(containerName)
	if err != nil {
		slog.Error("Error getting container key", "error", err.Error())
		return "", err
	}

	err = db.put(containerKey, string(value))
	if err != nil {
		slog.Error("Error on UpdateContainerState", "error", err.Error())
		return "", err
	}

	return state, nil
}

func (db *DaemonDB) UpdateContainerImage(containerName string, image string) (string, error) {
	slog.Info("db.UpdateContainerImage", "container", containerName, "image", image)

	// Get existing container
	container, err := db.GetContainer(containerName)
	if err != nil {
		return "", err
	}

	// Update image
	container.Image = image

	// Save back to etcd
	value, err := json.Marshal(container)
	if err != nil {
		slog.Error("Error marshaling container", "error", err.Error())
		return "", err
	}

	containerKey, err := db.containerKey(containerName)
	if err != nil {
		slog.Error("Error getting container key", "error", err.Error())
		return "", err
	}

	err = db.put(containerKey, string(value))
	if err != nil {
		slog.Error("Error on UpdateContainerImage", "error", err.Error())
		return "", err
	}

	return image, nil
}

func (db *DaemonDB) DeleteContainer(containerName string) error {
	containerKey, err := db.containerKey(containerName)
	if err != nil {
		return err
	}
	return db.delete(containerKey)
}
