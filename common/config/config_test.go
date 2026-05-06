package config

import (
	//	"fmt"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resolveConfigFromPath(t *testing.T, path string) ([]byte, error) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return resolveConfig(f, filepath.Dir(path))
}

func TestConfigIncludes_Simple(t *testing.T) {
	dir := t.TempDir()

	daemonYAML := `
listen_address: 127.0.0.1
listen_port: 12345
`
	if err := os.WriteFile(filepath.Join(dir, "daemon.yaml"), []byte(daemonYAML), 0644); err != nil {
		t.Fatal(err)
	}

	mainYAML := `
data_path: ./test_data
daemon: !include daemon.yaml
`
	mainPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0644); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveConfigFromPath(t, mainPath)
	if err != nil {
		t.Fatalf("resolveIncludes failed: %v", err)
	}
	config, err := InitFromBuffer(string(resolved))
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, config.DataPath, "./test_data")
	assert.Equal(t, config.Daemon.ListenAddress, "127.0.0.1")
	assert.Equal(t, config.Daemon.ListenPort, 12345)
}

func TestConfigIncludes_Nested(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(subdir, "etcd_mode.yaml"), []byte("server\n"), 0644); err != nil {
		t.Fatal(err)
	}

	daemonYAML := `
listen_port: 12345
etcd_mode: !include sub/etcd_mode.yaml
`
	if err := os.WriteFile(filepath.Join(dir, "daemon.yaml"), []byte(daemonYAML), 0644); err != nil {
		t.Fatal(err)
	}

	mainYAML := `daemon: !include daemon.yaml`
	mainPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0644); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveConfigFromPath(t, mainPath)
	if err != nil {
		t.Fatalf("resolveIncludes failed: %v", err)
	}
	config, err := InitFromBuffer(string(resolved))
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, config.Daemon.EtcdMode, "server")
	assert.Equal(t, config.Daemon.ListenPort, 12345)
}

func TestConfigIncludes_MissingFile(t *testing.T) {
	dir := t.TempDir()

	mainYAML := `daemon: !include nonexistent.yaml
`
	mainPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveConfigFromPath(t, mainPath)

	require.NotNil(t, err)
}

func TestConfigIncludes_NoIncludes(t *testing.T) {
	dir := t.TempDir()

	mainYAML := `
data_path: ./another_test
daemon:
    listen_port: 12346
`
	mainPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0644); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveConfigFromPath(t, mainPath)
	if err != nil {
		t.Fatalf("resolveIncludes failed: %v", err)
	}
	config, err := InitFromBuffer(string(resolved))
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, config.DataPath, "./another_test")
	assert.Equal(t, config.Daemon.ListenPort, 12346)
}

func TestConfig_EnvVars(t *testing.T) {
	os.Setenv("TEST_REALM_LISTEN_PORT", "202123")
	yaml := `
daemon:
    listen_port: ${TEST_REALM_LISTEN_PORT}
`
	config, err := readConfigFromReader(bytes.NewBuffer([]byte(yaml)))
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, config.Daemon.ListenPort, 202123)
}
