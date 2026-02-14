package common

type NodeDriverID string

type NodeDriverInfo struct {
	ID  NodeDriverID
	New func(config *any) (NodeDriver, error)
}

type NodeStatusCode string

const (
	NodeStatusUnreachable NodeStatusCode = "unreachable"
	NodeStatusOnline      NodeStatusCode = "online"
	NodeStatusReady       NodeStatusCode = "ready"
	NodeStatusError       NodeStatusCode = "error"
)

type NodeStatus struct {
	StatusCode NodeStatusCode `json:"status"`
	Reason     string         `json:"reason"`
}

type NodeDriver interface {
	// GetNodeDriverID returns the unique identifier for this node driver.
	GetNodeDriverID() NodeDriverID

	// DriverInfo returns metadata describing the driver for internal factory use.
	DriverInfo() NodeDriverInfo

	// MarshalJSON serializes the driver into JSON.
	MarshalJSON() ([]byte, error)

	// UnmarshalJSON deserializes the driver from JSON.
	UnmarshalJSON(data []byte) error

	// Provision validates prerequisites and creates or replace the current database entry.
	// It shall check node requirements but it won't check depending nodes.
	// This is invoked within the daemon and does not affect client behavior.
	Provision(nodeName string, repository NodesRepository) error

	// Deprovision cleanup and removes the node
	// Only operates on deployments in "provisioned" status.
	Deprovision(repository NodesRepository) error

	// Startup starts the node
	Startup() error

	// Shutdown shuts down the node
	// Message will be shown to users before shutdown on the time
	// offset specified
	Shutdown(message string, time uint32, repository NodesRepository) error

	// Restart restarts the node
	// Message will be shown to users before shutdown on the time
	// offset specified
	Restart(message string, time uint32, repository NodesRepository) error

	// GetDriverConfig returns the configuration for this node driver.
	GetDriverConfig() NodeDriverConfig

	// UpdateStatus update and returns current status based on internal drivers factors
	UpdateStatus(repository NodesRepository) (NodeStatus, error)

	// GetCapabilities returns current node capabilities
	GetCapabilities() (Capabilities, error)
}
