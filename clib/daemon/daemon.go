package main

/*
#include <stdlib.h>
#include <string.h>

typedef const char cchar_t;
*/
import "C"

import (
	"encoding/json"

	"github.com/bitomia/realm/clib/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/daemon"
	"github.com/bitomia/realm/daemon/api"
	"github.com/bitomia/realm/drivers"
)

/**
 * Start a daemon instance using the realm YAML config file found in the working dir
 */
//export StartDaemon
func StartDaemon() *C.char {
	drivers.RegisterStdDrivers()
	err := config.Init(nil)
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}
	daemon.Start()
	return nil
}

/**
 * Start a daemon instance using a config buffer
 */
//export StartDaemonWithConfig
func StartDaemonWithConfig(configBuffer *C.cchar_t) *C.char {
	drivers.RegisterStdDrivers()
	err := config.InitFromBuffer(C.GoString(configBuffer))
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}
	daemon.Start()
	return nil
}

/**
 * Get daemon build version
 * @return build version
 */
//export GetVersion
func GetVersion() *C.char {
	return C.CString(config.GetVersion())
}

//export GetNodeState
func GetNodeState() *C.char {
	state, err := api.GetNodeState()
	if err != nil || state == nil {
		return nil
	}
	b, err := json.Marshal(state)
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}
	return MakeCString(string(b))
}

func main() {}
