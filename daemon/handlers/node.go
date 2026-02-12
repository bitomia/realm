package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/daemon/api"
)

func GetNodeHandler(w http.ResponseWriter, r *http.Request) {
	state, err := api.GetNode()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func GetSystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	info, err := api.GetSystemInfo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func PlanNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.PlanNodeHandler")

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("handlers.PlanNodeHandler", "node", node.Name, "driver", node.Driver)

	if err := api.PlanNode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func UnplanNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.UnplanNodeHandler")

	if err := api.UnplanNode(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ShutdownNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.ShutdownNodeHandler")

	var request dto.ShutdownNodeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.ShutdownNode(request.WallMessage, request.Time); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func RestartNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.RestartNodeHandler")

	var request dto.RestartNodeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.RestartNode(request.WallMessage, request.Time); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
