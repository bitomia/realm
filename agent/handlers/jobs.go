package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bitomia/realm/agent/api"
	"github.com/bitomia/realm/common/dto"
)

func RunJobHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.RunJobHandler")

	var req dto.JobRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Job == nil {
		http.Error(w, "Job cannot be nil on request", http.StatusBadRequest)
		return
	}

	slog.Info("handlers.RunJobHandler", "job", req.Job.Name, "driver", req.Job.Driver)

	result, err := api.RunJob(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
