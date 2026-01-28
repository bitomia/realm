package containers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/daemon/cruntime"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/daemon/network"
	"github.com/bitomia/realm/daemon/volumes"
)

type DBContainerEntry struct {
	LastState common.LoadState `json:"last_state"`
}

type ContainerInfo struct {
	Container  containers.Container `json:"container"`
	Status     string               `json:"status"`
	DBEntry    DBContainerEntry     `json:"db_entry"`
	VolumeInfo volumes.VolumeInfo   `json:"volume_info"`
}

type ContainerError struct {
	Code    int
	Message string
}

func (e *ContainerError) Error() string {
	return fmt.Sprintf("Error %d: %s", e.Code, e.Message)
}

func NewError(code int, format string, a ...any) *ContainerError {
	return &ContainerError{code, fmt.Sprintf(format, a...)}
}

// GetContainerdVersion verifies that containerd version is accessible.
// Returns the version information if successful, or nil if connection fails.
func GetContainerdVersion() (*containerd.Version, error) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		slog.Error("Failed to create containerd client", "error", err.Error())
		return nil, err
	}
	defer client.Close()

	version, err := client.Version(ctx)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func RepairContainer(c db.Container, stdoutPath string, stderrPath string) error {
	// Plan:
	// Check first if the container is in the expected state:
	// 1. containers.last_status == containerd status

	database := db.GetDB()
	containerRow, err := database.GetContainer(c.ContainerName)
	if err != nil {
		return err
	}

	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return err
	}
	defer client.Close()

	container, err := client.LoadContainer(ctx, c.ContainerName)
	if err != nil {
		return err
	}

	var status containerd.Status
	status.Status = containerd.Unknown

	task, err := container.Task(ctx, nil)
	if task != nil {
		status, _ = task.Status(ctx)
	}

	shall_restart := (containerRow.LastState == common.LoadStart || containerRow.LastState == common.LoadStartFailed) && status.Status != containerd.Running
	shall_stop := (containerRow.LastState == common.LoadStop || containerRow.LastState == common.LoadStopFailed) && (status.Status == containerd.Running || status.Status == containerd.Paused || status.Status == containerd.Pausing)

	if shall_restart {
		slog.Info("Restarting container", "container", c.ContainerName)
		err = stopContainer(c.ContainerName, syscall.SIGTERM)
		if err != nil {
			return err
		}
		_, err = startContainer(c.ContainerName, stdoutPath, stderrPath)
		if err != nil {
			return err
		}
		return network.RepairNetwork(c.ContainerName)
	} else if shall_stop {
		slog.Info("Stopping container", "container", c.ContainerName)
		return stopContainer(c.ContainerName, syscall.SIGTERM)
	} else {
		slog.Info("Container state matches, doing nothing", "container", c.ContainerName)
	}
	return nil

	// 2. network survives reboot?
	// 3. subnet survives reboot?
	// 4. caddy config survives reboot?
}

