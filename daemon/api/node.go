package api

import (
	"fmt"

	"github.com/bitomia/realm/daemon/cpu"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/internal/config"
)

// GetVersion returns the daemon version
func GetVersion() (string, error) {
	version := config.GetVersion()
	return version, nil
}

// GetHealthStatus returns the health status of all monitored services
func GetHealthStatus() (map[string]any, error) {
	database := db.GetDB()
	healthStatuses, err := database.GetAllHealthStatuses()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve health statuses: %w", err)
	}

	result := map[string]any{
		"health_statuses": healthStatuses,
		"count":           len(healthStatuses),
	}
	return result, nil
}

// TODO fix this to return a dto(e.g. dto.NodeStateResponse)
// GetNodeState returns the current node status (CPU, memory, etc.) and health status
func GetNodeState() (any, error) {
	nodeState, err := cpu.GetNodeState()
	if err != nil {
		return nil, fmt.Errorf("failed to get node status: %w", err)
	}

	database := db.GetDB()
	healthStatuses, err := database.GetAllHealthStatuses()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve health statuses: %w", err)
	}

	result := map[string]any{
		"node_state":      nodeState,
		"health_statuses": healthStatuses,
		"health_count":    len(healthStatuses),
	}

	return result, nil
}
