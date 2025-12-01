package main

/*
#include <stdlib.h>

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

typedef struct {
    NodeStateResponse* nodes;
    int nodes_count;
} NodesStateResponse;
*/
import "C"

import (
	"unsafe"

	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/internal/dto"
)

//export GetNodesState
func GetNodesState() *C.NodesStateResponse {
	client := clientPkg.NewClient()
	nodes := clientPkg.GetNodes()

	// First collect only successful node states
	// Reason we sanitize here is to allocate the exact amount of C memory needed
	// Because it looks like clientPkg.GetNodes() return all nodes from the config
	// even if they are unhealthy or offline.
	// TODO? log unreachable nodes?
	statuses := make([]*dto.NodeStateResponse, 0, len(nodes))
	for _, node := range nodes {
		status, err := client.GetNodeState(node.Url)
		if err != nil || status == nil {
			continue
		}
		statuses = append(statuses, status)
	}

	if len(statuses) == 0 {
		return nil
	}

	// Allocate response in C heap
	resp := (*C.NodesStateResponse)(C.malloc(C.size_t(C.sizeof_NodesStateResponse)))
	if resp == nil {
		return nil
	}

	count := len(statuses)
	resp.nodes_count = C.int(count)

	// Allocate C array of NodeStateResponse
	bytes := C.size_t(C.sizeof_NodeStateResponse) * C.size_t(count)
	resp.nodes = (*C.NodeStateResponse)(C.malloc(bytes))
	if resp.nodes == nil {
		C.free(unsafe.Pointer(resp))
		return nil
	}

	// Create a Go slice view over the C-allocated array using
	// the "big array trick". Creates a fake array with a very large size
	// and then slices it to the actual size we need.
	nodesSlice := (*[1 << 30]C.NodeStateResponse)(
		unsafe.Pointer(resp.nodes),
	)[:count:count]

	for i, status := range statuses {
		var cContainers *C.ContainerStateResponse
		var containersCount C.int

		if len(status.Containers) > 0 {
			containersCount = C.int(len(status.Containers))
			// convert to Go int, not same as C.int on all platforms
			// ie: 32-bit vs 64-bit, realistically should never overflow in this case
			count_GOint := int(containersCount)

			cBytes := C.size_t(C.sizeof_ContainerStateResponse) * C.size_t(containersCount)
			cArr := (*C.ContainerStateResponse)(C.malloc(cBytes))
			if cArr != nil {
				// Same big fake array trick for containers
				containersSlice := (*[1 << 30]C.ContainerStateResponse)(
					unsafe.Pointer(cArr),
				)[:count_GOint:count_GOint]

				for j, c := range status.Containers {
					containersSlice[j].container_id = C.CString(c.ContainerID)
					containersSlice[j].cpu_usage = C.double(c.CPUUsage)
					containersSlice[j].cpu_system = C.double(c.CPUSystem)
					containersSlice[j].cpu_user = C.double(c.CPUUser)
					containersSlice[j].memory_usage = C.double(c.MemoryUsage)
					containersSlice[j].memory_limit = C.double(c.MemoryLimit)
					containersSlice[j].memory_percent = C.double(c.MemoryPercent)
				}

				cContainers = cArr
			} else {
				containersCount = 0
			}
		}

		nodesSlice[i] = C.NodeStateResponse{
			num_cpu:           C.int(status.NumCPU),
			user_cpu:          C.ulonglong(status.UserCPU),
			idle_cpu:          C.ulonglong(status.IdleCPU),
			system_cpu:        C.ulonglong(status.SystemCPU),
			total_cpu:         C.ulonglong(status.TotalCPU),
			usage_cpu_percent: C.double(status.UsageCPUPercent),
			total_mem:         C.ulonglong(status.TotalMem),
			used_mem:          C.ulonglong(status.UsedMem),
			free_mem:          C.ulonglong(status.FreeMem),
			free_mem_percent:  C.double(status.FreeMemPercent),
			free_storage:      C.ulonglong(status.FreeStorage),
			containers:        cContainers,
			containers_count:  containersCount,
		}
	}

	return resp
}

//export FreeNodesStateResponse
func FreeNodesStateResponse(response *C.NodesStateResponse) {
	if response == nil {
		return
	}

	// Free each node's containers + strings
	if response.nodes != nil && response.nodes_count > 0 {
		nodes_count := int(response.nodes_count)
		nodesSlice := (*[1 << 30]C.NodeStateResponse)(
			unsafe.Pointer(response.nodes),
		)[:nodes_count:nodes_count]

		for i := 0; i < nodes_count; i++ {
			node := &nodesSlice[i]

			if node.containers != nil && node.containers_count > 0 {
				containers_count := int(node.containers_count)
				containersSlice := (*[1 << 30]C.ContainerStateResponse)(
					unsafe.Pointer(node.containers),
				)[:containers_count:containers_count]

				for j := 0; j < containers_count; j++ {
					if containersSlice[j].container_id != nil {
						C.free(unsafe.Pointer(containersSlice[j].container_id))
					}
				}

				C.free(unsafe.Pointer(node.containers))
			}
		}

		C.free(unsafe.Pointer(response.nodes))
	}

	C.free(unsafe.Pointer(response))
}
