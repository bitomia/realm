package types

// ContainerState represents the state of a container
type ContainerState string

const (
	// StateStart indicates the container should be running
	StateStart ContainerState = "start"
	// StateStartFailed indicates the container failed to start
	StateStartFailed ContainerState = "start_failed"
	// StateStop indicates the container should be stopped
	StateStop ContainerState = "stop"
	// StateStopFailed indicates the container failed to stop
	StateStopFailed ContainerState = "stop_failed"
)
