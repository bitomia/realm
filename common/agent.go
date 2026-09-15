package common

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

const (
	AgentSchemeHTTP  = "http"
	AgentSchemeHTTPS = "https"
	AgentSchemeUnix  = "unix"
)

const unixHost = "http://unix"

func ResolveAgentURL(nodeConfig *NodeConfig) (string, error) {
	if nodeConfig == nil {
		return "", fmt.Errorf("nil nodeConfig")
	}

	agentURL := strings.TrimSpace(nodeConfig.Url)
	if agentURL == "" {
		return "", fmt.Errorf("node '%s' with empty url", nodeConfig.Name)
	}

	if _, err := ResolveAgentTarget(agentURL); err != nil {
		return "", fmt.Errorf("node '%s': %w", nodeConfig.Name, err)
	}

	return strings.TrimSuffix(agentURL, "/"), nil
}

type AgentTarget struct {
	baseURL   string
	transport http.RoundTripper
}

func ResolveAgentTarget(agentURL string) (*AgentTarget, error) {
	parsed, err := url.Parse(strings.TrimSpace(agentURL))
	if err != nil {
		return nil, fmt.Errorf("invalid agent url '%s': %w", agentURL, err)
	}

	switch parsed.Scheme {
	case AgentSchemeHTTP, AgentSchemeHTTPS:
		if parsed.Host == "" {
			return nil, fmt.Errorf("invalid agent url '%s': missing host", agentURL)
		}
		return &AgentTarget{baseURL: strings.TrimSuffix(agentURL, "/")}, nil

	case AgentSchemeUnix:
		socketPath, err := unixSocketPath(parsed)
		if err != nil {
			return nil, fmt.Errorf("invalid agent url '%s': %w", agentURL, err)
		}
		return &AgentTarget{baseURL: unixHost, transport: unixTransport(socketPath)}, nil

	case "":
		return nil, fmt.Errorf("invalid agent url '%s': missing scheme, expected one of http, https or unix", agentURL)

	default:
		return nil, fmt.Errorf("unsupported agent url scheme '%s', expected one of http, https or unix", parsed.Scheme)
	}
}

func (t *AgentTarget) URL(path string) string {
	return t.baseURL + path
}

func (t *AgentTarget) Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: t.transport}
}

func unixSocketPath(parsed *url.URL) (string, error) {
	if parsed.Host != "" {
		return "", fmt.Errorf("socket path must be absolute, e.g. unix:///run/realm/agent.sock")
	}

	socketPath := parsed.Path
	if socketPath == "" {
		socketPath = parsed.Opaque
	}
	if !filepath.IsAbs(socketPath) {
		return "", fmt.Errorf("socket path must be absolute, e.g. unix:///run/realm/agent.sock")
	}

	return socketPath, nil
}

func unixTransport(socketPath string) http.RoundTripper {
	dialer := &net.Dialer{}
	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
}
