package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/bitomia/realm/agent/api"
	"github.com/bitomia/realm/common"
)

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

// writeLoadNodeConfigResult loads the node config or checks it when validating.
func writeLoadNodeConfigResult(w http.ResponseWriter, node *common.Node, onlyValidate bool) {
	var err error
	if onlyValidate {
		err = api.CheckNodeConfig(node)
	} else {
		err = api.LoadNodeConfig(node)
	}

	switch {
	case err == nil:
		w.WriteHeader(http.StatusCreated)
	case errors.Is(err, common.ErrNodeAlreadyConfigured):
		http.Error(w, err.Error(), http.StatusOK)
	case errors.Is(err, common.ErrNodeConfigMismatch):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
