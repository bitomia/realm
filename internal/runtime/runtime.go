package runtime

type RuntimeType string
type RuntimeStatus string

const (
	// RuntimeStart indicates the runtime should be running
	RuntimeStart RuntimeStatus = "start"
	// RuntimeStartFailed indicates the runtime failed to start
	RuntimeStartFailed RuntimeStatus = "start_failed"
	// RuntimeStop indicates the runtime should be stopped
	RuntimeStop RuntimeStatus = "stop"
	// RuntimeStopFailed indicates the runtime failed to stop
	RuntimeStopFailed RuntimeStatus = "stop_failed"
)

type Runtime interface {
	GetKey() string
	GetStatus() RuntimeStatus
	GetRuntimeType() RuntimeType
	MarshalJSON() ([]byte, error)
	UnmarshalJSON(data []byte) error
}
