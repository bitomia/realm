package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bitomia/realm/agent/api"
	"github.com/bitomia/realm/common"
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

	slog.Info("handlers.RunJobHandler", "job", req.Name, "driver", req.Driver)
	if err := api.RunJob(common.NewJobResultWriter(w), req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}
