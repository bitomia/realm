package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"syscall"

	"github.com/bitomia/realm/internal/config"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/gorilla/mux"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/bitomia/realm/daemon/api"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/cruntime"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/daemon/network"
	"github.com/bitomia/realm/daemon/volumes"
	"github.com/bitomia/realm/internal/dto"
)

func RepairContainerHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	slog.Info("RepairContainerHandler", "container", containerName)

	db := db.GetDB()
	c, error := db.GetContainer(containerName)
	if error != nil {
		http.Error(w, error.Error(), http.StatusInternalServerError)
		return
	}

	error = containers.RepairContainer(c)
	if error != nil {
		http.Error(w, error.Error(), http.StatusInternalServerError)
		return
	}
}

func ListContainersHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("ListContainersHandler")

	w.Header().Set("Content-Type", "application/json")

	// Use the new API layer
	containersState, err := api.ListContainers()
	if err != nil {
		slog.Error("Failed to list containers", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(containersState)
}

func CreateContainerHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	slog.Info("CreateContainerHandler", "container", containerName)

	var opts dto.CreateContainerRequest
	json.NewDecoder(r.Body).Decode(&opts)

	// Use the new API layer
	if err := api.CreateContainer(containerName, opts); err != nil {
		slog.Error("CreateContainer error", "container", containerName, "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func newLinuxMemory(memLimit int64) *specs.LinuxMemory {
	return &specs.LinuxMemory{
		Limit: &memLimit,
	}
}

func newLinuxCPUResources(shares *uint64, quota *int64, period *uint64) *specs.LinuxCPU {
	if shares != nil && (quota == nil || period == nil) {
		return &specs.LinuxCPU{
			Shares: shares,
		}
	} else if shares != nil && quota != nil && period != nil {
		return &specs.LinuxCPU{
			Shares: shares,
			Quota:  quota,
			Period: period,
		}
	} else {
		return nil
	}
}

func UpdateContainerQuotasHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	slog.Info("UpdateContainerQuotasHandler", "container", containerName)

	type UpdateContainerQuotas struct {
		Quotas dto.Quotas `json:"quotas"`
	}
	var updateContainerQuotas UpdateContainerQuotas
	json.NewDecoder(r.Body).Decode(&updateContainerQuotas)

	if updateContainerQuotas.Quotas.VolumeSize != nil && volumes.IsVolume(containerName) {
		if err := volumes.SetVolumeQuota(containerName, *updateContainerQuotas.Quotas.VolumeSize); err != nil {
			slog.Info("UpdateContainerQuotas: Failed to enable volume quota for container", "container", containerName, "error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		slog.Info("UpdateContainerQuotas", "container", containerName, "VolumeSize", *updateContainerQuotas.Quotas.VolumeSize)
	}

	shallUpdateLinuxResources := false
	linuxResources := specs.LinuxResources{}

	if updateContainerQuotas.Quotas.MemLimit != nil {
		memLimit := *updateContainerQuotas.Quotas.MemLimit * 1024 * 1024
		slog.Info("updateContainerQuotas", "container", containerName, "MemLimit", memLimit)
		linuxResources.Memory = newLinuxMemory(int64(memLimit))
		shallUpdateLinuxResources = true
	}

	cpuResources := newLinuxCPUResources(updateContainerQuotas.Quotas.CpuShares, &updateContainerQuotas.Quotas.CpuCFS.CpuQuota, &updateContainerQuotas.Quotas.CpuCFS.CpuPeriod)
	if cpuResources != nil {
		shallUpdateLinuxResources = true
	}

	if shallUpdateLinuxResources {
		ctx, client, err := cruntime.CreateClient()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer client.Close()

		container, err := client.LoadContainer(ctx, containerName)
		if err != nil {
			slog.Info("Failed to retrieve container on updateContainerQuotas", "container", containerName, "error", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		task, err := container.Task(ctx, nil)
		if err != nil {
			slog.Info("Failed to create new task for container on updateContainerQuotas", "container", containerName, "error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = task.Update(ctx, containerd.WithResources(linuxResources))
		if err != nil {
			fmt.Println("Error updating container resources:", err)
			return
		}
	}
}

func UpdateContainerStateHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	slog.Info("UpdateContainerStateHandler", "container", containerName)

	var opts containers.UpdateContainerOpts
	json.NewDecoder(r.Body).Decode(&opts)

	// Use the new API layer
	if err := api.UpdateContainerState(containerName, opts); err != nil {
		slog.Error("UpdateContainerState", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func RemoveContainerHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	slog.Info("RemoveContainerHandler", "container", containerName)

	var opts containers.DeleteContainerOpts
	json.NewDecoder(r.Body).Decode(&opts)

	// Use the new API layer
	if err := api.RemoveContainer(containerName, opts); err != nil {
		slog.Error("DeleteContainer", "container", containerName, "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func ReadContainerLogsHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	configLogPath := config.Get().Daemon.ContainersLogPath
	logPath := fmt.Sprintf("%s/%s.log", configLogPath, containerName)
	stdout, err := os.ReadFile(logPath)
	if err != nil {
		slog.Error("Failed to read log for container", "container", containerName, "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(stdout)
}

type SignalOpts struct {
	Signal int `json:"signal"`
}

func SendContainerSignalHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	slog.Info("SendContainerSignalHandler", "container", containerName)

	var opts SignalOpts
	json.NewDecoder(r.Body).Decode(&opts)

	if err := containers.SendSignal(containerName, syscall.Signal(opts.Signal)); err != nil {
		slog.Error("SendSignal", "container", containerName, "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type MigrateContainerOpts struct {
	Image  string   `json:"image"`
	Env    []string `json:"env,omitempty"`
	Signal int      `json:"signal,omitempty"`
}

func MigrateContainerHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	slog.Info("MigrateContainerHandler started", "container", containerName)

	var opts MigrateContainerOpts
	json.NewDecoder(r.Body).Decode(&opts)

	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		slog.Error("cruntime.CreateClient", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	slog.Info("MigrateContainerHandler. Loading container", "container", containerName)
	container, err := client.LoadContainer(ctx, containerName)
	if err != nil {
		slog.Error("LoadContainer", "container", containerName, "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	spec, err := container.Spec(ctx)
	if err != nil {
		slog.Error("Retrieving container spec", "container", containerName, "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(opts.Env) > 0 {
		spec.Process.Env = append(spec.Process.Env, opts.Env...)
	}

	slog.Info("MigrateContainerHandler. Pulling image", "container", containerName, "image", opts.Image)

	githubToken := config.Get().Daemon.GitHubRegistryToken
	resolver := docker.NewResolver(docker.ResolverOptions{
		Hosts: docker.ConfigureDefaultRegistries(docker.WithAuthorizer(docker.NewDockerAuthorizer(
			docker.WithAuthCreds(func(host string) (string, string, error) {
				if host == "ghcr.io" {
					return "USERNAME", githubToken, nil
				}
				return "", "", nil
			}),
		))),
	})
	image, err := client.Pull(ctx, opts.Image, containerd.WithPullUnpack, containerd.WithResolver(resolver))
	if err != nil {
		slog.Error("client.Pull", "container", container, "image", opts.Image, "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("-> MigrateContainerHandler %s: Imaged pulled %s", containerName, image.Name())

	// Used later to restore DNS
	networkConfig := network.GetNetworkConfig(containerName)

	slog.Info("MigrateContainerHandler. Deleting previous container", "container", containerName)

	signal := syscall.SIGTERM
	if opts.Signal != 0 {
		signal = syscall.Signal(opts.Signal)
	}
	deleteContainerOpts := containers.DeleteContainerOpts{
		RemoveVolume:    false,
		RemoveSnapshots: true,
	}
	containers.DeleteContainer(containerName, deleteContainerOpts, signal, false, false)

	slog.Info("MigrateContainerHandler. Creating new container", "container", containerName)
	_, err = client.NewContainer(
		ctx,
		containerName,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(containerName+"-snapshot", image),
		containerd.WithSpec(spec),
	)
	if err != nil {
		slog.Error("NewContainer", "container", containerName, "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("MigrateContainerHandler. Update container image in DB", "container", containerName)
	_, err = db.GetDB().UpdateContainerImage(containerName, opts.Image)
	if err != nil {
		slog.Warn("Failed on update container image. Continuing.", "container", containerName, "error", err.Error())
	}

	slog.Info("MigrateContainerHandler. Repairing to previous state", "container", containerName)
	db := db.GetDB()
	c, error := db.GetContainer(containerName)
	if error != nil {
		http.Error(w, error.Error(), http.StatusInternalServerError)
		return
	}
	error = containers.RepairContainer(c)
	if error != nil {
		slog.Error("RepairContainer", "container", containerName, "error", err.Error())
		http.Error(w, error.Error(), http.StatusInternalServerError)
		return
	}

	if len(networkConfig) > 0 {
		slog.Info("MigrateContainerHandler. Restoring DNS", "container", containerName)

		task, err := container.Task(ctx, nil)
		if err != nil {
			slog.Error("Task", "error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if task == nil {
			slog.Error("Error retrieving task", "container", containerName)
			err := fmt.Sprintf("Error retrieving task for %s", containerName)
			http.Error(w, err, http.StatusInternalServerError)
			return
		}

		pid := task.Pid()
		gw := networkConfig[0].Gateway
		slog.Info("Updating resolv.conf for container", "container", containerName, "pid", pid, "gw", gw)
		resolvConfContent := fmt.Sprintf("nameserver %s\nnameserver 8.8.8.8\nnameserver 8.8.4.4\n", gw)
		if err := network.WriteStringToResolvConf(ctx, task, resolvConfContent); err != nil {
			slog.Error("WriteStringToResolv", "container", containerName, "error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		slog.Info("MigrateContainerHandler. DNS not needed to be restored", "container", containerName)
	}

	slog.Info("MigrateContainerHandler finished", "container", containerName)
}
