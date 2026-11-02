package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/agent/api"
)

func StartLoadDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("handlers.StartLoadDeploymentsHandler", "loadName", loadName)

	if err := api.StartLoadDeployments(loadName); err != nil {
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

func GetLoadsDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.GetLoadsDeploymentsHandler")

	response, err := api.GetLoadsDeployments()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
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
