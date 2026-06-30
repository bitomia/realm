package handlers

import (
	"encoding/json"
	"errors"
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

func GetNodeConfigHandler(w http.ResponseWriter, r *http.Request) {
	slog.Debug("handlers.GetNodeConfigHandler")

	if nodeConfig, err := api.GetNodeConfig(); err != nil {
		if errors.Is(err, common.ErrNodeNotConfigured) {
			w.WriteHeader(http.StatusNoContent)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	} else {
		if nodeConfig != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(*nodeConfig)
		} else {
			http.NotFound(w, r)
		}
	}
}

func LoadNodeConfigHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.LoadNodeConfigHandler")

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("handlers.LoadNodeConfigHandler", "node", node.Name, "driver", node.Driver)

	if err := api.LoadNodeConfig(&node); err != nil {
		if errors.Is(err, common.ErrNodeAlreadyConfigured) {
			w.WriteHeader(http.StatusConflict)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func UnloadNodeConfigHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.UnloadNodeConfigHandler")

	if guestNodeName := r.URL.Query().Get("guest"); guestNodeName != "" {
		slog.Info("handlers.UnloadNodeConfigHandler", "guest", guestNodeName)
		if err := api.UnloadGuestNodeConfig(guestNodeName); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		slog.Info("handlers.UnloadNodeConfigHandler", "self", true)
		if err := api.UnloadNodeConfig(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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
