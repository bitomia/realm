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

// GetNode returns the node state (CPU, memory, etc) and status (planned, error, etc..)
func GetNode() (*dto.NodeResponse, error) {
	state, err := cpu.GetNodeState()
	if err != nil {
		return nil, fmt.Errorf("failed to get node state: %w", err)
	}

	database := db.GetDB()
	node, err := database.NodesRepository.GetSelf()
	if err != nil {
		return &dto.NodeResponse{State: state, Status: common.NodeStatus{StatusCode: common.NodeStatusNotDeployed, Reason: ""}}, nil
	}

	status, err := node.NodeDriver.UpdateStatus(database.NodesRepository)
	if err != nil {
		return &dto.NodeResponse{State: state, Status: common.NodeStatus{StatusCode: common.NodeStatusError, Reason: err.Error()}}, nil
	}

	return &dto.NodeResponse{State: state, Status: status}, nil
}

// GetSystemInfo returns static system information about the host
func GetSystemInfo() (*dto.SystemInfo, error) {
	info, err := cpu.GetSystemInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get system info: %w", err)
	}
	return info, nil
}

func PlanNode(node *common.Node) error {
	database := db.GetDB()

	if err := node.Driver.Plan(node.Name, database.NodesRepository); err != nil {
		return err
	}

	return nil
}

func UnplanNode() error {
	database := db.GetDB()

	node, err := database.NodesRepository.GetSelf()
	if err != nil {
		return err
	}

	// Unplan all deployments on this node before unplanning the node
	deployments, err := database.DeploymentsRepository.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get deployments: %w", err)
	}

	for _, deployment := range deployments {
		if err := UnplanLoadDeployments(deployment.LoadName); err != nil {
			return fmt.Errorf("failed to unplan deployment %s: %w", deployment.ID, err)
		}
	}

	// if GetSelf() worked then node is planned
	if err := node.NodeDriver.Unplan(database.NodesRepository); err != nil {
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

	if err := nodeEntry.NodeDriver.Shutdown(message, time, database.NodesRepository); err != nil {
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

	if err := nodeEntry.NodeDriver.Restart(message, time, database.NodesRepository); err != nil {
		return fmt.Errorf("failed to restart self node: %w", err)
	}

	return nil
}
