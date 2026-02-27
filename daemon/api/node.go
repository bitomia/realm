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

// GetNode returns the node state (CPU, memory, etc) and status (provisioned, error, etc..)
func GetNode() (*dto.NodeResponse, error) {
	state, err := cpu.GetNodeState()
	if err != nil {
		return nil, fmt.Errorf("failed to get node state: %w", err)
	}

	database := db.GetDB()
	node, err := database.NodesRepository.GetSelf()
	if err != nil {
		return &dto.NodeResponse{State: state, Status: common.NodeStatus{StatusCode: common.NodeStatusOnline, Reason: ""}}, nil
	}

	status, err := node.NodeDriver.UpdateStatus(&node.NodeName, database.NodesRepository)
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

func ProvisionNode(node *common.Node) error {
	database := db.GetDB()

	if err := node.Driver.Provision(node.Name, database.NodesRepository); err != nil {
		return err
	}

	return nil
}

func DeprovisionNode() error {
	database := db.GetDB()

	node, err := database.NodesRepository.GetSelf()
	if err != nil {
		return err
	}

	// Deprovision all deployments on this node before deprovisioning the node
	deployments, err := database.DeploymentsRepository.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get deployments: %w", err)
	}

	for _, deployment := range deployments {
		if err := DeprovisionLoadDeployments(deployment.LoadName); err != nil {
			return fmt.Errorf("failed to deprovision deployment %s: %w", deployment.ID, err)
		}
	}

	// if GetSelf() worked then node is provisioned
	if err := node.NodeDriver.Deprovision(database.NodesRepository); err != nil {
		return err
	}

	return nil
}

func StartupNode(node *common.Node) error {
	driverInfo, err := node.Driver.DriverInfo()
	if err != nil {
		return fmt.Errorf("failed to startup node: %w", err)
	}

	if driverInfo.StartupMode != common.DaemonMode {
		return fmt.Errorf("startup expects daemon mode")
	}

	if err := node.Driver.Startup(&node.Name, db.GetDB().NodesRepository); err != nil {
		return fmt.Errorf("failed to startup node: %w", err)
	}

	return nil
}

// Return self node if nodename is nil, or try to return guest node otherwise
func getNode(nodeName *string) (*common.NodeEntry, error) {
	database := db.GetDB()

	if nodeName == nil {
		node, err := database.NodesRepository.GetSelf()
		if err != nil {
			return nil, err
		} else {
			return &node, err
		}
	} else {
		node, err := database.NodesRepository.GetGuestNode(*nodeName)
		if err != nil {
			return nil, err
		} else {
			return &node, err
		}
	}
}

func ShutdownNode(nodeName *string, message string, time uint32) error {
	node, err := getNode(nodeName)
	if err != nil {
		return fmt.Errorf("Node not provisioned")
	}

	driverInfo, err := node.NodeDriver.DriverInfo()
	if err != nil {
		return fmt.Errorf("cannot retrieve driver info for %s", node.NodeName)
	}

	if driverInfo.ShutdownMode != common.DaemonMode {
		return fmt.Errorf("shutdown expects daemon mode")
	}

	if err := node.NodeDriver.Shutdown(&node.NodeName, message, time, db.GetDB().NodesRepository); err != nil {
		return fmt.Errorf("failed to shutdown node: %w", err)
	}

	return nil
}

func RestartNode(nodeName *string, message string, time uint32) error {
	node, err := getNode(nodeName)
	if err != nil {
		return fmt.Errorf("Node not provisioned")
	}

	driverInfo, err := node.NodeDriver.DriverInfo()
	if err != nil {
		return fmt.Errorf("cannot retrieve driver info for %s", node.NodeName)
	}

	if driverInfo.RestartMode != common.DaemonMode {
		return fmt.Errorf("restart expects daemon mode")
	}

	if err := node.NodeDriver.Restart(&node.NodeName, message, time, db.GetDB().NodesRepository); err != nil {
		return fmt.Errorf("failed to restart node: %w", err)
	}

	return nil
}
