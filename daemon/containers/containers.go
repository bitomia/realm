package containers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/oci"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/daemon/config"
	"github.com/bitomia/realm/daemon/cruntime"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/daemon/volumes"
)

// TODO remove
type DBContainerEntry struct {
}

type ContainerInfo struct {
	Container  containers.Container `json:"container"`
	Status     string               `json:"status"`
	DBEntry    DBContainerEntry     `json:"db_entry"`
	VolumeInfo volumes.VolumeInfo   `json:"volume_info"`
}

// GetHostNetworkSpecOpts returns OCI spec options for host network mode
func GetHostNetworkSpecOpts() []oci.SpecOpts {
	return []oci.SpecOpts{
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithHostHostsFile,
		oci.WithHostResolvconf,
	}
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

	var container containerd.Container

	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithEnv(opts.Env),
	}

	// Handle entrypoint and args
	// If entrypoint is set, combine it with args (if any)
	if opts.Entrypoint != nil {
		var processArgs = []string{*opts.Entrypoint}
		if len(opts.Args) > 0 {
			processArgs = append(processArgs, opts.Args...)
		}
		specOpts = append(specOpts, oci.WithProcessArgs(processArgs...))
	} else if len(opts.Args) > 0 {
		specOpts = append(specOpts, oci.WithProcessArgs(opts.Args...))
	}

	// Handle working directory
	if opts.WorkingDir != nil {
		specOpts = append(specOpts, oci.WithProcessCwd(*opts.WorkingDir))
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

			var mountSource string
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
					slog.Warn("CreateContainer", "msg", "failed to set volume quota, continuing without quota", "volume", volumeName, "error", err)
				} else {
					slog.Info("CreateContainer", "volume", volumeName, "volumeSize", *mount.VolumeSize)
				}
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
		return errors.New("Unexpected condition container not existent")
	}

	database := db.GetDB()
	if _, err := database.CreateContainer(containerName, opts.Image, ""); err != nil {
		return fmt.Errorf("Failed to create container %s in database: %s", containerName, err.Error())
	}

	return nil
}

func DeleteContainer(containerName string, signal syscall.Signal, shallRemoveVolume bool) error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return fmt.Errorf("Cannot create cruntime client: %s - %s", containerName, err.Error())
	}
	defer client.Close()

	container, err := client.LoadContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("Failed to retrieve container %s on remove: %s", containerName, err.Error())
	}

	task, err := container.Task(ctx, nil)
	// skip not found errors because we want to delete the container and associated resources, and
	// in this case we can ignore when the task is already not found
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("failed to retrieve container task %s: %w", containerName, err)
	}

	if task != nil {
		if err := task.Kill(ctx, signal); err != nil {
			slog.Error("ContainerDriver.DeleteContainer", "msg", "failed to kill task", "container", containerName, "error", err)
		}

		statusC, err := task.Wait(ctx)
		if err != nil {
			slog.Error("ContainerDriver.DeleteContainer", "msg", "failed to wait for task", "container", containerName, "error", err)
		} else {
			status := <-statusC
			if status.Error() != nil {
				slog.Error("ContainerDriver.DeleteContainer", "msg", "task exited with error", "container", containerName, "error", status.Error())
			}
		}

		if _, err := task.Delete(ctx); err != nil {
			slog.Error("ContainerDriver.DeleteContainert", "msg", "failed to delete task", "container", containerName, "error", err)
		}
	}

	if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		return fmt.Errorf("failed to delete container %s: %w", containerName, err)
	}

	if shallRemoveVolume {
		err = volumes.DeleteVolume(containerName, false)
		if err != nil {
			slog.Warn("removeContainer deleting volume failed", "container", containerName, "error", err)
		} else {
			slog.Info("removeContainer volume deleted for container", "container", containerName)
		}
	}

	database := db.GetDB()
	if err = database.DeleteContainer(containerName); err != nil {
		slog.Error("Error while trying to delete container from DB", "container", container, "error", err.Error())
		return err
	}

	return nil
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
		_ = task.Kill(ctx, signal)
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
