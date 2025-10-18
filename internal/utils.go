package internal

import (
	"fmt"
	"os"
)

const (
	SIGABRT = 0x6
	SIGALRM = 0xe
	SIGHUP  = 0x1
	SIGILL  = 0x4
	SIGINT  = 0x2
	SIGKILL = 0x9
	SIGPWR  = 0x1e
	SIGQUIT = 0x3
	SIGSTOP = 0x13
	SIGTERM = 0xf
	SIGTRAP = 0x5
	SIGUSR1 = 0xa
	SIGUSR2 = 0xc
)

func StringToSignal(s string) (int, bool) {
	signals := map[string]int{
		"SIGABRT": SIGABRT,
		"SIGALRM": SIGALRM,
		"SIGHUP":  SIGHUP,
		"SIGILL":  SIGILL,
		"SIGINT":  SIGINT,
		"SIGKILL": SIGKILL,
		"SIGPWR":  SIGPWR,
		"SIGQUIT": SIGQUIT,
		"SIGSTOP": SIGSTOP,
		"SIGTERM": SIGTERM,
		"SIGTRAP": SIGTRAP,
		"SIGUSR1": SIGUSR1,
		"SIGUSR2": SIGUSR2,
	}

	sig, ok := signals[s]
	return sig, ok
}

func SignalToString(sig int) string {
	signals := map[int]string{
		SIGABRT: "SIGABRT",
		SIGALRM: "SIGALRM",
		SIGHUP:  "SIGHUP",
		SIGILL:  "SIGILL",
		SIGINT:  "SIGINT",
		SIGKILL: "SIGKILL",
		SIGPWR:  "SIGPWR",
		SIGQUIT: "SIGQUIT",
		SIGSTOP: "SIGSTOP",
		SIGTERM: "SIGTERM",
		SIGTRAP: "SIGTRAP",
		SIGUSR1: "SIGUSR1",
		SIGUSR2: "SIGUSR2",
	}

	if s, ok := signals[sig]; ok {
		return s
	}
	return ""
}

func DirExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Directory doesn't exist: %s", path)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("Path is not a directory: %s", path)
	}
	return nil
}
