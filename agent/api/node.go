package api

import (
	"fmt"

	"github.com/bitomia/realm/agent/capabilities"
	"github.com/bitomia/realm/agent/cloudinit"
	"github.com/bitomia/realm/agent/cpu"
	"github.com/bitomia/realm/agent/db"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
)

// GetVersion returns the agent version
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
// If a node is provided, it will be used to query the node state. Otherwise, the self node is used.
func GetNode(nodeName *string) (*dto.NodeResponse, error) {
	database := db.GetDB()

	var nodeEntry common.NodeEntry
	if nodeName == nil {
		var err error
		nodeEntry, err = database.NodesRepository.GetSelf()
		if err != nil {
			return nil, fmt.Errorf("failed to get self node: %w", err)
		}
	} else {
		var err error
		nodeEntry, err = database.NodesRepository.GetGuestNode(*nodeName)
		if err != nil {
			return nil, fmt.Errorf("failed to get guest node: %w", err)
		}

	}

	state, err := nodeEntry.NodeDriver.GetState(&nodeEntry.NodeName, database.NodesRepository)
	if err != nil {
		return nil, fmt.Errorf("failed to get node state: %w", err)
	}

	status, err := nodeEntry.NodeDriver.UpdateStatus(&nodeEntry.NodeName, database.NodesRepository)
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

	caps := capabilities.Get()
	if caps == nil {
		return nil, fmt.Errorf("nil system capabilities")
	}
	info.Capabilities = dto.NewCapabilities(caps)

	return info, nil
}

func ProvisionNode(node *common.Node) error {
	database := db.GetDB()

	if err := node.Driver.Provision(node.Name, node.CloudInit, database.NodesRepository); err != nil {
		return err
	}

	if node.CloudInit != nil {
		if err := cloudinit.RegisterNode(node); err != nil {
			return err
		}
	}

	return nil
}

func DeprovisionNode(nodeName *string) error {
	database := db.GetDB()
	var node common.NodeEntry

	if nodeName == nil {
		var err error
		if node, err = database.NodesRepository.GetSelf(); err != nil {
			return err
		}

		// Deprovision all guest nodes if they exist
		if nodes, err := database.NodesRepository.GetAllGuestNodes(); err != nil {
			return err
		} else {
			for _, node := range nodes {
				if err := DeprovisionNode(&node.NodeName); err != nil {
					return err
				}
			}
		}
	} else {
		var err error
		if node, err = database.NodesRepository.GetGuestNode(*nodeName); err != nil {
			return err
		}
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
	if err := node.NodeDriver.Deprovision(&node.NodeName, database.NodesRepository); err != nil {
		return err
	}

	_ = cloudinit.UnregisterNode(node.NodeName)

	return nil
}

func StartNode(node *common.Node) error {
	driverInfo, err := node.Driver.DriverInfo()
	if err != nil {
		return fmt.Errorf("failed to retrieve node driver info: %w", err)
	}
	if driverInfo.StartMode != common.AgentMode {
		return fmt.Errorf("start expects agent mode")
	}
	if err := node.Driver.Start(&node.Name, db.GetDB().NodesRepository); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
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

func StopNode(nodeName *string, message string, time uint32, force bool) error {
	node, err := getNode(nodeName)
	if err != nil {
		return fmt.Errorf("node not provisioned")
	}

	driverInfo, err := node.NodeDriver.DriverInfo()
	if err != nil {
		return fmt.Errorf("cannot retrieve driver info for %s", node.NodeName)
	}

	if driverInfo.StopMode != common.AgentMode {
		return fmt.Errorf("stop expects agent mode")
	}

	if err := node.NodeDriver.Stop(&node.NodeName, message, time, db.GetDB().NodesRepository, force); err != nil {
		return fmt.Errorf("failed to stop node: %w", err)
	}

	return nil
}

func RestartNode(nodeName *string, message string, time uint32) error {
	node, err := getNode(nodeName)
	if err != nil {
		return fmt.Errorf("node not provisioned")
	}

	driverInfo, err := node.NodeDriver.DriverInfo()
	if err != nil {
		return fmt.Errorf("cannot retrieve driver info for %s", node.NodeName)
	}

	if driverInfo.RestartMode != common.AgentMode {
		return fmt.Errorf("restart expects agent mode")
	}

	if err := node.NodeDriver.Restart(&node.NodeName, message, time, db.GetDB().NodesRepository); err != nil {
		return fmt.Errorf("failed to restart node: %w", err)
	}

	return nil
}
