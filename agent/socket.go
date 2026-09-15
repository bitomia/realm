package agent

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/bitomia/realm/common/config"
)

// listenOnSocket creates a unix socket listener at the given path, making sure
// the parent directory exists and that a stale socket left behind by a previous
// run does not prevent binding
func listenOnSocket(socketPath string, group string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}

	if info, err := os.Stat(socketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("%s exists and is not a socket", socketPath)
		}
		if conn, err := net.Dial("unix", socketPath); err == nil {
			conn.Close()
			return nil, fmt.Errorf("%s is already in use by another agent", socketPath)
		}
		if err := os.Remove(socketPath); err != nil {
			return nil, fmt.Errorf("failed to remove stale socket: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}

	// Ownership first: chown resets the setuid and setgid bits of the file.
	if err := setSocketGroup(socketPath, group); err != nil {
		listener.Close()
		return nil, err
	}

	if err := os.Chmod(socketPath, 0o660); err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	return listener, nil
}

func setSocketGroup(socketPath string, group string) error {
	if group == "" {
		return nil
	}

	gid, err := lookupGid(group)
	if err == nil {
		// -1 keeps the current owner: the agent need not run as root.
		err = os.Chown(socketPath, -1, gid)
	}
	if err != nil && group != config.DefaultSocketGroup {
		return fmt.Errorf("failed to give socket to group '%s': %w", group, err)
	}

	return nil
}

// lookupGid resolves a group name or a numeric gid into a gid
func lookupGid(nameOrGid string) (int, error) {
	if gid, err := strconv.Atoi(nameOrGid); err == nil {
		return gid, nil
	}

	found, err := user.LookupGroup(nameOrGid)
	if err != nil {
		return -1, err
	}

	return strconv.Atoi(found.Gid)
}
