package common

type NodeEntry struct {
	NodeName   string
	NodeDriver NodeDriver
	Metadata   any
}

// Store interface for nodes
//
// Notice all data is returned by an immutable copy because
// this repository might be stored in a distributed database
// so no memory references must be used
type NodesRepository interface {
	// SetSelf creates or updates the node for the caller
	SetSelf(nodeName string, driver NodeDriver, metadata any) error

	// GetSelf return nodeentry for the caller node
	GetSelf() (NodeEntry, error)

	// DeleteSelf caller node from repository
	DeleteSelf() error

	// Retrieve node entry by daemon ID
	GetByDaemonId(daemonId string) (NodeEntry, error)

	// SetGuestNode creates or update the guest node entry for the caller host node
	SetGuestNode(guestNodeName string, guestDriver NodeDriver, metadata any) error

	// DeleteGuestNode deletes the guest node entry for the caller host node
	DeleteGuestNode(guestNodeName string, guestDriver NodeDriver, metadata any) error
}
