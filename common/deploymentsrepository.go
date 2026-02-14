package common

import (
	"encoding/json"

	"github.com/google/uuid"
)

type DeploymentID = uuid.UUID

type DeploymentStatusCode string

const (
	DeploymentStatusNotReady DeploymentStatusCode = "not ready"
	DeploymentStatusReady    DeploymentStatusCode = "ready"
	DeploymentStatusRunning  DeploymentStatusCode = "running"
	DeploymentStatusStopped  DeploymentStatusCode = "stopped"
	DeploymentStatusError    DeploymentStatusCode = "error"
)

type DeploymentStatus struct {
	StatusCode DeploymentStatusCode `json:"status"`
	Reason     string               `json:"reason"`
}

// A deployment is the object created when a load has been loaded
// in the cluster
// Notice it doesn't reference Load but contains an immutable copy
// because a deployment cannot be modified directly, it must be
// updated from the DeploymentsRepository
type Deployment struct {
	ID         DeploymentID
	LoadName   string
	LoadDriver LoadDriver
	Status     DeploymentStatus
	Metadata   any
}

// Store interface for deployments
// All methods are related to the deployments of the self node
//
// Notice all data is returned by an immutable copy because
// this repository might be stored in a distributed database
// so no memory references must be used
type DeploymentsRepository interface {
	Create(loadName string, driver LoadDriver, status DeploymentStatus, metadata any) (DeploymentID, error)
	UpdateStatus(deploymentID DeploymentID, status DeploymentStatus) error
	UpdateMetadata(deploymentID DeploymentID, updateFn func(metadata any) error) error

	GetAll() ([]Deployment, error)
	GetByLoad(loadName string) ([]Deployment, error)
	GetByLoadAndStatus(loadName string, statusCode DeploymentStatusCode) ([]Deployment, error)
	GetDeployment(deploymentID DeploymentID) (*Deployment, error)

	DeleteByLoad(loadName string) error
	DeleteDeployment(deploymentID uuid.UUID) error
}

// UpdateMetadata is a generic helper that provides type-safe metadata updates.
// It handles the JSON marshal/unmarshal internally so callers get a typed pointer.
func UpdateMetadata[T any](repo DeploymentsRepository, deploymentID DeploymentID, updateFn func(metadata *T) error) error {
	return repo.UpdateMetadata(deploymentID, func(metadataPtr any) error {
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
