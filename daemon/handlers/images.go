package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/containerd/containerd"

	"github.com/bitomia/realm/daemon/cruntime"
)

func ListImagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer client.Close()

	images, err := client.ImageService().List(ctx)
	if err != nil {
		log.Printf("Failed to list images: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(images)
}

func PullImageHandler(w http.ResponseWriter, r *http.Request) {
	type PullImage struct {
		Image string `json:"image"`
	}
	var pullImage PullImage
	json.NewDecoder(r.Body).Decode(&pullImage)

	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer client.Close()

	image, err := client.Pull(ctx, pullImage.Image, containerd.WithPullUnpack)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(image)
}
