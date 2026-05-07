package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bitomia/realm/agent/api"
)

// VersionHandler returns the agent version (refactored to use API)
func VersionHandler(w http.ResponseWriter, r *http.Request) {
	version, err := api.GetVersion()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"version": version,
	})
}
