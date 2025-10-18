package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bitomia/realm/internal/loads"
)

func VerifyLoadHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("loads.VerifyLoadHandler")

	var load loads.Load
	err := json.NewDecoder(r.Body).Decode(&load)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("loads.VerifyLoadHandler", "load", load.Name, "driver", load.Driver)
	if err := load.Driver.VerifyDaemon(); err != nil {
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

	slog.Info("loads.StartLoadHandler", "load", load.Name, "driver", load.Driver)
	if err := load.Driver.StartOnDaemon(); err != nil {
		http.Error(w, err.Error(), http.StatusNotAcceptable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}
