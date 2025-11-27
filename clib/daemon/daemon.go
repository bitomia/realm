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

typedef const char cchar_t;
*/
import "C"

import (
	"unsafe"

	"github.com/bitomia/realm/config"
	"github.com/bitomia/realm/daemon"
	"github.com/bitomia/realm/daemon/api"
	"github.com/bitomia/realm/drivers"
)

/**
 * Start a daemon instance using the realm YAML config file found in the working dir
 */
//export StartDaemon
func StartDaemon() {
	drivers.RegisterStdDrivers()
	config.Init(nil)
	daemon.Start()
}

/**
 * Start a daemon instance using a config buffer
 */
//export StartDaemonWithConfig
func StartDaemonWithConfig(configBuffer *C.cchar_t) {
	drivers.RegisterStdDrivers()
	config.InitFromBuffer(C.GoString(configBuffer))
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
	// Needs to be freed by C
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

		// convert to Go int, not same as C.int on all platforms
		// ie: 32-bit vs 64-bit, realistically should never overflow in this case
		count_GOint := int(count)

		// Create a Go slice view over the C-allocated array using
		// the "big array trick". Creates a fake array with a very large size
		// and then slices it to the actual size we need.
		containers := (*[1 << 30]C.ContainerStateResponse)(
			unsafe.Pointer(cArr),
		)[:count_GOint:count_GOint]

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
		n := int(resp.containers_count)
		containers := (*[1 << 30]C.ContainerStateResponse)(
			unsafe.Pointer(resp.containers),
		)[:n:n]

		for i := 0; i < n; i++ {
			if containers[i].container_id != nil {
				C.free(unsafe.Pointer(containers[i].container_id))
			}
		}
		C.free(unsafe.Pointer(resp.containers))
	}
	C.free(unsafe.Pointer(resp))
}

func main() {}
