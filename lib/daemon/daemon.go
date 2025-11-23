package main

import (
	"C"
	"encoding/json"

	"github.com/bitomia/realm/daemon"
	"github.com/bitomia/realm/daemon/api"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/internal/config"
	"github.com/bitomia/realm/internal/drivers"
	"github.com/bitomia/realm/internal/requests"
)

/**
 * Start a daemon instance using the realm YAML config file found in the working dir
 */
//export StartDaemon
func StartDaemon() {
	drivers.RegisterStdDrivers()
	daemon.Start()
}

/**
 * Get daemon build version
 * @return build version
 */
//export GetVersion
func GetVersion() *C.char {
	return C.CString(config.GetVersion())
}

/**
 * Get health status of the daemon
 * @return JSON response with health status
 */
//export GetHealthStatus
func GetHealthStatus() *C.char {
	status, err := api.GetHealthStatus()
	if err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, status, ""))
}

/**
 * Get node status information
 * @return JSON response with node status
 */
//export GetNodeStatus
func GetNodeStatus() *C.char {
	status, err := api.GetNodeStatus()
	if err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, status, ""))
}

/**
 * List all containers managed by the daemon
 * @return JSON response with list of containers
 */
//export ListContainers
func ListContainers() *C.char {
	containersList, err := api.ListContainers()
	if err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, containersList, ""))
}

/**
 * Create a new container
 * @param containerName Name of the container to create
 * @param optsJSON JSON string with container creation options
 * @return JSON response with creation result
 */
//export CreateContainer
func CreateContainer(containerName *C.char, optsJSON *C.char) *C.char {
	goContainerName := C.GoString(containerName)
	goOptsJSON := C.GoString(optsJSON)

	var opts requests.CreateContainerOpts
	if err := json.Unmarshal([]byte(goOptsJSON), &opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, "invalid JSON options: "+err.Error()))
	}

	if err := api.CreateContainer(goContainerName, opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, map[string]string{"container": goContainerName}, ""))
}

/**
 * Update the state of an existing container
 * @param containerName Name of the container to update
 * @param optsJSON JSON string with container update options
 * @return JSON response with update result
 */
//export UpdateContainerState
func UpdateContainerState(containerName *C.char, optsJSON *C.char) *C.char {
	goContainerName := C.GoString(containerName)
	goOptsJSON := C.GoString(optsJSON)

	var opts containers.UpdateContainerOpts
	if err := json.Unmarshal([]byte(goOptsJSON), &opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, "invalid JSON options: "+err.Error()))
	}

	if err := api.UpdateContainerState(goContainerName, opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, map[string]string{"container": goContainerName}, ""))
}

/**
 * Remove an existing container
 * @param containerName Name of the container to remove
 * @param optsJSON JSON string with container deletion options
 * @return JSON response with removal result
 */
//export RemoveContainer
func RemoveContainer(containerName *C.char, optsJSON *C.char) *C.char {
	goContainerName := C.GoString(containerName)
	goOptsJSON := C.GoString(optsJSON)

	var opts containers.DeleteContainerOpts
	if err := json.Unmarshal([]byte(goOptsJSON), &opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, "invalid JSON options: "+err.Error()))
	}

	if err := api.RemoveContainer(goContainerName, opts); err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, map[string]string{"container": goContainerName}, ""))
}

func main() {}
