package common

type NodeDriverID string

type Mode int

const (
	ClientMode Mode = iota
	DaemonMode
)

type NodeDriverBuilder func(config *any) (NodeDriver, error)
type NodeDriverInfo struct {
	ID           NodeDriverID
	New          NodeDriverBuilder
	StartupMode  Mode
	ShutdownMode Mode
	RestartMode  Mode
}

type NewNodeDriverInfoOpts func(i *NodeDriverInfo) error

func WithStartupMode(m Mode) NewNodeDriverInfoOpts {
	return func(i *NodeDriverInfo) error {
		i.StartupMode = m
		return nil
	}
}

func WithShutdownMode(m Mode) NewNodeDriverInfoOpts {
	return func(i *NodeDriverInfo) error {
		i.ShutdownMode = m
		return nil
	}
}

func WithRestartMode(m Mode) NewNodeDriverInfoOpts {
	return func(i *NodeDriverInfo) error {
		i.RestartMode = m
		return nil
	}
}

func NewNodeDriverInfo(id NodeDriverID, builder NodeDriverBuilder, opts ...NewNodeDriverInfoOpts) (NodeDriverInfo, error) {
	info := NodeDriverInfo{
		ID:           id,
		New:          builder,
		StartupMode:  DaemonMode,
		ShutdownMode: DaemonMode,
		RestartMode:  DaemonMode,
	}
	for _, o := range opts {
		if err := o(&info); err != nil {
			return NodeDriverInfo{}, err
		}
	}
	return info, nil
}

type NodeStatusCode string

const (
	NodeStatusOffline NodeStatusCode = "offline"
	NodeStatusOnline  NodeStatusCode = "online"
	NodeStatusReady   NodeStatusCode = "ready"
	NodeStatusError   NodeStatusCode = "error"
)

type NodeStatus struct {
	StatusCode NodeStatusCode `json:"status"`
	Reason     string         `json:"reason"`
}

type NodeDriver interface {
	// GetNodeDriverID returns the unique identifier for this node driver.
	GetNodeDriverID() NodeDriverID

	// DriverInfo returns metadata describing the driver for internal factory use.
	DriverInfo() (NodeDriverInfo, error)

	// GetDriverConfig returns the configuration for this node driver.
	GetDriverConfig() NodeDriverConfig

	// GetCapabilities returns current node capabilities
	GetCapabilities() (Capabilities, error)

	// MarshalJSON serializes the driver into JSON.
	MarshalJSON() ([]byte, error)

	// UnmarshalJSON deserializes the driver from JSON.
	UnmarshalJSON(data []byte) error

	// Startup starts the node
	//
	// nodeName as nil for self-node
	Startup(nodeName *string, repository NodesRepository) error

	// Shutdown shuts down the node
	// Message will be shown to users before shutdown on the time
	// offset specified
	//
	// nodeName as nil for self-node
	Shutdown(nodeName *string, message string, time uint32, repository NodesRepository) error

	// Restart restarts the node
	// Message will be shown to users before shutdown on the time
	// offset specified
	//
	// nodeName as nil for self-node
	Restart(nodeName *string, message string, time uint32, repository NodesRepository) error

	// UpdateStatus update and returns current status based on internal drivers factors
	//
	// nodeName as nil for self-node
	UpdateStatus(nodeName *string, repository NodesRepository) (NodeStatus, error)

	// Provision validates self-node prerequisites and creates or replace the current
	// database entry.
	// Notice that nodes are nameless, provisioning is also the action of naming the self-node
	// It shall check node requirements but it won't check depending nodes.
	// This is invoked within the daemon and does not affect client behavior.
	Provision(nodeName string, repository NodesRepository) error

	// Deprovision cleanup and removes the self-node
	// Only operates on deployments in "provisioned" status.
	Deprovision(repository NodesRepository) error
}
