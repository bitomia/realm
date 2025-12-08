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

	if err := load.Driver.PlanDaemon(database.DeploymentsRepository, load.Name); err != nil {
		http.Error(w, err.Error(), http.StatusNotAcceptable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
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

	slog.Info("loads.StartLoadHandler", "load", load.Name, "driver", load.Driver, "msg", "start planning")
	if err := load.Driver.PlanDaemon(database.DeploymentsRepository, load.Name); err != nil {
		slog.Info("loads.StartLoadHandler", "load", load.Name, "driver", load.Driver, "msg", "planning error", "error", err)
		http.Error(w, err.Error(), http.StatusNotAcceptable)
		return
	}

	slog.Info("loads.StartLoadHandler", "load", load.Name, "driver", load.Driver, "msg", "start on daemon")
	if deploymentID, err := load.Driver.StartOnDaemon(db.GetDB().DeploymentsRepository, config.Daemon.LogsPath, load.Name); err != nil {
		http.Error(w, err.Error(), http.StatusNotAcceptable)
	} else {
		slog.Info("StartLoadHandler", "msg", "load deployed", "deploymentID", deploymentID)
		w.WriteHeader(http.StatusOK)
	}
}

func StopLoadHandler(w http.ResponseWriter, r *http.Request) {
	loadKey := mux.Vars(r)["loadKey"]
	slog.Info("loads.StopLoadHandler", "loadKey", loadKey)

	database := db.GetDB()
	deployments, err := database.DeploymentsRepository.GetByLoad(loadKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(deployments) == 0 {
		http.Error(w, "There is no load running", http.StatusBadRequest)
		return
	}

	for _, deployment := range deployments {
		slog.Info("loads.StopLoadHandler", "loadKey", loadKey, "driverID", deployment.LoadDriver.GetLoadDriverID())

		if err := deployment.LoadDriver.StopOnDaemon(db.GetDB().DeploymentsRepository, deployment); err != nil {
			http.Error(w, err.Error(), http.StatusNotAcceptable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}

}
