package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/daemon/api"
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

func ProvisionNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.ProvisionNodeHandler")

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("handlers.ProvisionNodeHandler", "node", node.Name, "driver", node.Driver)

	if err := api.ProvisionNode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func DeprovisionNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.DeprovisionNodeHandler")

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
		slog.Info("handlers.DeprovisionNodeHandler", "node", *nodeName)
	} else {
		slog.Info("handlers.DeprovisionNodeHandler", "node", "self")
	}

	if err := api.DeprovisionNode(nodeName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func StartNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.StartNodeHandler")

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.StartNode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func StopNodeHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.StopNodeHandler")

	var request dto.StopNodeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.StopNode(request.NodeName, request.WallMessage, request.Time, request.Force); err != nil {
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
