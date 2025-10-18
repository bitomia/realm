package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bitomia/realm/internal/loads"
	"github.com/bitomia/realm/internal/loads/drivers"
)

func VerifyLoadHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("loads.VerifyLoadHandler")

	var load loads.Load
	err := json.NewDecoder(r.Body).Decode(&load)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("loads.VerifyLoadHandler", "load", load.Name, "driver", load.Driver, "w", load.Driver.(*drivers.ProcessDriver).WorkingDir)
	if err := load.Driver.VerifyDaemon(); err != nil {
		http.Error(w, err.Error(), http.StatusNotAcceptable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}
