package cruntime

import (
	"context"
	"fmt"
	"log"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"

	"github.com/bitomia/realm/internal/config"
)

func CreateClient() (ctx context.Context, c *containerd.Client, err error) {
	containerdSock := config.Get().Daemon.ContainerdSock
	client, error := containerd.New(containerdSock)
	if error != nil {
		log.Printf("Failed to create containerd client: %v", error)
		return nil, nil, error
	}

	namespace := config.Get().Daemon.ContainerdNamespace
	ctx = namespaces.WithNamespace(context.Background(), namespace)

	return ctx, client, nil
}

func GetContainerTaskPID(ctx context.Context, c *containerd.Client, containerName string) (uint32, error) {
	containers, err := c.Containers(ctx)
	if err != nil {
		return 0, err
	}
	for _, container := range containers {
		if container.ID() != containerName {
			continue
		}

		task, err := container.Task(ctx, nil)
		if err != nil {
			log.Printf("Error while retrieving task for container %s: %s", containerName, err.Error())
			return 0, err
		}
		if task == nil {
			continue
		}

		return task.Pid(), nil
	}
	return 0, fmt.Errorf("Container task not found")
}
