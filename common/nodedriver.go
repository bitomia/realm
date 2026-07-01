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
	//
	// nodeName as nil for self-node
	PowerOn(nodeName *string) error

	// PowerOff stops the node immediately
	//
	// nodeName as nil for self-node
	PowerOff(nodeName *string) error

	// Shutdown stops the node
	// Message will be shown to users before stop on the time
	// offset specified
	//
	// nodeName as nil for self-node
	Shutdown(nodeName *string, message string, time uint32) error

	// Restart restarts the node
	// Message will be shown to users before shutdown on the time
	// offset specified
	//
	// nodeName as nil for self-node
	Restart(nodeName *string, message string, time uint32) error

	// RefreshStatus update and returns current status based on internal drivers factors
	//
	// nodeName as nil for self-node
	RefreshStatus(nodeName *string) (NodeStatus, error)

	// State returns current node state like cpu, mem, etc..
	State(nodeName *string) (NodeState, error)

	// Register validates self-node prerequisites and creates or replace the current
	// database entry.
	// Notice that nodes are nameless, registering is also the action of naming the self-node
	// It shall check node requirements but it won't check depending nodes.
	// This is invoked within the agent and does not affect client behavior.
	Register(nodeName string, cloudInit *cloudinit.CloudInit) error

	// Unregister cleanup and removes the self-node if node name is nil or guest node
	// otherwise
	// Only operates on deployments in registered status.
	Unregister(nodeName *string) error
}
