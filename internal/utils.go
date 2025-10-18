package internal

import (
	"fmt"
	"os"
	"syscall"
)

func StringToSignal(s string) (os.Signal, bool) {
	signals := map[string]os.Signal{
		"SIGABRT":   syscall.SIGABRT,
		"SIGALRM":   syscall.SIGALRM,
		"SIGBUS":    syscall.SIGBUS,
		"SIGCHLD":   syscall.SIGCHLD,
		"SIGCONT":   syscall.SIGCONT,
		"SIGFPE":    syscall.SIGFPE,
		"SIGHUP":    syscall.SIGHUP,
		"SIGILL":    syscall.SIGILL,
		"SIGINT":    syscall.SIGINT,
		"SIGKILL":   syscall.SIGKILL,
		"SIGPIPE":   syscall.SIGPIPE,
		"SIGQUIT":   syscall.SIGQUIT,
		"SIGSEGV":   syscall.SIGSEGV,
		"SIGSTOP":   syscall.SIGSTOP,
		"SIGTERM":   syscall.SIGTERM,
		"SIGTRAP":   syscall.SIGTRAP,
		"SIGTSTP":   syscall.SIGTSTP,
		"SIGTTIN":   syscall.SIGTTIN,
		"SIGTTOU":   syscall.SIGTTOU,
		"SIGUSR1":   syscall.SIGUSR1,
		"SIGUSR2":   syscall.SIGUSR2,
		"SIGPROF":   syscall.SIGPROF,
		"SIGSYS":    syscall.SIGSYS,
		"SIGURG":    syscall.SIGURG,
		"SIGVTALRM": syscall.SIGVTALRM,
		"SIGWINCH":  syscall.SIGWINCH,
		"SIGXCPU":   syscall.SIGXCPU,
		"SIGXFSZ":   syscall.SIGXFSZ,
	}

	sig, ok := signals[s]
	return sig, ok
}

func SignalToString(sig os.Signal) string {
	signals := map[os.Signal]string{
		syscall.SIGABRT:   "SIGABRT",
		syscall.SIGALRM:   "SIGALRM",
		syscall.SIGBUS:    "SIGBUS",
		syscall.SIGCHLD:   "SIGCHLD",
		syscall.SIGCONT:   "SIGCONT",
		syscall.SIGFPE:    "SIGFPE",
		syscall.SIGHUP:    "SIGHUP",
		syscall.SIGILL:    "SIGILL",
		syscall.SIGINT:    "SIGINT",
		syscall.SIGKILL:   "SIGKILL",
		syscall.SIGPIPE:   "SIGPIPE",
		syscall.SIGQUIT:   "SIGQUIT",
		syscall.SIGSEGV:   "SIGSEGV",
		syscall.SIGSTOP:   "SIGSTOP",
		syscall.SIGTERM:   "SIGTERM",
		syscall.SIGTRAP:   "SIGTRAP",
		syscall.SIGTSTP:   "SIGTSTP",
		syscall.SIGTTIN:   "SIGTTIN",
		syscall.SIGTTOU:   "SIGTTOU",
		syscall.SIGUSR1:   "SIGUSR1",
		syscall.SIGUSR2:   "SIGUSR2",
		syscall.SIGPROF:   "SIGPROF",
		syscall.SIGSYS:    "SIGSYS",
		syscall.SIGURG:    "SIGURG",
		syscall.SIGVTALRM: "SIGVTALRM",
		syscall.SIGWINCH:  "SIGWINCH",
		syscall.SIGXCPU:   "SIGXCPU",
		syscall.SIGXFSZ:   "SIGXFSZ",
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
