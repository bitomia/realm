package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const DefaultClusterStateFile = "realm.state"

type ClusterState struct {
	// Nodes sorted by name to keep state deterministic
	Nodes []*Node `json:"nodes"`
}

// WriteFile stores the state on the given path.
// File is written to a temporary file first and rename it (failures never leaves a truncated state)
// Note: Only readable by owner, it may hold credentials (e.g. registries auth).
func (s ClusterState) WriteFile(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cluster state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create cluster state directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create cluster state file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write cluster state file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write cluster state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write cluster state file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to write cluster state file: %w", err)
	}
	return nil
}

func ReadClusterStateFile(path string) (*ClusterState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster state file: %w", err)
	}

	var state ClusterState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse cluster state file %s: %w", path, err)
	}
	return &state, nil
}
