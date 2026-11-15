package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// decodeOptionalBody decodes a JSON body into v, treating an empty body as valid.
func decodeOptionalBody(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
