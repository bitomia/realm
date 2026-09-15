//go:build !windows

package agent

import (
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/common/config"
)

func socketPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "agent.sock")
}

func socketGid(t *testing.T, path string) int {
	t.Helper()
	var stat syscall.Stat_t
	require.NoError(t, syscall.Stat(path, &stat))
	return int(stat.Gid)
}

// currentGid is the only group a test can hand the socket to: chowning to a
// group the process does not belong to is reserved to root.
func currentGid(t *testing.T) int {
	t.Helper()
	current, err := user.Current()
	require.NoError(t, err)
	gid, err := strconv.Atoi(current.Gid)
	require.NoError(t, err)
	return gid
}

func TestListenOnSocketKeepsThePermissions(t *testing.T) {
	path := socketPath(t)
	listener, err := listenOnSocket(path, "")
	require.NoError(t, err)
	defer listener.Close()

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o660), info.Mode().Perm())
}

func TestListenOnSocketGivesTheSocketToAGroup(t *testing.T) {
	gid := currentGid(t)

	t.Run("by gid", func(t *testing.T) {
		path := socketPath(t)
		listener, err := listenOnSocket(path, strconv.Itoa(gid))
		require.NoError(t, err)
		defer listener.Close()

		assert.Equal(t, gid, socketGid(t, path))
	})

	t.Run("by name", func(t *testing.T) {
		group, err := user.LookupGroupId(strconv.Itoa(gid))
		if err != nil {
			t.Skipf("no group database entry for gid %d", gid)
		}

		path := socketPath(t)
		listener, err := listenOnSocket(path, group.Name)
		require.NoError(t, err)
		defer listener.Close()

		assert.Equal(t, gid, socketGid(t, path))
	})
}

func TestListenOnSocketWithoutAGroupLeavesItAlone(t *testing.T) {
	path := socketPath(t)
	listener, err := listenOnSocket(path, "")
	require.NoError(t, err)
	defer listener.Close()

	assert.Equal(t, currentGid(t), socketGid(t, path), "the socket keeps the group of the agent")
}

func TestListenOnSocketRejectsAnUnknownGroup(t *testing.T) {
	path := socketPath(t)

	_, err := listenOnSocket(path, "no-such-group-realm-test")
	assert.ErrorContains(t, err, "failed to give socket to group 'no-such-group-realm-test'")

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "a rejected group must not leave a socket behind")
}

// Nothing creates the default group, so its absence must not stop the agent.
func TestListenOnSocketToleratesTheDefaultGroupMissing(t *testing.T) {
	if _, err := user.LookupGroup(config.DefaultSocketGroup); err == nil {
		t.Skipf("this host has a '%s' group", config.DefaultSocketGroup)
	}

	path := socketPath(t)
	listener, err := listenOnSocket(path, config.DefaultSocketGroup)
	require.NoError(t, err)
	defer listener.Close()

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o660), info.Mode().Perm())
	assert.Equal(t, currentGid(t), socketGid(t, path))
}

func TestListenOnSocketReplacesAStaleSocket(t *testing.T) {
	path := socketPath(t)
	stale, err := net.Listen("unix", path)
	require.NoError(t, err)
	// Closing without unlinking is what an unclean shutdown leaves behind.
	stale.(*net.UnixListener).SetUnlinkOnClose(false)
	require.NoError(t, stale.Close())

	listener, err := listenOnSocket(path, "")
	require.NoError(t, err)
	defer listener.Close()

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o660), info.Mode().Perm())
}
