package common

import (
	"encoding/json"
	"errors"
	"fmt"
)

type NodeEntry struct {
	Node
	Metadata any
}

type nodeEntryJSON struct {
	Node     *Node `json:"node"`
	Metadata any   `json:"metadata"`
}

func (e NodeEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(nodeEntryJSON{Node: &e.Node, Metadata: e.Metadata})
}

func (e *NodeEntry) UnmarshalJSON(data []byte) error {
	aux := nodeEntryJSON{Node: &e.Node}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	e.Metadata = aux.Metadata
	return nil
}

var (
	ErrNodeNotConfigured      = errors.New("node not configured")
	ErrNodeAlreadyConfigured  = errors.New("node already configured")
	ErrNodeConfigMismatch     = errors.New("node already configured with a different config")
	ErrNodeLoadsMismatch      = fmt.Errorf("%w: loads differ", ErrNodeConfigMismatch)
	ErrNodeLiveConfigMismatch = fmt.Errorf("%w: only timeout or registries differ", ErrNodeConfigMismatch)
)

// Store interface for nodes
//
// Notice all data is returned by an immutable copy because
// this repository might be stored in a distributed database
// so no memory references must be used
type NodesRepository interface {
	// SetSelf creates the node for the caller
	SetSelf(node Node) error

	// UpdateSelf replaces the config of the caller node
	// metadata stays untouched (use UpdateSelfMetadata)
	UpdateSelf(node Node) error

	// GetSelf return nodeentry for the caller node
	GetSelf() (NodeEntry, error)

	// DeleteSelf caller node from repository
	DeleteSelf() error

	// Update self node metadata
	UpdateSelfMetadata(updateFn func(metadataPtr any) error) error

	// Retrieve node entry by agent ID
	GetByAgentId(agentId string) (NodeEntry, error)
}

func UpdateSelfNodeMetadata[T any](repo NodesRepository, updateFn func(metadata *T) error) error {
	return repo.UpdateSelfMetadata(func(metadataPtr any) error {
		ptr := metadataPtr.(*any)

		var metadata T
		if *ptr != nil {
			data, err := json.Marshal(*ptr)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(data, &metadata); err != nil {
				return err
			}
		}

		if err := updateFn(&metadata); err != nil {
			return err
		}

		*ptr = metadata
		return nil
	})
}

func CastMetadata[T any](metadataPtr any) (*T, error) {
	ptr := metadataPtr.(*any)

	var metadata T
	if *ptr != nil {
		data, err := json.Marshal(*ptr)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &metadata); err != nil {
			return nil, err
		}
	}

	return &metadata, nil
}
