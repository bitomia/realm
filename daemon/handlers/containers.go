package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bitomia/realm/daemon/api"
)

func ListContainersHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("ListContainersHandler")

	w.Header().Set("Content-Type", "application/json")

	containersState, err := api.ListContainers()
	if err != nil {
		slog.Error("Failed to list containers", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(containersState)
}