func CreateContainer(containerName string, opts dto.CreateContainerRequest, extraSpecOpts []oci.SpecOpts) error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return fmt.Errorf("Cannot create cruntime client: %s: %s", containerName, err.Error())
	}
	defer client.Close()

	image, err := TryPullAndGetImage(ctx, client, opts.Image)
	if err != nil {
		return fmt.Errorf("Failed to pull image %s: %s", opts.Image, err.Error())
	}

	var container containerd.Container = nil

	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithEnv(opts.Env),
	}

	if extraSpecOpts != nil {
		specOpts = append(specOpts, extraSpecOpts...)
	}

	if opts.MountVolume != nil {
		for _, mount := range *opts.MountVolume {
			if mount.VolumeMountPoint == "" {
				slog.Warn("CreateContainer", "msg", "Skipping due to mount volume point cannot be empty.")
				continue
			}

			var mountSource string = ""
			var volumeName = fmt.Sprintf("%s-%s", containerName, uuid.New())

			if volumes.IsVolume(volumeName) {
				slog.Info("CreateContainer", "msg", "reusing existent volume", "volume", volumeName)
				mountSource, err = volumes.MountVolume(volumeName)
				if err != nil {
					slog.Error("CreateContainer", "msg", "error on mounting to reuse volume for container", "volume", volumeName, "error", err)
					return fmt.Errorf("Failed to reuse volume %s: %s", volumeName, err.Error())
				}
			} else {
				err = volumes.CreateVolume(volumeName)
				if err != nil {
					return fmt.Errorf("Failed to create volume %s: %s", volumeName, err.Error())
				}

				mountSource, err = volumes.MountVolume(volumeName)
				if err != nil {
					return fmt.Errorf("Error on mounting volume %s: %s", volumeName, err.Error())
				}
			}

			// Set volume quota
			if mount.VolumeSize != nil {
				if err := volumes.SetVolumeQuota(volumeName, *mount.VolumeSize); err != nil {
					return fmt.Errorf("Failed to enable volume quota for %s: %s", volumeName, err.Error())
				}

				slog.Info("CreateContainer", "volume", volumeName, "volumeSize", *mount.VolumeSize)
			}

			if len(mountSource) == 0 {
				return fmt.Errorf("Failed to create volume %s: Unexpected condition mountSource empty", volumeName)
			}

			mountOptions := []specs.Mount{
				{
					Type:        "bind",
					Source:      mountSource,
					Destination: mount.VolumeMountPoint,
					Options:     []string{"rw", "rbind", "mode=755"},
				},
			}
			specOpts = append(specOpts, oci.WithMounts(mountOptions))
		}
	}

	if len(opts.BindMounts) > 0 {
		for _, bindMount := range opts.BindMounts {
			if bindMount.Source == "" || bindMount.Destination == "" {
				err := fmt.Errorf("bind mount: source and destination are required")
				slog.Error("CreateContainer", "error", err)
				return err
			}

			mountOpts := []string{"rbind"}
			if bindMount.ReadOnly {
				mountOpts = append(mountOpts, "ro")
			} else {
				mountOpts = append(mountOpts, "rw")
			}

			bindMountSpec := []specs.Mount{
				{
					Type:        "bind",
					Source:      bindMount.Source,
					Destination: bindMount.Destination,
					Options:     mountOpts,
				},
			}
			specOpts = append(specOpts, oci.WithMounts(bindMountSpec))
			slog.Info("CreateContainer", "msg", "Added bind mount", "source", bindMount.Source, "destination", bindMount.Destination, "readonly", bindMount.ReadOnly)
		}
	}

	if opts.Quotas != nil {
		if opts.Quotas.MemLimit != nil {
			memLimit := *opts.Quotas.MemLimit * 1024 * 1024
			slog.Info("CreateContainer", "container", containerName, "memLimit", memLimit)
			specOpts = append(specOpts, oci.WithMemoryLimit(memLimit))
		}
		if opts.Quotas.CpuShares != nil {
			specOpts = append(specOpts, oci.WithCPUShares(*opts.Quotas.CpuShares))
		}
		if opts.Quotas.CpuCFS != nil {
			specOpts = append(specOpts, oci.WithCPUCFS(opts.Quotas.CpuCFS.CpuQuota, opts.Quotas.CpuCFS.CpuPeriod))
		}
	}

	container, err = client.NewContainer(
		ctx,
		containerName,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(containerName, image),
		containerd.WithNewSpec(specOpts...),
	)

	if err != nil {
		return fmt.Errorf("Failed to create new container %s with image %s: %s", containerName, opts.Image, err.Error())
	}
	if container == nil {
		return errors.New("Unexpected condition container not existant")
	}

	database := db.GetDB()
	database.CreateContainer(containerName, opts.Image, "")

	return nil
}

