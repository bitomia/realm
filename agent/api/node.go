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

	state, err := nodeEntry.NodeDriver.State()
	if err != nil {
		return nil, fmt.Errorf("failed to get node state: %w", err)
	}

	status, err := nodeEntry.NodeDriver.RefreshStatus()
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

func GetNodeConfig() (*common.NodeDriverConfig, error) {
	db := db.GetDB()
	if db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if node, err := db.NodesRepository.GetSelf(); err != nil {
		return nil, err
	} else {
		config := node.NodeDriver.Config()
		return &config, nil
	}
}

func LoadNodeConfig(node *common.Node) error {
	db := db.GetDB()
	if db == nil {
		return fmt.Errorf("db not initialized")
	}
	if err := db.NodesRepository.SetSelf(node.Name, node.Driver, nil); err != nil {
		return err
	}
	return nil
}

func UnloadGuestNodeConfig(nodeName string) error {
	db := db.GetDB()
	if db == nil {
		return fmt.Errorf("db not initialized")
	}

	var err error
	var node common.NodeEntry
	if node, err = db.NodesRepository.GetGuestNode(nodeName); err != nil {
		return err
	}

	if err := db.NodesRepository.DeleteGuestNode(nodeName, node.NodeDriver, nil); err != nil {
		return err
	}

	return nil
}

func UnloadNodeConfig() error {
	db := db.GetDB()
	if db == nil {
		return fmt.Errorf("db not initialized")
	}

	// Unregister all guest nodes if they exist
	if nodes, err := db.NodesRepository.GetAllGuestNodes(); err != nil {
		return err
	} else {
		for _, node := range nodes {
			if err := UnloadGuestNodeConfig(node.NodeName); err != nil {
				return err
			}
		}
	}

	// Deprovision all deployments on this node before deprovisioning the node
	deployments, err := db.DeploymentsRepository.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get deployments: %w", err)
	}

	for _, deployment := range deployments {
		if err := DeprovisionLoadDeployments(deployment.LoadName); err != nil {
			return fmt.Errorf("failed to deprovision deployment %s: %w", deployment.ID, err)
		}
	}

	if err := db.NodesRepository.DeleteSelf(); err != nil {
		return err
	}

	return nil
}

func PowerOnNode(node *common.Node) error {
	if node.CloudInit != nil {
		if err := cloudinit.RegisterNode(node); err != nil {
			return err
		}
	}

	if err := node.Driver.PowerOn(node.CloudInit); err != nil {
		return fmt.Errorf("failed to poweron node: %w", err)
	}
	return nil
}

// Return self node if nodename is nil, or try to return guest node otherwise
func getNode(nodeName *string) (*common.NodeEntry, error) {
	db := db.GetDB()

	if nodeName == nil {
		node, err := db.NodesRepository.GetSelf()
		if err != nil {
			return nil, err
		} else {
			return &node, err
		}
	} else {
		node, err := db.NodesRepository.GetGuestNode(*nodeName)
		if err != nil {
			return nil, err
		} else {
			return &node, err
		}
	}
}

func PowerOffNode(nodeName *string) error {
	node, err := getNode(nodeName)
	if err != nil {
		return fmt.Errorf("node not registerd")
	}

	if err := node.NodeDriver.PowerOff(); err != nil {
		return fmt.Errorf("failed to poweroff node: %w", err)
	}

	_ = cloudinit.UnregisterNode(node.NodeName)

	return nil
}

func ShutdownNode(nodeName *string, message string, time uint32) error {
	node, err := getNode(nodeName)
	if err != nil {
		return fmt.Errorf("node not registered")
	}

	if err := node.NodeDriver.Shutdown(message, time); err != nil {
		return fmt.Errorf("failed to stop node: %w", err)
	}

	_ = cloudinit.UnregisterNode(node.NodeName)

	return nil
}

func RestartNode(nodeName *string, message string, time uint32) error {
	node, err := getNode(nodeName)
	if err != nil {
		return fmt.Errorf("node not registered")
	}

	if err := node.NodeDriver.Restart(message, time); err != nil {
		return fmt.Errorf("failed to restart node: %w", err)
	}

	return nil
}
