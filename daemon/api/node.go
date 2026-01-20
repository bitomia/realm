package api

import (
	"fmt"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/daemon/cpu"
	"github.com/bitomia/realm/daemon/db"
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

// GetSystemInfo returns static system information about the host
func GetSystemInfo() (*dto.SystemInfo, error) {
	info, err := cpu.GetSystemInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get system info: %w", err)
	}
	return info, nil
}

func PlanAndRegisterNode(node *common.Node) error {
	database := db.GetDB()

	if err := node.Driver.PlanAndRegister(node.Name, database.NodesRepository); err != nil {
		return err
	}

	return nil
}

func ShutdownNode(message string, time uint32) error {
	database := db.GetDB()

	nodeEntry, err := database.NodesRepository.GetSelf()
	if err != nil {
		return fmt.Errorf("failed to get self node: %w", err)
	}

	if err := nodeEntry.NodeDriver.Shutdown(message, time); err != nil {
		return fmt.Errorf("failed to shutdown self node: %w", err)
	}

	return nil
}

func RestartNode(message string, time uint32) error {
	database := db.GetDB()

	nodeEntry, err := database.NodesRepository.GetSelf()
	if err != nil {
		return fmt.Errorf("failed to get self node: %w", err)
	}

	if err := nodeEntry.NodeDriver.Restart(message, time); err != nil {
		return fmt.Errorf("failed to restart self node: %w", err)
	}

	return nil
}
