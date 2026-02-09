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
	slog.Info("handlers.PlanLoadHandler")

	var load common.Load
	err := json.NewDecoder(r.Body).Decode(&load)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("handlers.PlanLoadHandler", "load", load.Name, "driver", load.Driver)

	planLoadInfo, err := api.PlanLoad(&load)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(planLoadInfo)
}

func RunLoadDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("handlers.RunLoadDeploymentsHandler", "loadName", loadName)

	if err := api.RunLoadDeployments(loadName); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func StopLoadDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("handlers.StopLoadDeploymentsHandler", "loadName", loadName)

	if err := api.StopLoadDeployments(loadName); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func KillLoadDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("handlers.KillLoadDeploymentsHandler", "loadName", loadName)

	if err := api.KillLoadDeployments(loadName); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func UnplanLoadDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("handlers.UnplanLoadDeploymentsHandler", "loadName", loadName)

	if err := api.UnplanLoadDeployments(loadName); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func GetLoadsDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.GetLoadsDeploymentsHandler")

	response, err := api.GetLoadsDeployments()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func ReadLoadStdoutHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("handlers.ReadStdoutLoadHandler", "loadName", loadName)
	if err := api.StreamLoadStdout(loadName, w); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
}

func ReadLoadStderrHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("handlers.ReadStderrLoadHandler", "loadName", loadName)
	if err := api.StreamLoadStderr(loadName, w); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
}
