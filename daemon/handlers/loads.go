package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/daemon/api"
)

func PlanLoadHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("loads.PlanLoadHandler")

	var load common.Load
	err := json.NewDecoder(r.Body).Decode(&load)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("loads.PlanLoadHandler", "load", load.Name, "driver", load.Driver)

	planLoadInfo, err := api.PlanLoad(&load)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(planLoadInfo)
}

func StartLoadDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("loads.StartLoadHandler", "loadName", loadName)

	if err := api.StartLoadDeployments(loadName); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func StopLoadDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("loads.StopLoadDeploymentsHandler", "loadName", loadName)

	if err := api.StopLoadDeployments(loadName); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func UnplanLoadHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("loads.UnplanLoadHandler", "loadName", loadName)

	if err := api.UnplanLoad(loadName); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func GetLoadsDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("loads.GetLoadStatesHandler")

	response, err := api.GetLoadsDeployments()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
