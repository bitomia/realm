package loads

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/cruntime"

	"github.com/bitomia/realm/internal/dto"
)

const ContainerDriverID common.LoadDriverID = "container"

type ContainerConfig struct {
	Image            string           `json:"image"`
	Env              []string         `json:"env"`
	Quotas           *dto.Quotas      `json:"quotas"`
	VolumeMountPoint string           `json:"volume_mount_point"`
	MountVolume      *dto.MountVolume `json:"mount_volume"`
}

type ContainerDriver struct {
	Config ContainerConfig
}

func NewContainerDriverFromConfig(c any) (common.LoadDriver, error) {
	var config ContainerConfig

	// Configure mapstructure decoder to use 'json' tags
	// because it has to work for config files (yaml)
	// and for remote commands (json)
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &config,
	})
	if err != nil {
		return nil, err
	}
	if err := decoder.Decode(c); err != nil {
		return nil, err
	}

	driver := &ContainerDriver{
		Config: config,
	}

	if err := driver.Verify(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (c ContainerDriver) DriverInfo() common.LoadDriverInfo {
	return common.LoadDriverInfo{
		ID:  ContainerDriverID,
		New: NewContainerDriverFromConfig,
	}
}

func (c ContainerDriver) GetLoadDriverID() common.LoadDriverID {
	return ContainerDriverID
}

func (c ContainerDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.GetDriverConfig())
}

func (c ContainerDriver) UnmarshalJSON(data []byte) error {
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if loadDriver, err := NewContainerDriverFromConfig(config); err != nil {
		return err
	} else {
		c = loadDriver.(ContainerDriver)
		return nil
	}

}

func (c ContainerDriver) Verify() error {
	if c.Config.Image == "" {
		return fmt.Errorf("Container image not specified")
	}
	return nil
}

func (c ContainerDriver) PlanDaemon(repository common.DeploymentsRepository, loadName string) error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		slog.Error("ContainerDriver.PlanDaemon", "error", err)
		return err
	}
	defer client.Close()

	_, err = containers.TryPullAndGetImage(ctx, client, c.Config.Image)
	if err != nil {
		slog.Error("ContainerDriver.PlanDaemon", "error", err)
	}

	return nil
}

func (c ContainerDriver) StartOnDaemon(repository common.DeploymentsRepository, loadName string) (common.DeploymentID, error) {
	// Use loadName to create an unique container namey
	containerName := fmt.Sprintf("%s-%s", loadName, uuid.New())
	slog.Info("ContainerDriver.StartOnDaemon", "msg", "starting container", "container", containerName)

	createOpts := dto.CreateContainerRequest{
		Image:  c.Config.Image,
		Quotas: c.Config.Quotas,
		Env:    c.Config.Env,
	}

	if err := containers.CreateContainer(containerName, createOpts, nil); err != nil {
		slog.Error("ContainerDriver.StartOnDaemon", "msg", "create container failed", "error", err)
		return uuid.Nil, err
	}

	updateOpts := dto.UpdateContainerOpts{
		State: common.LoadStart,
	}
	task, err := containers.UpdateContainerState(containerName, updateOpts)
	if err != nil {
		slog.Error("ContainerDriver.StartOnDaemon", "msg", "update container state failed. rolling back...", "error", err)

		// Delete container if it failed
		deleteOpts := dto.DeleteContainerOpts{
			RemoveVolume: true,
		}
		err := containers.DeleteContainer(containerName, deleteOpts, syscall.SIGKILL, true, true)
		if err != nil {
			slog.Error("ContainerDriver.StartOnDaemon", "msg", "delete container on rolling back failed", "error", err)
		}
		return uuid.Nil, err
	}

	// TODO network options

	slog.Info("ContainerDriver.StartOnDaemon", "msg", "container started", "container", containerName, "taskPID", task.Pid())

	// Create deployment entry in repository
	did, err := repository.Create(loadName, c)
	if err != nil {
		slog.Error("ContainerDriver.StartOnDaemon", "msg", "failed to create deployment", "error", err)
		return uuid.Nil, err
	}

	return did, nil
}

func (c ContainerDriver) StopOnDaemon(repository common.DeploymentsRepository, deployment common.Deployment) error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		slog.Error("ContainerDriver.StopOnDaemon", "error", err)
		return err
	}
	defer client.Close()

	// Use loadName as the container name
	containerName := deployment.LoadName

	// Load the container
	container, err := client.LoadContainer(ctx, containerName)
	if err != nil {
		slog.Error("ContainerDriver.StopOnDaemon", "msg", "failed to load container", "container", containerName, "error", err)
		return fmt.Errorf("failed to load container %s: %w", containerName, err)
	}

	// Get the task and stop it
	task, err := container.Task(ctx, nil)
	if err != nil {
		slog.Warn("ContainerDriver.StopOnDaemon", "msg", "no task found for container", "container", containerName)
	} else {
		// Kill the task with SIGTERM
		if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
			slog.Error("ContainerDriver.StopOnDaemon", "msg", "failed to kill task", "container", containerName, "error", err)
		}

		// Wait for the task to exit
		statusC, err := task.Wait(ctx)
		if err != nil {
			slog.Error("ContainerDriver.StopOnDaemon", "msg", "failed to wait for task", "container", containerName, "error", err)
		} else {
			status := <-statusC
			if status.Error() != nil {
				slog.Error("ContainerDriver.StopOnDaemon", "msg", "task exited with error", "container", containerName, "error", status.Error())
			}
		}

		// Delete the task
		if _, err := task.Delete(ctx); err != nil {
			slog.Error("ContainerDriver.StopOnDaemon", "msg", "failed to delete task", "container", containerName, "error", err)
		}
	}

	// Delete the container
	if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		slog.Error("ContainerDriver.StopOnDaemon", "msg", "failed to delete container", "container", containerName, "error", err)
		return fmt.Errorf("failed to delete container %s: %w", containerName, err)
	}

	slog.Info("ContainerDriver.StopOnDaemon", "msg", "container stopped and deleted", "container", containerName)

	// Delete deployment from repository
	if err := repository.DeleteDeployment(deployment.ID); err != nil {
		slog.Error("ContainerDriver.StopOnDaemon", "msg", "failed to delete deployment", "deploymentID", deployment.ID, "error", err)
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

func (c ContainerDriver) GetDriverConfig() common.LoadDriverConfig {
	return common.LoadDriverConfig{Driver: ContainerDriverID, DriverConfig: c.Config}
}
