package loads

import (
	"github.com/bitomia/realm/internal"
)

type LoadDriverType string

type LoadDriver interface {
	GetDriverType() LoadDriverType
	Verify() error
	VerifyDaemon() error
	MarshalJSON() ([]byte, error)
	UnmarshalJSON(data []byte) error
	StartOnDaemon(repository LoadsRepository, logsPath internal.LogsPath, loadName string) error
	StopOnDaemon(repository LoadsRepository, loadName string) error
}
