package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/internal/config"
	"github.com/bitomia/realm/internal/loads"
	"github.com/gorilla/mux"
)

func PlanLoadHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("loads.PlanLoadHandler")

	var load loads.Load
	err := json.NewDecoder(r.Body).Decode(&load)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	database := db.GetDB()
	if _, err := database.LoadsRepository.GetLoad(load.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("loads.PlanLoadHandler", "load", load.Name, "driver", load.Driver)
	if err := load.Driver.PlanDaemon(); err != nil {
		http.Error(w, err.Error(), http.StatusNotAcceptable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func StartLoadHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("loads.StartLoadHandler")

	var load loads.Load
	err := json.NewDecoder(r.Body).Decode(&load)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	database := db.GetDB()
	activeLoad, err := database.LoadsRepository.GetLoad(load.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if activeLoad != nil {
		http.Error(w, "This load is already active", http.StatusBadRequest)
		return
	}

	config := config.Get()
	if config == nil {
		http.Error(w, "Cannot open configuration", http.StatusBadGateway)
		return
	}

	slog.Info("loads.StartLoadHandler", "load", load.Name, "driver", load.Driver)
	if err := load.Driver.StartOnDaemon(db.GetDB().LoadsRepository, config.Daemon.LogsPath, load.Name); err != nil {
		http.Error(w, err.Error(), http.StatusNotAcceptable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func StopLoadHandler(w http.ResponseWriter, r *http.Request) {
	loadKey := mux.Vars(r)["loadKey"]
	slog.Info("loads.StopLoadHandler", "loadKey", loadKey)

	database := db.GetDB()
	activeLoad, err := database.LoadsRepository.GetLoad(loadKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if activeLoad == nil {
		http.Error(w, "There is no load running", http.StatusBadRequest)
		return
	}

	slog.Info("loads.StopLoadHandler", "loadKey", loadKey, "driver", activeLoad.Driver)
	if err := activeLoad.Driver.StopOnDaemon(db.GetDB().LoadsRepository, activeLoad.Name); err != nil {
		http.Error(w, err.Error(), http.StatusNotAcceptable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}
