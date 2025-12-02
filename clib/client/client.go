package main

/*
#include <stdlib.h>
typedef const char cchar_t;
*/
import "C"

import (
	"github.com/bitomia/realm/clib/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/drivers"
)

/**
 * Start a client using the realm YAML config file found in the working dir
 */
//export StartClient
func StartClient() *C.char {
	drivers.RegisterStdDrivers()
	err := config.Init(nil)
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}
	return nil
}

/**
 * Start a client using a config buffer
 */
//export StartClientWithConfig
func StartClientWithConfig(configBuffer *C.cchar_t) *C.char {
	drivers.RegisterStdDrivers()
	err := config.InitFromBuffer(C.GoString(configBuffer))
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}
	return nil
}

//export GetVersion
func GetVersion() *C.char {
	version := config.GetVersion()
	return C.CString(version)
}

func main() {}
