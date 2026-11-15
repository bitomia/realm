package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
