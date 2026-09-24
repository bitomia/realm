package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/agent/api"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

func GetNodeState(w http.ResponseWriter, r *http.Request) {
	state, err := api.GetNode(nil)
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
			_ = json.NewEncoder(w).Encode(*nodeConfig)
		} else {
			http.NotFound(w, r)
		}
	}
}

func LoadNodeConfig(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.LoadNodeConfigHandler")

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("handlers.LoadNodeConfigHandler", "node", node.Name, "driver", node.Driver)

	onlyValidate := false
	if v := r.URL.Query().Get("validate"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			http.Error(w, "invalid value for validate", http.StatusBadRequest)
			return
		}
		onlyValidate = parsed
	}

	if !onlyValidate {
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
}

func UnloadNodeConfig(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.UnloadNodeConfig")

	if err := api.UnloadNodeConfig(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	if err := api.ShutdownNode(nil, request.WallMessage, request.Time); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func PowerOffNode(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlers.PowerOffNodeHandler")

	if err := api.PowerOffNode(nil); err != nil {
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

	if err := api.RestartNode(nil, request.WallMessage, request.Time); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func GetGuestNodes(w http.ResponseWriter, r *http.Request) {
	guests, err := api.GetGuestNodes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(guests)
}

func GetGuestNodeState(w http.ResponseWriter, r *http.Request) {
	guestName := mux.Vars(r)["guestName"]

	state, err := api.GetNode(&guestName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func GetGuestNodeConfig(w http.ResponseWriter, r *http.Request) {
	guestName := mux.Vars(r)["guestName"]
	slog.Debug("handlers.GetGuestNodeConfig", "guest", guestName)

	if nodeConfig, err := api.GetGuestNodeConfig(guestName); err != nil {
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

func LoadGuestNodeConfig(w http.ResponseWriter, r *http.Request) {
	guestName := mux.Vars(r)["guestName"]
	slog.Info("handlers.LoadGuestNodeConfig", "guest", guestName)

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !checkGuestName(w, guestName, node.Name) {
		return
	}
	node.Name = guestName

	slog.Info("handlers.LoadGuestNodeConfig", "node", node.Name, "driver", node.Driver)

	info, err := node.Driver.Info()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !info.GuestMode {
		http.Error(w, "node driver is not in guest mode", http.StatusBadRequest)
		return
	}

	onlyValidate := false
	if v := r.URL.Query().Get("validate"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			http.Error(w, "invalid value for validate", http.StatusBadRequest)
			return
		}
		onlyValidate = parsed
	}

	if !onlyValidate {
		if err := api.LoadNodeConfig(&node); err != nil {
			if errors.Is(err, common.ErrNodeAlreadyConfigured) {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func UnloadGuestNodeConfig(w http.ResponseWriter, r *http.Request) {
	guestName := mux.Vars(r)["guestName"]
	slog.Info("handlers.UnloadGuestNodeConfig", "guest", guestName)

	if err := api.UnloadGuestNodeConfig(guestName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func PowerOnGuestNode(w http.ResponseWriter, r *http.Request) {
	guestName := mux.Vars(r)["guestName"]
	slog.Info("handlers.PowerOnGuestNode", "guest", guestName)

	var node common.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !checkGuestName(w, guestName, node.Name) {
		return
	}
	node.Name = guestName

	if err := api.PowerOnNode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func PowerOffGuestNode(w http.ResponseWriter, r *http.Request) {
	guestName := mux.Vars(r)["guestName"]
	slog.Info("handlers.PowerOffGuestNode", "guest", guestName)

	if err := api.PowerOffNode(&guestName); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ShutdownGuestNode(w http.ResponseWriter, r *http.Request) {
	guestName := mux.Vars(r)["guestName"]
	slog.Info("handlers.ShutdownGuestNode", "guest", guestName)

	var request dto.ShutdownNodeRequest
	if err := decodeOptionalBody(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.ShutdownNode(&guestName, request.WallMessage, request.Time); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func RestartGuestNode(w http.ResponseWriter, r *http.Request) {
	guestName := mux.Vars(r)["guestName"]
	slog.Info("handlers.RestartGuestNode", "guest", guestName)

	var request dto.RestartNodeRequest
	if err := decodeOptionalBody(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.RestartNode(&guestName, request.WallMessage, request.Time); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// checkGuestName rejects requests whose body names a different node than the URL path.
func checkGuestName(w http.ResponseWriter, pathName, bodyName string) bool {
	if bodyName != "" && bodyName != pathName {
		http.Error(w, "node name in body does not match guest name in path", http.StatusBadRequest)
		return false
	}
	return true
}

// decodeOptionalBody decodes a JSON body into v, treating an empty body as valid.
func decodeOptionalBody(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
