package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bitomia/realm/daemon/api"
)

func GetNodeStateHandler(w http.ResponseWriter, r *http.Request) {
	state, err := api.GetNodeState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func GetSystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	info, err := api.GetSystemInfo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
