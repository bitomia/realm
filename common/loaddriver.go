package common

type LoadDriverID string

type LoadDriverInfo struct {
	ID  LoadDriverID
	New func(config map[string]interface{}) (LoadDriver, error)
}

type LoadDriver interface {
	// GetLoadDriverID returns the unique identifier for this load driver.
	GetLoadDriverID() LoadDriverID

	// DriverInfo returns metadata describing the driver for internal factory use.
	DriverInfo() LoadDriverInfo

	// Verify checks whether the driver options are valid.
	Verify() error

	// PlanDaemon prepares the load execution plan from the daemon side.
	// This is invoked within the daemon and does not affect client behavior.
	PlanDaemon() error

	// MarshalJSON serializes the driver into JSON.
	MarshalJSON() ([]byte, error)

	// UnmarshalJSON deserializes the driver from JSON.
	UnmarshalJSON(data []byte) error

	// StartOnDaemon starts the load execution within the daemon.
	// This has no effect when called from the client.
	StartOnDaemon(repository LoadsRepository, logsPath LogsPath, loadName string) error

	// StopOnDaemon stops the running load execution within the daemon.
	// This has no effect when called from the client.
	StopOnDaemon(repository LoadsRepository, loadName string) error
}
