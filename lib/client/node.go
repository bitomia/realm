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
)

//export GetNodesState
func GetNodesState() *C.NodesStateResponse {
	client := clientPkg.NewClient()
	nodes := clientPkg.GetNodes()

	// Collect all node states
	nodeStates := make([]C.NodeStateResponse, 0, len(nodes))

	for _, node := range nodes {
		status, err := client.GetNodeState(node.Url)
		if err != nil {
			continue
		}

		// Convert containers to C array
		var cContainers *C.ContainerStateResponse
		if len(status.Containers) > 0 {
			containers := make([]C.ContainerStateResponse, len(status.Containers))
			for i, container := range status.Containers {
				containers[i] = C.ContainerStateResponse{
					container_id:   C.CString(container.ContainerID),
					cpu_usage:      C.double(container.CPUUsage),
					cpu_system:     C.double(container.CPUSystem),
					cpu_user:       C.double(container.CPUUser),
					memory_usage:   C.double(container.MemoryUsage),
					memory_limit:   C.double(container.MemoryLimit),
					memory_percent: C.double(container.MemoryPercent),
				}
			}
			cContainers = &containers[0]
		}

		// Create the node response
		nodeState := C.NodeStateResponse{
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
			containers_count:  C.int(len(status.Containers)),
		}

		nodeStates = append(nodeStates, nodeState)
	}

	if len(nodeStates) == 0 {
		return nil
	}

	// Create the response with all nodes
	response := &C.NodesStateResponse{
		nodes:       &nodeStates[0],
		nodes_count: C.int(len(nodeStates)),
	}

	return response
}

//export FreeNodesStateResponse
func FreeNodesStateResponse(response *C.NodesStateResponse) {
	if response == nil {
		return
	}

	// Free each node's data
	if response.nodes != nil {
		nodes := (*[1 << 30]C.NodeStateResponse)(unsafe.Pointer(response.nodes))[:response.nodes_count:response.nodes_count]

		for i := 0; i < int(response.nodes_count); i++ {
			node := &nodes[i]

			// Free containers for this node
			if node.containers != nil {
				containers := (*[1 << 30]C.ContainerStateResponse)(unsafe.Pointer(node.containers))[:node.containers_count:node.containers_count]

				// Free each container's strings
				for j := 0; j < int(node.containers_count); j++ {
					if containers[j].container_id != nil {
						C.free(unsafe.Pointer(containers[j].container_id))
					}
				}
			}
		}
	}
}
