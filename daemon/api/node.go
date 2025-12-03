package api

import (
	"fmt"

	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/daemon/cpu"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/internal/dto"
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

// GetNodeState returns the current node status (CPU, memory, etc) as a typed dto.NodeStateResponse
func GetNodeState() (*dto.NodeStateResponse, error) {
	state, err := cpu.GetNodeState()
	if err != nil {
		return nil, fmt.Errorf("failed to get node state: %w", err)
	}
	return state, nil
}
