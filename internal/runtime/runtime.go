package runtime

type RuntimeType string
type RuntimeState string

const (
	// RuntimeStart indicates the runtime should be running
	RuntimeStart RuntimeState = "start"
	// RuntimeStartFailed indicates the runtime failed to start
	RuntimeStartFailed RuntimeState = "start_failed"
	// RuntimeStop indicates the runtime should be stopped
	RuntimeStop RuntimeState = "stop"
	// RuntimeStopFailed indicates the runtime failed to stop
	RuntimeStopFailed RuntimeState = "stop_failed"
)

type Runtime interface {
	GetKey() string
	GetState() RuntimeState
	GetRuntimeType() RuntimeType
	MarshalJSON() ([]byte, error)
	UnmarshalJSON(data []byte) error
}
