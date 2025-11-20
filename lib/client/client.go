package main

import (
	"C"

	"github.com/bitomia/realm/internal/config"
)

//export GetVersion
func GetVersion() *C.char {
	version := config.GetVersion()
	return C.CString(version)
}

func main() {}
