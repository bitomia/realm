//go:build ignore

// Minimal PID 1 for microVM guests whose rootfs has no init
// Boot args:
//
//	init=/init -- [mount=<dev>:<dir>[:<fstype>[:<opts>]]]... [ENV_VAR=VALUE]... /hello arg1 arg2
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

// mountPoint is a filesystem init brings up before the payload runs
type mountPoint struct {
	source string
	target string
	fstype string
	flags  uintptr
}

// baseMounts are the pseudo filesystems most payloads can expect
var baseMounts = []mountPoint{
	{source: "proc", target: "/proc", fstype: "proc"},
	{source: "sysfs", target: "/sys", fstype: "sysfs"},
	{source: "devtmpfs", target: "/dev", fstype: "devtmpfs"},
	{source: "devpts", target: "/dev/pts", fstype: "devpts"},
	{source: "tmpfs", target: "/run", fstype: "tmpfs"},
	{source: "tmpfs", target: "/tmp", fstype: "tmpfs"},
}

func main() {
	run()
	syscall.Sync()

	if err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART); err != nil {
		select {}
	}
}

func run() {
	for _, m := range baseMounts {
		mount(m)
	}

	if err := loopbackUp(); err != nil {
		fmt.Fprintf(os.Stderr, "init: cannot bring up lo: %v\n", err)
	}

	argv, env := directives(os.Args[1:])
	if len(argv) == 0 {
		fmt.Println("init: no payload on the command line")
		return
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "init: cannot run %s: %v\n", argv[0], err)
	}
}

// directives consumes the leading mount= and ENV_VAR=VALUE and returns the payload command line
// with the environment to add to it
func directives(args []string) ([]string, []string) {
	var env []string

	for i, arg := range args {
		switch {
		case strings.HasPrefix(arg, "mount="):
			if m, err := parseMount(strings.TrimPrefix(arg, "mount=")); err != nil {
				fmt.Fprintf(os.Stderr, "init: %v\n", err)
			} else {
				mount(m)
			}
		case isEnvAssignment(arg):
			env = append(env, arg)
		default:
			return args[i:], env
		}
	}

	return nil, env
}

func isEnvAssignment(arg string) bool {
	name, _, ok := strings.Cut(arg, "=")
	if !ok || name == "" {
		return false
	}
	for _, r := range name {
		if r != '_' && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func parseMount(spec string) (mountPoint, error) {
	fields := strings.Split(spec, ":")
	if len(fields) < 2 || fields[0] == "" || fields[1] == "" {
		return mountPoint{}, fmt.Errorf("mount=%s: want <dev>:<dir>[:<fstype>[:<opts>]]", spec)
	}

	m := mountPoint{source: fields[0], target: fields[1], fstype: "ext4"}
	if len(fields) > 2 && fields[2] != "" {
		m.fstype = fields[2]
	}
	if len(fields) > 3 {
		for _, opt := range strings.Split(fields[3], ",") {
			switch opt {
			case "ro":
				m.flags |= syscall.MS_RDONLY
			case "noexec":
				m.flags |= syscall.MS_NOEXEC
			case "nosuid":
				m.flags |= syscall.MS_NOSUID
			case "nodev":
				m.flags |= syscall.MS_NODEV
			}
		}
	}

	return m, nil
}

// mount reports failures on stderr rather than aborting because a payload may well not
// need the filesystem that could not be mounted
func mount(m mountPoint) {
	if err := os.MkdirAll(m.target, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "init: cannot create %s: %v\n", m.target, err)
		return
	}

	if err := syscall.Mount(m.source, m.target, m.fstype, m.flags, ""); err != nil {
		if err == syscall.EBUSY {
			return
		}
		fmt.Fprintf(os.Stderr, "init: cannot mount %s on %s: %v\n", m.source, m.target, err)
	}
}

func loopbackUp() error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	var ifreq struct {
		Name  [syscall.IFNAMSIZ]byte
		Flags uint16
		_     [22]byte
	}
	copy(ifreq.Name[:], "lo")

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.SIOCGIFFLAGS, uintptr(unsafe.Pointer(&ifreq))); errno != 0 {
		return errno
	}
	ifreq.Flags |= syscall.IFF_UP | syscall.IFF_RUNNING
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.SIOCSIFFLAGS, uintptr(unsafe.Pointer(&ifreq))); errno != 0 {
		return errno
	}

	return nil
}
