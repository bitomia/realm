package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bitomia/realm/agent/api"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

func NodeStateHandler(w http.ResponseWriter, r *http.Request) {
	var nodeName *string
	if guest := r.URL.Query().Get("guest"); guest != "" {
		nodeName = &guest
	}

	state, err := api.GetNode(nodeName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func GetSystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	info, err := api.GetSystemInfo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func RegisterNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.RegisterNodeHandler")

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("handlers.RegisterNodeHandler", "node", node.Name, "driver", node.Driver)

	if err := api.RegisterNode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func UnregisterNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.UnregisterNodeHandler")

	var nodeName *string
	if guest := r.URL.Query().Get("guest"); guest != "" {
		nodeName = &guest
	} else if r.Body != nil && r.ContentLength != 0 {
		var node common.Node
		if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if node.Name != "" {
			nodeName = &node.Name
		}
	}

	if nodeName != nil {
		slog.Info("handlers.UnregisterNodeHandler", "node", *nodeName)
	} else {
		slog.Info("handlers.UnregisterNodeHandler", "node", "self")
	}

	if err := api.UnregisterNode(nodeName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func PowerOnNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.PowerOnNodeHandler")

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.PowerOnNode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ShutdownNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.ShutdownNodeHandler")

	var request dto.ShutdownNodeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.ShutdownNode(request.NodeName, request.WallMessage, request.Time); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func PowerOffNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.PowerOffNodeHandler")

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.PowerOffNode(&node.Name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func RestartNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.RestartNodeHandler")

	var request dto.RestartNodeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.RestartNode(request.NodeName, request.WallMessage, request.Time); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
