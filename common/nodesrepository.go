package common

import (
	"encoding/json"

	"github.com/bitomia/realm/common/cloudinit"
)

type NodeEntry struct {
	NodeName   string
	NodeDriver NodeDriver
	CloudInit  *cloudinit.CloudInit
	Metadata   any
}

// Store interface for nodes
//
// Notice all data is returned by an immutable copy because
// this repository might be stored in a distributed database
// so no memory references must be used
type NodesRepository interface {
	// SetSelf creates or updates the node for the caller
	SetSelf(nodeName string, driver NodeDriver, cloudInit *cloudinit.CloudInit, metadata any) error

	// GetSelf return nodeentry for the caller node
	GetSelf() (NodeEntry, error)

	// DeleteSelf caller node from repository
	DeleteSelf() error

	// Update self node metadata
	UpdateSelfMetadata(updateFn func(metadataPtr any) error) error

	// Retrieve node entry by daemon ID
	GetByDaemonId(daemonId string) (NodeEntry, error)

	// SetGuestNode creates or update the guest node entry for the caller host node
	SetGuestNode(guestNodeName string, guestDriver NodeDriver, cloudInit *cloudinit.CloudInit, metadata any) error

	// GetGuestNode returns a guest node of the caller host node
	GetGuestNode(guestNodeName string) (NodeEntry, error)

	// GetAllGuestNodes returns all guest nodes of the self node
	GetAllGuestNodes() ([]NodeEntry, error)

	// DeleteGuestNode deletes the guest node entry for the caller host node
	DeleteGuestNode(guestNodeName string, guestDriver NodeDriver, metadata any) error

	// Update guest node metadata
	UpdateGuestMetadata(guestNodeName string, updateFn func(metadataPtr any) error) error
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

func UpdateGuestNodeMetadata[T any](guestNodeName string, repo NodesRepository, updateFn func(metadata *T) error) error {
	return repo.UpdateGuestMetadata(guestNodeName, func(metadataPtr any) error {
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
