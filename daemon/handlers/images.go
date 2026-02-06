package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/containerd/containerd/images"

	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/daemon/cruntime"
)

func toImagesResponse(images []images.Image) dto.ImagesResponse {
	result := make(dto.ImagesResponse, len(images))
	for i, img := range images {
		result[i] = dto.Image{
			Name:      img.Name,
			CreatedAt: img.CreatedAt,
			UpdatedAt: img.UpdatedAt,
		}
	}
	return result
}

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

	json.NewEncoder(w).Encode(toImagesResponse(images))
}
