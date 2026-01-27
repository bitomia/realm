package containers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"

	"github.com/bitomia/realm/daemon/cruntime"
)

func createTask(ctx context.Context, container containerd.Container, containerName string, stdoutPath string, stderrPath string) (containerd.Task, error) {
	if err := os.MkdirAll(stdoutPath, 0755); err != nil {
		slog.Error("Failed to create containers log directory", "path", stdoutPath, "error", err.Error())
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	if err := os.MkdirAll(stderrPath, 0755); err != nil {
		slog.Error("Failed to create containers log directory", "path", stderrPath, "error", err.Error())
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout log file: %w", err)
	}

	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		stdoutFile.Close()
		return nil, fmt.Errorf("failed to create stderr log file: %w", err)
	}

	task, err := container.NewTask(ctx, cio.NewCreator(
		cio.WithStreams(nil, stdoutFile, stderrFile),
	))
	if err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		slog.Error("Failed to create new task for container on restart", "container", containerName, "error", err.Error())
		return nil, err
	}

	slog.Info("Task created for container", "taskPID", task.Pid(), "container", containerName, "stdout", stdoutPath, "stderr", stderrPath)

	return task, nil
}

func startContainer(containerName string, stdoutPath string, stderrPath string) (containerd.Task, error) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	container, err := client.LoadContainer(ctx, containerName)
	if err != nil {
		slog.Error("Failed to retrieve container on start", "container", containerName, "error", err.Error())
		return nil, err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		slog.Info("Task doesn't exist for container. Creating task again", "container", containerName)
		task, err = createTask(ctx, container, containerName, stdoutPath, stderrPath)
	}
	if err != nil {
		slog.Error("Impossible to retrieve task for container", "container", containerName)
		return nil, err
	}
	if err := task.Start(ctx); err != nil {
		slog.Error("Failed to start task for container on start", "container", containerName, "error", err.Error())
		return nil, err
	}

	return task, nil
}

func tryDeleteContainerTask(ctx context.Context, container containerd.Container, signal syscall.Signal) error {
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

		io := task.IO()
		_, err = task.Delete(ctx)
		if io != nil {
			io.Close()
		}

		return err
	}
	return nil
}

func stopContainer(containerName string, signal syscall.Signal) error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return err
	}
	defer client.Close()

	container, err := client.LoadContainer(ctx, containerName)
	if err != nil {
		slog.Error("Failed to retrieve container on stop", "container", containerName, "error", err.Error())
		return err
	}

	err = tryDeleteContainerTask(ctx, container, signal)
	if err != nil {
		slog.Error("Failed to delete task for container on stop", "container", containerName, "error", err.Error())
		return err
	}
	return nil
}
