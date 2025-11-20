package main

import (
	"C"
	"encoding/json"

	"github.com/bitomia/realm/daemon"
	"github.com/bitomia/realm/daemon/api"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/internal/requests"
)

//export StartDaemon
func StartDaemon() {
	daemon.Start()
}

//export GetVersion
func GetVersion() *C.char {
	version, err := api.GetVersion()
	if err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, map[string]string{"version": version}, ""))
}

//export GetHealthStatus
func GetHealthStatus() *C.char {
	status, err := api.GetHealthStatus()
	if err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, status, ""))
}

//export GetNodeStatus
func GetNodeStatus() *C.char {
	status, err := api.GetNodeStatus()
	if err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, status, ""))
}

//export ListContainers
func ListContainers() *C.char {
	containersList, err := api.ListContainers()
	if err != nil {
		return C.CString(api.ResponseToJSON(false, nil, err.Error()))
	}
	return C.CString(api.ResponseToJSON(true, containersList, ""))
}

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
