package common

type NodeDriverID string

type NodeDriverInfo struct {
	ID  NodeDriverID
	New func(config *any) (NodeDriver, error)
}

type NodeStatus string

const (
	NodeNotAvailable NodeStatus = "not_available"
	NodeAvailable    NodeStatus = "available"
)

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

	// GetStatus returns the availability status of the node
	GetStatus() (NodeStatus, error)

	// GetDriverConfig returns the configuration for this node driver.
	GetDriverConfig() NodeDriverConfig
}
