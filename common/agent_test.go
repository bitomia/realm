package common

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAgentURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"http", "http://192.168.1.54:9000", "http://192.168.1.54:9000"},
		{"https", "https://node.example.com:9000", "https://node.example.com:9000"},
		{"http with base path", "http://192.168.1.54:9000/agent", "http://192.168.1.54:9000/agent"},
		{"http trailing slash", "http://192.168.1.54:9000/", "http://192.168.1.54:9000"},
		{"unix", "unix:///run/realm/agent.sock", "unix:///run/realm/agent.sock"},
		{"unix without authority", "unix:/run/realm/agent.sock", "unix:/run/realm/agent.sock"},
		{"surrounding spaces", " http://192.168.1.54:9000 ", "http://192.168.1.54:9000"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := ResolveAgentURL(&NodeConfig{Name: "lab1", Url: test.url})
			require.NoError(t, err)
			assert.Equal(t, test.expected, resolved)
		})
	}
}

func TestResolveAgentURLErrors(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"no scheme", "192.168.1.54:9000"},
		{"unsupported scheme", "ftp://192.168.1.54:9000"},
		{"http without host", "http:///images"},
		{"unix with relative path", "unix://run/realm/agent.sock"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ResolveAgentURL(&NodeConfig{Name: "lab1", Url: test.url})
			assert.Error(t, err)
		})
	}
}

func TestAgentTargetURL(t *testing.T) {
	tests := []struct {
		name     string
		agentURL string
		path     string
		expected string
	}{
		{"http", "http://192.168.1.54:9000", "/images", "http://192.168.1.54:9000/images"},
		{"http with query", "http://192.168.1.54:9000", "/node?guest=lab1", "http://192.168.1.54:9000/node?guest=lab1"},
		{"http with base path", "http://192.168.1.54:9000/agent", "/images", "http://192.168.1.54:9000/agent/images"},
		{"unix", "unix:///run/realm/agent.sock", "/images", "http://unix/images"},
		{"unix with query", "unix:///run/realm/agent.sock", "/node?guest=lab1", "http://unix/node?guest=lab1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target, err := ResolveAgentTarget(test.agentURL)
			require.NoError(t, err)
			assert.Equal(t, test.expected, target.URL(test.path))
		})
	}
}

func TestAgentTargetTransport(t *testing.T) {
	tcp, err := ResolveAgentTarget("http://192.168.1.54:9000")
	require.NoError(t, err)
	assert.Nil(t, tcp.Client(0).Transport, "TCP targets use the default transport")

	socket, err := ResolveAgentTarget("unix:///run/realm/agent.sock")
	require.NoError(t, err)
	assert.NotNil(t, socket.Client(0).Transport, "unix targets dial the socket")
}

func TestAgentTargetOverUnixSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "agent.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s %s", r.Method, r.URL.String())
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	target, err := ResolveAgentTarget("unix://" + socketPath)
	require.NoError(t, err)

	resp, err := target.Client(5 * time.Second).Get(target.URL("/node?guest=lab1"))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "GET /node?guest=lab1", string(body))
}
