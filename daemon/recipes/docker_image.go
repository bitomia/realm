package recipes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"syscall"

	"github.com/google/uuid"

	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/network"
	"github.com/bitomia/realm/daemon/proxy"
	"github.com/bitomia/realm/internal/requests"
)

type RecipeDockerImageRet struct {
	recipeId uuid.UUID
}

type DockerExposeOpt struct {
	Port  int32    `json:"port"`
	Proto string   `json:"proto"`
	Hosts []string `json:"hosts"`
}

type DockerImageRecipeOpts struct {
	Image  string           `json:"image"`
	Expose *DockerExposeOpt `json:"expose,omitempty"`
}

func LaunchDockerImage(w http.ResponseWriter, recipeId uuid.UUID, recipeOpts DockerImageRecipeOpts, memLimit uint64) (*RecipeDockerImageRet, error) {
	slog.Info("LaunchDockerImage", "recipeId", recipeId.String(), "opts", recipeOpts)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return nil, errors.ErrUnsupported
	}

	// Start docker image
	{
		json.NewEncoder(w).Encode(Progress{0.3, "starting_docker_image"})
		flusher.Flush()

		slog.Info("LaunchDockerImage - Start docker image", "recipeID", recipeId.String())

		containerName := fmt.Sprintf("%s_%s", "di", recipeId)
		createContainerOpts := requests.CreateContainerOpts{
			Image: recipeOpts.Image,
			Quotas: requests.Quotas{
				MemLimit: &memLimit,
			},
		}
		if err := containers.CreateContainer(containerName, createContainerOpts, nil); err != nil {
			return nil, err
		}

		json.NewEncoder(w).Encode(Progress{0.6, "docker_image_downloaded"})
		flusher.Flush()

		updateStateOpts := containers.UpdateContainerOpts{
			State: "start",
		}
		if err := containers.UpdateContainerState(containerName, updateStateOpts); err != nil {
			opts := containers.DeleteContainerOpts{}
			if containers.DeleteContainer(containerName, opts, syscall.SIGKILL, true, true) != nil {
				slog.Info("DeleteContainer for UpdateContainerState failed on LaunchDockerImage", "container", containerName)
			}
			return nil, err
		}

		json.NewEncoder(w).Encode(Progress{0.8, "docker_image_running"})
		flusher.Flush()

		startNetworkOpts := network.StartNetworkOpts{
			Network: recipeId.String(),
			DNS:     true,
		}
		err, _, _, _ := network.StartNetwork(containerName, startNetworkOpts)
		if err != nil {
			opts := containers.DeleteContainerOpts{}
			if containers.DeleteContainer(containerName, opts, syscall.SIGKILL, true, true) != nil {
				slog.Info("DeleteContainer for StartNetwork failed on LaunchDockerImage", "container", containerName)
			}
			return nil, err
		}

		if recipeOpts.Expose != nil && len(recipeOpts.Expose.Hosts) > 0 {
			slog.Info("LaunchDockerImage - Creating reverse proxy", "recipeID", recipeId.String(), "opts", recipeOpts.Expose)

			if recipeOpts.Expose.Proto == "http" || recipeOpts.Expose.Proto == "https" {
				// Create reverse proxy
				proxyOpts := proxy.ProxyOpts{
					Hosts:        recipeOpts.Expose.Hosts,
					Upstream:     fmt.Sprintf(`%s.realm:%d`, containerName, recipeOpts.Expose.Port),
					HttpUpstream: true,
				}

				caddyError := proxy.SetReverseProxy(containerName, proxyOpts)
				if caddyError.Error != nil {
					slog.Error("SetReverseProxy failed", "container", containerName, "opts", proxyOpts, "body", caddyError.Body)
					return nil, caddyError.Error
				}

				slog.Info("LaunchDockerImage - Reverse proxy created", "recipeID", recipeId.String())
			} else {
				slog.Info("LaunchDockerImage - Reverse proxy not created. Proto not supported", "recipeID", recipeId.String())
			}
		}
	}

	json.NewEncoder(w).Encode(Progress{1.0, "done"})
	flusher.Flush()

	return &RecipeDockerImageRet{recipeId}, nil
}

func RollbackDockerImage(recipeId string) error {
	slog.Info("RollbackDockerImage", "recipeID", recipeId)

	containerName := fmt.Sprintf("%s_%s", "di", recipeId)
	containers.UpdateContainerState(containerName, containers.UpdateContainerOpts{State: "stop"})
	err := containers.DeleteContainer(containerName, containers.DeleteContainerOpts{RemoveVolume: false}, syscall.SIGTERM, true, true)
	if err != nil {
		if err.Code == 1 {
			slog.Error("Ignored error while deleting container because it doesn't exist", "container", containerName)
		} else {
			return err
		}
	}
	caddyError := proxy.DeleteReverseProxy(containerName)
	if caddyError.Error != nil {
		slog.Error("Ignored error while deleting reverse proxy", "container", containerName, "error", caddyError)
	}
	return nil
}
