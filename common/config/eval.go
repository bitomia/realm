package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EvalVars replaces every occurrence of {{var}} with the corresponding value
// from vars, mutating object in place. object must be a pointer.
func EvalVars[T any](object *T, vars map[string]string) error {
	data, err := json.Marshal(object)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	result := string(data)
	for key, value := range vars {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}

	if err := json.Unmarshal([]byte(result), object); err != nil {
		return fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return nil
}