func DeleteContainer(containerName string, opts dto.DeleteContainerOpts, signal syscall.Signal, shallRemoveDBEntry bool, shallRemoveNetwork bool) *ContainerError {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return NewError(0, "Cannot create cruntime client: %s - %s", containerName, err.Error())
	}
	defer client.Close()

	container, err := client.LoadContainer(ctx, containerName)
	if err != nil {
		return NewError(1, "Failed to retrieve container %s on remove: %s", containerName, err.Error())
	}

	pid, err := cruntime.GetContainerTaskPID(ctx, client, containerName)
	if shallRemoveNetwork && err == nil {
		if err = network.DeleteNetworkConfig(ctx, containerName, pid); err != nil {
			return NewError(2, "Error while trying to remove network on container deletion %s: %s", containerName, err.Error())
		}
	} else {
		slog.Info("Network not removed for container because task PID not available", "container", containerName)
	}

	err = tryDeleteContainerTask(ctx, container, signal)
	if err != nil {
		return NewError(3, "Failed to delete task for container %s on remove: %s", containerName, err.Error())
	}

	if opts.RemoveSnapshots {
		if err := removeContainerSnapshots(ctx, client, container); err != nil {
			slog.Error("removeContainer deleting snapshots failed", "container", containerName, "error", err)
		}
	}

	if err = client.ContainerService().Delete(ctx, containerName); err != nil {
		return NewError(4, "Error while trying to remove container %s: %s", container, err.Error())
	}

	if opts.RemoveVolume {
		// Try with a deferred delete
		err = volumes.DeleteVolume(containerName, false)
		if err != nil {
			slog.Error("removeContainer deleting volume failed", "container", containerName, "error", err)
		} else {
			slog.Error("removeContainer volume deleted for container", "container", containerName)
		}
	}

	if shallRemoveDBEntry {
		database := db.GetDB()
		if err = database.DeleteContainer(containerName); err != nil {
			slog.Error("Error while trying to delete container from DB", "container", container, "error", err.Error())
		}
	}
	return nil
}

func UpdateContainerState(containerName string, opts dto.UpdateContainerOpts) (containerd.Task, error) {
	switch opts.State {
	case common.LoadStart:
		database := db.GetDB()
		if task, err := startContainer(containerName, opts.StdoutPath, opts.StderrPath); err != nil {
			database.UpdateContainerState(containerName, common.LoadStartFailed)
			return nil, err
		} else {
			database.UpdateContainerState(containerName, common.LoadStart)
			return task, nil
		}
	case common.LoadStop:
		database := db.GetDB()
		if err := stopContainer(containerName, syscall.SIGTERM); err != nil {
			database.UpdateContainerState(containerName, common.LoadStopFailed)
		} else {
			database.UpdateContainerState(containerName, common.LoadStop)
		}
	default:
		return nil, fmt.Errorf("Unknown container state: %s", opts.State)
	}
	return nil, fmt.Errorf("Unexpected condition on UpdateContainerState")
}

func SendSignal(containerName string, signal syscall.Signal) error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return err
	}
	defer client.Close()

	container, err := client.LoadContainer(ctx, containerName)
	if err != nil {
		slog.Error("Failed to retrieve container on SendSignal", "container", containerName, "error", err.Error())
		return err
	}
	task, _ := container.Task(ctx, nil)
	if task != nil {
		task.Kill(ctx, signal)
		statusC, err := task.Wait(ctx)
		if err != nil {
			return err
		}
		status := <-statusC
		if status.Error() != nil {
			return status.Error()
		}
		_, err = task.Delete(ctx)
		return err
	}
	return nil
}

func removeContainerSnapshots(ctx context.Context, c *containerd.Client, container containerd.Container) error {
	info, err := container.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container info: %w", err)
	}
	snapshotKey := info.SnapshotKey
	snapshotter := info.Snapshotter
	if snapshotKey == "" {
		slog.Info("No snapshots found for container", "containerID", container.ID())
		return nil
	}
	slog.Info("Removing snapshot using snapshotter", "snapshotKey", snapshotKey, "snapshotter", snapshotter)
	err = c.SnapshotService(snapshotter).Remove(ctx, snapshotKey)
	if err != nil {
		return fmt.Errorf("failed to remove snapshot: %w", err)
	}

	slog.Info("Snapshot removed successfully", "containerID", container.ID())
	return nil
}

func TryPullAndGetImage(ctx context.Context, client *containerd.Client, imageName string) (containerd.Image, error) {
	images, err := client.ImageService().List(ctx, fmt.Sprintf("name==%s", imageName))
	if err != nil {
		slog.Error("TryPullAndGetImage", "error", err)
		return nil, err
	}

	if len(images) == 0 {
		slog.Info("TryPullAndGetImage", "msg", "pulling image", "image", imageName)

		pullOpts := cruntime.GetPullOptions(&config.Get().Daemon)
		image, err := client.Pull(ctx, imageName, pullOpts...)
		if err != nil {
			slog.Error("TryPullAndGetImage", "error", err)
			return nil, err
		}

		slog.Info("TryPullAndGetImage", "msg", "image pulled", "name", image.Name())
		return image, nil
	}

	return client.GetImage(ctx, imageName)
}
