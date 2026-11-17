package api

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bitomia/realm/agent/capabilities"
	"github.com/bitomia/realm/agent/cloudinit"
	"github.com/bitomia/realm/agent/cpu"
	"github.com/bitomia/realm/agent/db"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/common/otel"
)

// nodeConfigMutex guards the stored node config: loading and unloading it
// hold the write lock, provisioning a load out of it holds the read lock
var nodeConfigMutex sync.RWMutex

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

// GetNode returns the node state (CPU, memory, etc) and status
func GetNode() (*dto.NodeResponse, error) {
	database := db.GetDB()

	var nodeEntry common.NodeEntry
	var err error
	nodeEntry, err = database.NodesRepository.GetSelf()
	if err != nil {
		return nil, fmt.Errorf("failed to get self node: %w", err)
	}

	return nodeResponse(nodeEntry.Driver)
}

// nodeResponse queries the driver for the node state and status
func nodeResponse(driver common.NodeDriver) (*dto.NodeResponse, error) {
	state, err := driver.State()
	if err != nil {
		return nil, fmt.Errorf("failed to get node state: %w", err)
	}

	status, err := driver.UpdateStatus()
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

// GetNodeConfig returns the node config loaded on this agent
func GetNodeConfig() (*common.Node, error) {
	db := db.GetDB()
	if db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if entry, err := db.NodesRepository.GetSelf(); err != nil {
		return nil, err
	} else {
		return &entry.Node, nil
	}
}

func CheckNodeConfig(node *common.Node) error {
	db := db.GetDB()
	if db == nil {
		return fmt.Errorf("db not initialized")
	}

	return checkNodeConfig(db.NodesRepository, node)
}

func LoadNodeConfig(node *common.Node) error {
	db := db.GetDB()
	if db == nil {
		return fmt.Errorf("db not initialized")
	}

	nodeConfigMutex.Lock()
	defer nodeConfigMutex.Unlock()

	// Loads are stored along with the node config, the client provisions them afterwards
	if err := checkNodeConfig(db.NodesRepository, node); errors.Is(err, common.ErrNodeLiveConfigMismatch) || errors.Is(err, common.ErrNodeLoadsMismatch) {
		return db.NodesRepository.UpdateSelf(*node)
	} else if err != nil {
		return err
	}

	if err := db.NodesRepository.SetSelf(*node); err != nil {
		return err
	}
	otel.SetNodeName(node.Name)
	slog.Info("Agent named", "node", node.Name)

	return nil
}

func ValidateNodeConfig(node *common.Node) error {
	// Node builders shall validate context conditions such as node capabilities
	_, err := common.BuildNodeDriver(common.NewNodeContext(node.Name), node.Driver.Config())
	return err
}

func UnloadNodeConfig() error {
	db := db.GetDB()
	if db == nil {
		return fmt.Errorf("db not initialized")
	}

	nodeConfigMutex.Lock()
	defer nodeConfigMutex.Unlock()

	// Loads shall be deprovisioned before unloading the node
	deployments, err := db.DeploymentsRepository.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get deployments: %w", err)
	}
	if len(deployments) > 0 {
		return fmt.Errorf("cannot unload node config: %d deployments still provisioned", len(deployments))
	}

	if err := db.NodesRepository.DeleteSelf(); err != nil {
		return err
	}
	otel.SetNodeName("")

	return nil
}

func PowerOnNode(node *common.Node) error {
	if info, err := node.Driver.Info(); err != nil {
		return fmt.Errorf("failed to retrieve node driver info: %w", err)
	} else if info.PowerOnMode != common.AgentMode {
		return fmt.Errorf("poweron expects agent mode")
	}

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

func getNode() (*common.NodeEntry, error) {
	db := db.GetDB()

	node, err := db.NodesRepository.GetSelf()
	if err != nil {
		return nil, err
	} else {
		return &node, err
	}
}

func PowerOffNode() error {
	node, err := getNode()
	if err != nil {
		return fmt.Errorf("node configuration not loaded")
	}

	if info, err := node.Driver.Info(); err != nil {
		return fmt.Errorf("failed to retrieve node driver info: %w", err)
	} else if info.PowerOffMode != common.AgentMode {
		return fmt.Errorf("poweroff expects agent mode")
	}

	if err := node.Driver.PowerOff(); err != nil {
		return fmt.Errorf("failed to poweroff node: %w", err)
	}

	_ = cloudinit.UnregisterNode(node.Name)

	return nil
}

func ShutdownNode(message string, time uint32) error {
	node, err := getNode()
	if err != nil {
		return fmt.Errorf("node configuration not loaded")
	}

	if info, err := node.Driver.Info(); err != nil {
		return fmt.Errorf("failed to retrieve node driver info: %w", err)
	} else if info.ShutdownMode != common.AgentMode {
		return fmt.Errorf("shutdown expects agent mode")
	}

	if err := node.Driver.Shutdown(message, time); err != nil {
		return fmt.Errorf("failed to stop node: %w", err)
	}

	_ = cloudinit.UnregisterNode(node.Name)

	return nil
}

func RestartNode(message string, time uint32) error {
	node, err := getNode()
	if err != nil {
		return fmt.Errorf("node configuration not loaded")
	}

	if info, err := node.Driver.Info(); err != nil {
		return fmt.Errorf("failed to retrieve node driver info: %w", err)
	} else if info.RestartMode != common.AgentMode {
		return fmt.Errorf("restart expects agent mode")
	}

	if err := node.Driver.Restart(message, time); err != nil {
		return fmt.Errorf("failed to restart node: %w", err)
	}

	return nil
}
