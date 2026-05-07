package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bitomia/realm/common/dto"
)

type Container struct {
	ContainerName string `json:"container_name"`
	Image         string `json:"image"`
}

func (db *AgentDB) GetAllContainers() ([]Container, error) {
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

func (db *AgentDB) GetContainer(containerName string) (Container, error) {
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
		return Container{}, fmt.Errorf("container %s not found", containerName)
	}

	var container Container
	if err := json.Unmarshal([]byte(value), &container); err != nil {
		slog.Error("Error unmarshaling container", "error", err.Error())
		return Container{}, err
	}
	return container, nil
}

func (db *AgentDB) CreateContainer(containerName string, image string, state dto.ContainerStatus) (Container, error) {
	container := Container{
		ContainerName: containerName,
		Image:         image,
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

	err = db.createIfNotExists(containerKey, string(value))
	if err != nil {
		slog.Error("Error on CreateContainer", "error", err.Error())
		return Container{}, err
	}

	return container, nil
}

func (db *AgentDB) UpdateContainerImage(containerName string, image string) (string, error) {
	slog.Info("db.UpdateContainerImage", "container", containerName, "image", image)

	containerKey, err := db.containerKey(containerName)
	if err != nil {
		slog.Error("Error getting container key", "error", err.Error())
		return "", err
	}

	err = db.OptimisticUpdate(containerKey, func(currentValue []byte) ([]byte, error) {
		var container Container
		if err := json.Unmarshal(currentValue, &container); err != nil {
			slog.Error("Error unmarshaling container", "error", err.Error())
			return nil, err
		}

		container.Image = image

		value, err := json.Marshal(container)
		if err != nil {
			slog.Error("Error marshaling container", "error", err.Error())
			return nil, err
		}
		return value, nil
	})

	if err != nil {
		return "", err
	}

	return image, nil
}

func (db *AgentDB) DeleteContainer(containerName string) error {
	containerKey, err := db.containerKey(containerName)
	if err != nil {
		return err
	}
	return db.delete(containerKey)
}
