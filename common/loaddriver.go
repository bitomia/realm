package common

import (
	"fmt"
	"io"
	"log/slog"
)

type LoadDriverID string

type LoadDriverInfo struct {
	ID  LoadDriverID
	New func(config any) (LoadDriver, error)
}

type LoadDriver interface {
	// GetLoadDriverID returns the unique identifier for this load driver.
	GetLoadDriverID() LoadDriverID

	// DriverInfo returns metadata describing the driver for internal factory use.
	DriverInfo() LoadDriverInfo

	// Verify checks whether the driver options are valid.
	Verify() error

	// MarshalJSON serializes the driver into JSON.
	MarshalJSON() ([]byte, error)

	// UnmarshalJSON deserializes the driver from JSON.
	UnmarshalJSON(data []byte) error

	// PlanDeployment validates prerequisites and creates a deployment in "planned" status.
	// It shall check load requirements but it won't check depending loads.
	// This is invoked within the daemon and does not affect client behavior.
	//
	// Returns the deployment ID for the planned deployment.
	PlanDeployment(repository DeploymentsRepository, loadName string) (DeploymentID, error)

	// UnplanDeployment removes a planned deployment with cleanup
	// Only operates on deployments in "planned" status.
	UnplanDeployment(repository DeploymentsRepository, deployment Deployment) error

	// RunDeployment starts the load execution for an existing planned deployment.
	// This has no effect when called from the client.
	//
	// LoadDriver is responsible of the consistency of the DeploymentsRepository
	RunDeployment(repository DeploymentsRepository, deployment Deployment) error

	// StopDeployment stops a running load execution within the daemon.
	// This has no effect when called from the client.
	//
	// LoadDriver is responsible of the consistency of the DeploymentsRepository
	StopDeployment(repository DeploymentsRepository, deployment Deployment) error

	// KillDeployment stops immediately a running load execution within the daemon.
	// This has no effect when called from the client.
	//
	// LoadDriver is responsible of the consistency of the DeploymentsRepository
	KillDeployment(repository DeploymentsRepository, deployment Deployment) error

	// UpdateDeploymentStatus update and returns current state based on internal drivers factors.
	UpdateDeploymentStatus(repository DeploymentsRepository, deployment Deployment) (DeploymentStatus, error)

	// GetDriverConfig returns the configuration for this load driver.
	GetDriverConfig() LoadDriverConfig

	// Stream load stdout to writer
	StreamStdout(repository DeploymentsRepository, deployment Deployment, w io.Writer) error

	// Stream load stderr to writer
	StreamStderr(repository DeploymentsRepository, deployment Deployment, w io.Writer) error

	// Read load stdout from offset, returns bytes read and end position
	ReadStdout(repository DeploymentsRepository, deployment Deployment, offset int64) ([]byte, int64, error)

	// Read load stderr from offset, returns bytes read and end position
	ReadStderr(repository DeploymentsRepository, deployment Deployment, offset int64) ([]byte, int64, error)
}

func SetDeploymentError(repository DeploymentsRepository, deployment Deployment, msg string, args ...any) error {
	slog.Error(msg, args)
	return repository.UpdateStatus(deployment.ID, DeploymentStatus{StatusCode: DeploymentStatusError, Reason: fmt.Sprintf(msg, args)})
}
