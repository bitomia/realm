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
	Create(nodeName string, driver NodeDriver, metadata any) error

	GetByDaemonId(daemonId string) (NodeEntry, error)
	GetSelf() (NodeEntry, error)

	Delete() error
}
