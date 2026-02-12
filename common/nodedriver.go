package common

type NodeDriverID string

type NodeDriverInfo struct {
	ID  NodeDriverID
	New func(config *any) (NodeDriver, error)
}

type NodeStatusCode string

const (
	NodeStatusUnreachable NodeStatusCode = "unreachable"
	NodeStatusNotDeployed NodeStatusCode = "not deployed"
	NodeStatusPlanned     NodeStatusCode = "planned"
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

	// Verify checks whether the driver options are valid.
	Verify() error

	// MarshalJSON serializes the driver into JSON.
	MarshalJSON() ([]byte, error)

	// UnmarshalJSON deserializes the driver from JSON.
	UnmarshalJSON(data []byte) error

	// Plan validates prerequisites and creates or replace the current database entry.
	// It shall check node requirements but it won't check depending nodes.
	// This is invoked within the daemon and does not affect client behavior.
	Plan(nodeName string, repository NodesRepository) error

	// Unplan cleanup and removes the node
	// Only operates on deployments in "planned" status.
	Unplan(repository NodesRepository) error

	// Startup starts the node
	Startup() error

	// Shutdown shuts down the node
	// Message will be shown to users before shutdown on the time
	// offset specified
	Shutdown(message string, time uint32) error

	// Restart restarts the node
	// Message will be shown to users before shutdown on the time
	// offset specified
	Restart(message string, time uint32) error

	// GetDriverConfig returns the configuration for this node driver.
	GetDriverConfig() NodeDriverConfig

	// UpdateStatus update and returns current status based on internal drivers factors
	UpdateStatus() (NodeStatus, error)

	// GetCapabilities returns current node capabilities
	GetCapabilities() (Capabilities, error)
}
