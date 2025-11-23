package loads

import (
	"github.com/bitomia/realm/internal"
)

type LoadDriverID string

type LoadDriverInfo struct {
	ID  LoadDriverID
	New func(config map[string]interface{}) (LoadDriver, error)
}

type LoadDriver interface {
	GetLoadDriverID() LoadDriverID
	DriverInfo() LoadDriverInfo
	Plan() error
	PlanDaemon() error
	MarshalJSON() ([]byte, error)
	UnmarshalJSON(data []byte) error
	StartOnDaemon(repository LoadsRepository, logsPath internal.LogsPath, loadName string) error
	StopOnDaemon(repository LoadsRepository, loadName string) error
}
