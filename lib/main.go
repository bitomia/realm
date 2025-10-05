package main

import (
	"C"

	"github.com/bitomia/realm/daemon"
)

//export StartDaemon
func StartDaemon() {
	daemon.Start()
}

func main() {}
