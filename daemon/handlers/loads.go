package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/daemon/db"
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
	database := db.GetDB()

	deploymentID, err := load.Driver.PlanAndRegister(database.DeploymentsRepository, load.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotAcceptable)
		return
	}

	// Return the deployment ID to the client
	response := map[string]string{"deployment_id": deploymentID.String()}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func StartLoadHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("loads.StartLoadHandler")

	var load common.Load
	err := json.NewDecoder(r.Body).Decode(&load)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	config := config.Get()
	if config == nil {
		http.Error(w, "Cannot open configuration", http.StatusBadGateway)
		return
	}

	slog.Info("loads.StartLoadHandler", "load", load.Name, "driver", load.Driver)
	database := db.GetDB()

	// Get planned deployments for this load
	deployments, err := database.DeploymentsRepository.GetByLoadAndState(load.Name, common.DeploymentStatePlanned)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if len(deployments) == 0 {
		http.Error(w, "No planned deployment found. Run 'plan' first.", http.StatusBadRequest)
		return
	}

	// Start all planned deployments
	for _, deployment := range deployments {
		slog.Info("loads.StartLoadHandler", "load", load.Name, "deployment", deployment.ID, "msg", "starting deployment")
		if err := deployment.LoadDriver.StartDeployment(database.DeploymentsRepository, deployment); err != nil {
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			return
		}
		slog.Info("StartLoadHandler", "msg", "load deployed", "deploymentID", deployment.ID)
	}

	w.WriteHeader(http.StatusOK)
}

func StopLoadDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("loads.StopLoadDeploymentsHandler", "loadName", loadName)

	config := config.Get()
	if config == nil {
		http.Error(w, "Cannot open configuration", http.StatusBadGateway)
		return
	}

	database := db.GetDB()

	// Only get RUNNING deployments (not planned ones)
	deployments, err := database.DeploymentsRepository.GetByLoadAndState(loadName, common.DeploymentStateRunning)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(deployments) == 0 {
		http.Error(w, "No running deployments found", http.StatusBadRequest)
		return
	}

	for _, deployment := range deployments {
		slog.Info("loads.StopLoadDeploymentsHandler", "loadName", loadName, "driverID", deployment.LoadDriver.GetLoadDriverID())

		if err := deployment.LoadDriver.StopDeployment(database.DeploymentsRepository, deployment); err != nil {
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func UnplanLoadHandler(w http.ResponseWriter, r *http.Request) {
	loadName := mux.Vars(r)["loadName"]
	slog.Info("loads.UnplanLoadHandler", "loadName", loadName)

	database := db.GetDB()

	// Only get PLANNED deployments
	deployments, err := database.DeploymentsRepository.GetByLoadAndState(loadName, common.DeploymentStatePlanned)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if len(deployments) == 0 {
		http.Error(w, "No planned deployments found", http.StatusBadRequest)
		return
	}

	for _, deployment := range deployments {
		slog.Info("loads.UnplanLoadHandler", "loadName", loadName, "driverID", deployment.LoadDriver.GetLoadDriverID())

		if err := deployment.LoadDriver.UnplanDeployment(database.DeploymentsRepository, deployment); err != nil {
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
