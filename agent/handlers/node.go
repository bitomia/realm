package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bitomia/realm/agent/api"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

func GetNodeState(w http.ResponseWriter, r *http.Request) {
	state, err := api.GetNode()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	info, err := api.GetSystemInfo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func GetNodeConfig(w http.ResponseWriter, r *http.Request) {
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
			_ = json.NewEncoder(w).Encode(nodeConfig)
		} else {
			http.NotFound(w, r)
		}
	}
}

func LoadNodeConfig(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.LoadNodeConfig")

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("handlers.LoadNodeConfig", "node", node.Name, "driver", node.Driver)

	onlyValidate := false
	if v := r.URL.Query().Get("validate"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			http.Error(w, "invalid value for validate", http.StatusBadRequest)
			return
		}
		onlyValidate = parsed
	}

	if onlyValidate {
		err = api.CheckNodeConfig(&node)
	} else {
		err = api.LoadNodeConfig(&node)
	}

	switch {
	case err == nil:
		w.WriteHeader(http.StatusCreated)
	case errors.Is(err, common.ErrNodeAlreadyConfigured):
		http.Error(w, err.Error(), http.StatusOK)
	case errors.Is(err, common.ErrNodeLoadsMismatch):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, common.ErrNodeLiveConfigMismatch):
		http.Error(w, err.Error(), http.StatusPreconditionFailed)
	case errors.Is(err, common.ErrNodeConfigMismatch):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func UnloadNodeConfig(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.UnloadNodeConfig")

	if err := api.UnloadNodeConfig(); err != nil {
		if errors.Is(err, common.ErrNodeNotConfigured) {
			w.WriteHeader(http.StatusNoContent)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func PowerOnNode(w http.ResponseWriter, r *http.Request) {
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

func ShutdownNode(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.ShutdownNodeHandler")

	var request dto.ShutdownNodeRequest
	if err := decodeOptionalBody(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.ShutdownNode(request.WallMessage, request.Time); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func PowerOffNode(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.PowerOffNodeHandler")

	if err := api.PowerOffNode(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func RestartNode(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.RestartNodeHandler")

	var request dto.RestartNodeRequest
	if err := decodeOptionalBody(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.RestartNode(request.WallMessage, request.Time); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
