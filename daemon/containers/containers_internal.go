package containers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"

	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/daemon/cruntime"
)

func createTask(ctx context.Context, container containerd.Container, containerName string) (containerd.Task, error) {
	// Get the containers log path from config
	containersLogPath := config.Get().Daemon.ContainersLogPath

	if err := os.MkdirAll(containersLogPath, 0755); err != nil {
		slog.Error("Failed to create containers log directory", "path", containersLogPath, "error", err.Error())
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(containersLogPath, fmt.Sprintf("%s.log", containerName))
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout log file: %w", err)
	}
	defer logFile.Close()

	errorLogPath := filepath.Join(containersLogPath, fmt.Sprintf("%s_error.log", containerName))
	errorLogFile, err := os.Create(errorLogPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr log file: %w", err)
	}
	defer errorLogFile.Close()

	task, err := container.NewTask(ctx, cio.NewCreator(
		cio.WithStreams(nil, logFile, errorLogFile),
	))
	if err != nil {
		slog.Error("Failed to create new task for container on restart", "container", containerName, "error", err.Error())
		return nil, err
	}

	slog.Info("Task create for container", "taskPID", task.Pid(), "container", containerName, "logPath", logPath, "errorLogPath", errorLogPath)
	return task, err
}

func startContainer(containerName string) error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return err
	}
	defer client.Close()

	container, err := client.LoadContainer(ctx, containerName)
	if err != nil {
		slog.Error("Failed to retrieve container on start", "container", containerName, "error", err.Error())
		return err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		slog.Info("Task doesn't exist for container. Creating task again", "container", containerName)
		task, err = createTask(ctx, container, containerName)
	}
	if err != nil {
		slog.Error("Impossible to retrieve task for container", "container", containerName)
		return err
	}
	if err := task.Start(ctx); err != nil {
		slog.Error("Failed to start task for container on start", "container", containerName, "error", err.Error())
		return err
	}

	return nil
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
		_, err = task.Delete(ctx)
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
