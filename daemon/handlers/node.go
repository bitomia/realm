package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bitomia/realm/daemon/api"
)

// GetNodeStatusHandler returns node status (refactored to use API)
func GetNodeStatusHandler(w http.ResponseWriter, r *http.Request) {
	status, err := api.GetNodeStatus()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
