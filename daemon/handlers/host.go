package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"

	"github.com/bitomia/realm/daemon/cpu"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/internal/config"
)

func GetNodeStatusHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("GetNodeStatusHandler")

	nodeStatus, err := cpu.GetNodeState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(nodeStatus)
}

func VersionHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("VersionHandler")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version": config.GetVersion(),
	})
}

func HealthStatusHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("HealthStatusHandler")

	database := db.GetDB()
	healthStatuses, err := database.GetAllHealthStatuses()
	if err != nil {
		slog.Error("Failed to get health statuses", "error", err.Error())
		http.Error(w, "Failed to retrieve health statuses", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"health_statuses": healthStatuses,
		"count":           len(healthStatuses),
	})
}

func ShutdownHostHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("ShutdownHostHandler")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Shutdown initiated",
	})

	go func() {
		cmd := exec.Command("shutdown", "-h", "now")
		if err := cmd.Run(); err != nil {
			slog.Error("Failed to shutdown host", "error", err.Error())
		}
	}()
}
