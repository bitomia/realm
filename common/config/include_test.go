package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveIncludes_Simple(t *testing.T) {
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

	resolved, err := resolveIncludes(mainPath)
	if err != nil {
		t.Fatalf("resolveIncludes failed: %v", err)
	}
	config, err := InitFromBuffer(string(resolved))
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, config.DataPath, "./test_data")
	assert.Equal(t, config.Daemon.ListenAddress, "127.0.0.1")
	assert.Equal(t, config.Daemon.ListenPort, 12345)
}

func TestResolveIncludes_Nested(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(subdir, "etcd.yaml"), []byte("etcd_mode: server\n"), 0644); err != nil {
		t.Fatal(err)
	}

	daemonYAML := `
listen_port: 12345
etcd: !include sub/etcd.yaml
`
	if err := os.WriteFile(filepath.Join(dir, "daemon.yaml"), []byte(daemonYAML), 0644); err != nil {
		t.Fatal(err)
	}

	mainYAML := `daemon: !include daemon.yaml
`
	mainPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0644); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveIncludes(mainPath)
	if err != nil {
		t.Fatalf("resolveIncludes failed: %v", err)
	}
	config, err := InitFromBuffer(string(resolved))
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, config.Daemon.EtcdMode, "server")
	assert.Equal(t, config.Daemon.ListenPort, 12345)

}

func TestResolveIncludes_MissingFile(t *testing.T) {
	dir := t.TempDir()

	mainYAML := `daemon: !include nonexistent.yaml
`
	mainPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveIncludes(mainPath)

	assert.NotNil(t, err)
}

func TestResolveIncludes_NoIncludes(t *testing.T) {
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

	resolved, err := resolveIncludes(mainPath)
	if err != nil {
		t.Fatalf("resolveIncludes failed: %v", err)
	}
	config, err := InitFromBuffer(string(resolved))
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, config.DataPath, "./another_test")
	assert.Equal(t, config.Daemon.ListenPort, 12346)
}
