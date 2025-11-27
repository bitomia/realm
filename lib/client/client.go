package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/bitomia/realm/internal/config"
)

//export GetVersion
func GetVersion() *C.char {
	version := config.GetVersion()
	return C.CString(version)
}

/**
 * free allocated C string
 */
//export Realm_Free
func Realm_Free(p *C.char) {
	if p != nil {
		C.free(unsafe.Pointer(p))
	}
}

func main() {}
