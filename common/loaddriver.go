package common

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
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

	// MarshalJSON serializes the driver into JSON.
	MarshalJSON() ([]byte, error)

	// UnmarshalJSON deserializes the driver from JSON.
	UnmarshalJSON(data []byte) error

	// Provision validates prerequisites and creates a deployment in "provisioned" status.
	// It shall check load requirements but it won't check depending loads.
	// This is invoked within the daemon and does not affect client behavior.
	//
	// Returns the deployment ID for the provisioned deployment.
	Provision(node NodeDriver, repository DeploymentsRepository, loadName string) (DeploymentID, error)

	// Deprovision removes a provisioned deployment with cleanup
	// Only operates on deployments in "provisioned" status.
	Deprovision(repository DeploymentsRepository, deployment Deployment) error

	// Start starts the load execution for a provisioned deployment.
	// This has no effect when called from the client.
	//
	// LoadDriver is responsible of the consistency of the DeploymentsRepository
	Start(repository DeploymentsRepository, deployment Deployment) error

	// Stop stops a running load execution within the daemon.
	// This has no effect when called from the client.
	//
	// LoadDriver is responsible of the consistency of the DeploymentsRepository
	Stop(repository DeploymentsRepository, deployment Deployment) error

	// Kill stops immediately a running load execution within the daemon.
	// This has no effect when called from the client.
	//
	// LoadDriver is responsible of the consistency of the DeploymentsRepository
	Kill(repository DeploymentsRepository, deployment Deployment) error

	// UpdateStatus update and returns current status based on internal drivers factors.
	UpdateStatus(repository DeploymentsRepository, deployment Deployment) (DeploymentStatus, error)

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
	slog.Error(msg, args...)
	var reason strings.Builder
	reason.WriteString(msg)
	for i := 0; i+1 < len(args); i += 2 {
		fmt.Fprintf(&reason, " %v=%v", args[i], args[i+1])
	}
	return repository.UpdateStatus(deployment.ID, DeploymentStatus{StatusCode: DeploymentStatusError, Reason: reason.String()})
}
