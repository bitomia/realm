package common

import "github.com/bitomia/realm/common/cloudinit"

type NodeDriverID string

type Mode int

const (
	ClientMode Mode = iota
	AgentMode
)

type NodeContext struct {
	Repository NodesRepository
	NodeName   *string
}

type NodeDriverBuilder func(ctx NodeContext, config *any) (NodeDriver, error)

type NodeDriverInfo struct {
	ID           NodeDriverID
	New          NodeDriverBuilder
	PowerOnMode  Mode
	PowerOffMode Mode
	ShutdownMode Mode
	RestartMode  Mode
	GuestMode    bool
}

type NewNodeDriverInfoOpts func(i *NodeDriverInfo) error

func WithPowerOnMode(m Mode) NewNodeDriverInfoOpts {
	return func(i *NodeDriverInfo) error {
		i.PowerOnMode = m
		return nil
	}
}

func WithPowerOffMode(m Mode) NewNodeDriverInfoOpts {
	return func(i *NodeDriverInfo) error {
		i.ShutdownMode = m
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

func WithGuestMode() NewNodeDriverInfoOpts {
	return func(i *NodeDriverInfo) error {
		i.GuestMode = true
		return nil
	}
}

func NewNodeDriverInfo(id NodeDriverID, builder NodeDriverBuilder, opts ...NewNodeDriverInfoOpts) (NodeDriverInfo, error) {
	info := NodeDriverInfo{
		ID:           id,
		New:          builder,
		PowerOnMode:  AgentMode,
		ShutdownMode: AgentMode,
		RestartMode:  AgentMode,
		GuestMode:    false,
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
	// ID returns the unique identifier for this node driver.
	ID() NodeDriverID

	// Info returns driver description for internal factory use.
	Info() (NodeDriverInfo, error)

	// Config returns the configuration for this node driver.
	Config() NodeDriverConfig

	// PowerOn starts the node
	PowerOn() error

	// PowerOff stops the node immediately
	PowerOff() error

	// Shutdown stops the node
	Shutdown(message string, time uint32) error

	// Restart restarts the node
	// Message will be shown to users before shutdown on the time
	// offset specified
	Restart(message string, time uint32) error

	// Register node into the cluster
	Register(cloudInit *cloudinit.CloudInit) error

	// Unregister cleanup and removes the node from the cluster
	Unregister() error

	// State returns current node state (e.g. cpu, mem, etc...)
	State() (NodeState, error)

	// RefreshStatus update and returns current status based on internal drivers factors
	//
	// nodeName as nil for self-node
	RefreshStatus() (NodeStatus, error)
}
