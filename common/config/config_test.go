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

	agentYAML := `
listen_address: 127.0.0.1
listen_port: 12345
`
	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte(agentYAML), 0644); err != nil {
		t.Fatal(err)
	}

	mainYAML := `
data_path: ./test_data
agent: !include agent.yaml
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
	assert.Equal(t, config.Agent.ListenAddress, "127.0.0.1")
	assert.Equal(t, config.Agent.ListenPort, 12345)
}

func TestConfigIncludes_Nested(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(subdir, "namespace.yaml"), []byte("realm-test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	agentYAML := `
listen_port: 12345
containerd_namespace: !include sub/namespace.yaml
`
	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte(agentYAML), 0644); err != nil {
		t.Fatal(err)
	}

	mainYAML := `agent: !include agent.yaml`
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
	assert.Equal(t, config.Agent.ContainerdNamespace, "realm-test")
	assert.Equal(t, config.Agent.ListenPort, 12345)
}

func TestConfigIncludes_MissingFile(t *testing.T) {
	dir := t.TempDir()

	mainYAML := `agent: !include nonexistent.yaml
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
agent:
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
	assert.Equal(t, config.Agent.ListenPort, 12346)
}

func TestConfig_EnvVars(t *testing.T) {
	os.Setenv("TEST_REALM_LISTEN_PORT", "202123")
	yaml := `
agent:
    listen_port: ${TEST_REALM_LISTEN_PORT}
`
	config, err := readConfigFromReader(bytes.NewBuffer([]byte(yaml)))
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, config.Agent.ListenPort, 202123)
}

func TestConfig_EnvVarsInCommentIgnored(t *testing.T) {
	os.Unsetenv("TEST_REALM_UNSET_VAR")
	os.Setenv("TEST_REALM_LISTEN_PORT", "202123")
	yaml := `
agent:
    listen_port: ${TEST_REALM_LISTEN_PORT}
    # listen_address: ${TEST_REALM_UNSET_VAR}
`
	config, err := readConfigFromReader(bytes.NewBuffer([]byte(yaml)))
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, config.Agent.ListenPort, 202123)
}
