package main

/*
#include <stdlib.h>
typedef const char cchar_t;
*/
import "C"

import (
	"github.com/bitomia/realm/internal/config"
	"github.com/bitomia/realm/internal/drivers"
)

/**
 * Start a client using the realm YAML config file found in the working dir
 */
//export StartClient
func StartClient() {
	drivers.RegisterStdDrivers()
	config.Init(nil)
}

/**
 * Start a client using a config buffer
 */
//export StartClientWithConfig
func StartClientWithConfig(configBuffer *C.cchar_t) {
	drivers.RegisterStdDrivers()
	config.InitFromBuffer(C.GoString(configBuffer))
}

//export GetVersion
func GetVersion() *C.char {
	version := config.GetVersion()
	return C.CString(version)
}

func main() {}
