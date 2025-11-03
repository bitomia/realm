package api

import (
	"fmt"
	"log/slog"

	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/cruntime"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/daemon/volumes"
	"github.com/bitomia/realm/internal/requests"
)

// ListContainers returns a list of all containers with their status
func ListContainers() (map[string]containers.ContainerInfo, error) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	containersList, err := client.ContainerService().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	db := db.GetDB()
	containersState := make(map[string]containers.ContainerInfo)

	for _, c := range containersList {
		var entry containers.ContainerInfo
		dbContainerEntry, err := db.GetContainer(c.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get container %s from DB: %w", c.ID, err)
		}
		entry.DBEntry.LastState = dbContainerEntry.LastState

		ctr, err := client.LoadContainer(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load container %s: %w", c.ID, err)
		}

		// Get volume info if available
		if volumeInfo, err := volumes.GetVolumeInfo(c.ID); err != nil {
			slog.Info("Error retrieving volume info", "containerID", c.ID, "error", err.Error())
		} else {
			entry.VolumeInfo = *volumeInfo
		}

		task, err := ctr.Task(ctx, nil)
		if err != nil {
			entry.Container = c
			entry.Status = "not running"
			containersState[c.ID] = entry
			continue
		}

		status, err := task.Status(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get status for container %s: %w", c.ID, err)
		}

		entry.Container = c
		entry.Status = string(status.Status)
		containersState[c.ID] = entry
	}

	return containersState, nil
}

// CreateContainer creates a new container with the given name and options
func CreateContainer(containerName string, opts requests.CreateContainerOpts) error {
	if err := containers.CreateContainer(containerName, opts, nil); err != nil {
		return fmt.Errorf("failed to create container %s: %w", containerName, err)
	}
	return nil
}

// UpdateContainerState updates the state of a container
func UpdateContainerState(containerName string, opts containers.UpdateContainerOpts) error {
	if err := containers.UpdateContainerState(containerName, opts); err != nil {
		return fmt.Errorf("failed to update container state %s: %w", containerName, err)
	}
	return nil
}

// RemoveContainer removes a container
func RemoveContainer(containerName string, opts containers.DeleteContainerOpts) error {
	if err := containers.DeleteContainer(containerName, opts, 15, true, true); err != nil {
		return fmt.Errorf("failed to delete container %s: %w", containerName, err)
	}
	return nil
}
