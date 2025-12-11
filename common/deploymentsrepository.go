package common

import (
	"github.com/google/uuid"
)

type DeploymentID = uuid.UUID

// A deployment is the object create when a load has been loaded
// in the cluster
// Notice it doesn't reference Load but contains an immutable copy
// because a deployment cannot be modified directly, it must be
// updated from the DeploymentsRepository
type Deployment struct {
	ID         DeploymentID
	LoadName   string
	LoadDriver LoadDriver
	Metadata   any
}

// Store interface for deployments
//
// Notice all data is returned by an immutable copy because
// this repository might be stored in a distributed database
// so no memory references must be used
type DeploymentsRepository interface {
	Create(loadName string, driver LoadDriver, metadata any) (DeploymentID, error)

	GetByLoad(loadName string) ([]Deployment, error)
	GetDeployment(deploymentID DeploymentID) (*Deployment, error)

	DeleteByLoad(loadName string) error
	DeleteDeployment(deploymentID uuid.UUID) error
}
