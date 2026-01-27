package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/gorilla/mux"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/daemon/api"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/cruntime"
)

func ListContainersHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("ListContainersHandler")

	w.Header().Set("Content-Type", "application/json")

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

	var opts dto.UpdateContainerOpts
	json.NewDecoder(r.Body).Decode(&opts)

	if err := api.UpdateContainerState(containerName, opts); err != nil {
		slog.Error("UpdateContainerState", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func RemoveContainerHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	slog.Info("RemoveContainerHandler", "container", containerName)

	var opts dto.DeleteContainerOpts
	json.NewDecoder(r.Body).Decode(&opts)

	if err := api.RemoveContainer(containerName, opts); err != nil {
		slog.Error("DeleteContainer", "container", containerName, "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
