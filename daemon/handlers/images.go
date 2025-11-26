package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/containerd/containerd"

	"github.com/bitomia/realm/daemon/cruntime"
)

func ListImagesHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("ListImagesHandler")

	w.Header().Set("Content-Type", "application/json")

	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer client.Close()

	images, err := client.ImageService().List(ctx)
	if err != nil {
		slog.Error("ListImagesHandler", "msg", "failed to list images", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(images)
}

func PullImageHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("PullImageHandler")

	type PullImage struct {
		Image string `json:"image"`
	}
	var pullImage PullImage
	json.NewDecoder(r.Body).Decode(&pullImage)

	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		slog.Error("PullImageHandler", "msg", "cannot create cruntime client", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer client.Close()

	image, err := client.Pull(ctx, pullImage.Image, containerd.WithPullUnpack)
	if err != nil {
		slog.Error("PullImageHandler", "msg", "cannot pull image", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(image)
}
