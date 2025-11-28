package main

/*
#include <stdlib.h>
#include <string.h>

typedef struct {
    char* container_id;
    double cpu_usage;
    double cpu_system;
    double cpu_user;
    double memory_usage;
    double memory_limit;
    double memory_percent;
} ContainerStateResponse;

typedef struct {
    int num_cpu;
    unsigned long long user_cpu;
    unsigned long long idle_cpu;
    unsigned long long system_cpu;
    unsigned long long total_cpu;
    double usage_cpu_percent;
    unsigned long long total_mem;
    unsigned long long used_mem;
    unsigned long long free_mem;
    double free_mem_percent;
    unsigned long long free_storage;
    ContainerStateResponse* containers;
    int containers_count;
} NodeStateResponse;
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"github.com/bitomia/realm/daemon"
	"github.com/bitomia/realm/daemon/api"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/internal/config"
	"github.com/bitomia/realm/internal/drivers"
	"github.com/bitomia/realm/internal/dto"
)

/**
 * Start a daemon instance using the realm YAML config file found in the working dir
 */
//export StartDaemon
func StartDaemon() {
	drivers.RegisterStdDrivers()
	daemon.Start()
}

/**
 * Get daemon build version
 * @return build version
 */
//export GetVersion
func GetVersion() *C.char {
	return C.CString(config.GetVersion())
}

/**
 * free allocated C string
 */
//export Realm_Free
func Realm_Free(p *C.char) {
	if p != nil {
		C.free(unsafe.Pointer(p))
	}
}

/**
 * Get health status of the daemon
 * @return JSON response with health status
 */
//export GetHealthStatus
func GetHealthStatus() *C.char {
	status, err := api.GetHealthStatus()
	if err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, status, ""))
}

/**
 * Get node status information
 * @return JSON response with node status
 */
//export GetNodeStatus
func GetNodeStatus() *C.NodeStateResponse {
	state, err := api.GetNodeState()
	if err != nil || state == nil {
		return nil
	}

	// Allocate NodeStateResponse in C heap
	resp := (*C.NodeStateResponse)(C.malloc(C.size_t(C.sizeof_NodeStateResponse)))
	if resp == nil {
		return nil
	}
	resp.num_cpu = C.int(state.NumCPU)
	resp.user_cpu = C.ulonglong(state.UserCPU)
	resp.idle_cpu = C.ulonglong(state.IdleCPU)
	resp.system_cpu = C.ulonglong(state.SystemCPU)
	resp.total_cpu = C.ulonglong(state.TotalCPU)
	resp.usage_cpu_percent = C.double(state.UsageCPUPercent)
	resp.total_mem = C.ulonglong(state.TotalMem)
	resp.used_mem = C.ulonglong(state.UsedMem)
	resp.free_mem = C.ulonglong(state.FreeMem)
	resp.free_mem_percent = C.double(state.FreeMemPercent)
	resp.free_storage = C.ulonglong(state.FreeStorage)
	resp.containers = nil
	resp.containers_count = C.int(0)

	// Allocate containers array if present
	if len(state.Containers) > 0 {
		count := C.int(len(state.Containers))
		bytes := C.size_t(C.sizeof_ContainerStateResponse) * C.size_t(count)

		cArr := (*C.ContainerStateResponse)(C.malloc(bytes))
		if cArr == nil {
			return resp // no containers
		}

		// Turn cArr into a Go slice view
		containers := (*[1 << 30]C.ContainerStateResponse)(
			unsafe.Pointer(cArr),
		)[:count:count]

		for i, c := range state.Containers {
			containers[i].container_id = C.CString(c.ContainerID)
			containers[i].cpu_usage = C.double(c.CPUUsage)
			containers[i].cpu_system = C.double(c.CPUSystem)
			containers[i].cpu_user = C.double(c.CPUUser)
			containers[i].memory_usage = C.double(c.MemoryUsage)
			containers[i].memory_limit = C.double(c.MemoryLimit)
			containers[i].memory_percent = C.double(c.MemoryPercent)
		}

		resp.containers = cArr
		resp.containers_count = count
	}
	return resp
}

//export FreeNodeStateResponse
func FreeNodeStateResponse(resp *C.NodeStateResponse) {
	if resp == nil {
		return
	}
	if resp.containers != nil && resp.containers_count > 0 {
		for i := 0; i < int(resp.containers_count); i++ {
			entry := (*C.ContainerStateResponse)(unsafe.Pointer(uintptr(unsafe.Pointer(resp.containers)) + uintptr(i)*uintptr(C.sizeof_ContainerStateResponse)))
			if entry.container_id != nil {
				C.free(unsafe.Pointer(entry.container_id))
			}
		}
		C.free(unsafe.Pointer(resp.containers))
	}
	C.free(unsafe.Pointer(resp))
}

/**
 * List all containers managed by the daemon
 * @return JSON response with list of containers
 */
//export ListContainers
func ListContainers() *C.char {
	containersList, err := api.ListContainers()
	if err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, containersList, ""))
}

/**
 * Create a new container
 * @param containerName Name of the container to create
 * @param optsJSON JSON string with container creation options
 * @return JSON response with creation result
 */
//export CreateContainer
func CreateContainer(containerName *C.char, optsJSON *C.char) *C.char {
	goContainerName := C.GoString(containerName)
	goOptsJSON := C.GoString(optsJSON)

	var opts dto.CreateContainerRequest
	if err := json.Unmarshal([]byte(goOptsJSON), &opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, "invalid JSON options: "+err.Error()))
	}

	if err := api.CreateContainer(goContainerName, opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, map[string]string{"container": goContainerName}, ""))
}

/**
 * Update the state of an existing container
 * @param containerName Name of the container to update
 * @param optsJSON JSON string with container update options
 * @return JSON response with update result
 */
//export UpdateContainerState
func UpdateContainerState(containerName *C.char, optsJSON *C.char) *C.char {
	goContainerName := C.GoString(containerName)
	goOptsJSON := C.GoString(optsJSON)

	var opts containers.UpdateContainerOpts
	if err := json.Unmarshal([]byte(goOptsJSON), &opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, "invalid JSON options: "+err.Error()))
	}

	if err := api.UpdateContainerState(goContainerName, opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, map[string]string{"container": goContainerName}, ""))
}

/**
 * Remove an existing container
 * @param containerName Name of the container to remove
 * @param optsJSON JSON string with container deletion options
 * @return JSON response with removal result
 */
//export RemoveContainer
func RemoveContainer(containerName *C.char, optsJSON *C.char) *C.char {
	goContainerName := C.GoString(containerName)
	goOptsJSON := C.GoString(optsJSON)

	var opts containers.DeleteContainerOpts
	if err := json.Unmarshal([]byte(goOptsJSON), &opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, "invalid JSON options: "+err.Error()))
	}

	if err := api.RemoveContainer(goContainerName, opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, map[string]string{"container": goContainerName}, ""))
}

func main() {}
