package common

type JobDriverID string

type JobDriverInfo struct {
	ID  JobDriverID
	New func(config any) (JobDriver, error)
}

type JobDriver interface {
	ID() JobDriverID

	Info() JobDriverInfo

	Run(w JobResultWriter, args ...string) error

	Config() JobDriverConfig
}
