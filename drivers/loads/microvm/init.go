//go:build ignore

// Minimal PID 1 for microVM guests whose rootfs has no init
// Boot args:
//
//	init=/init -- /hello arg1 arg2
package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	run()
	syscall.Sync()

	if err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART); err != nil {
		select {}
	}
}

func run() {
	argv := os.Args[1:]
	if len(argv) == 0 {
		fmt.Println("init: no payload on the command line")
	} else {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "init: cannot run %s: %v\n", argv[0], err)
		}
	}
}
