package common

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	AgentSchemeHTTP  = "http"
	AgentSchemeHTTPS = "https"
	AgentSchemeUnix  = "unix"
)

const unixHost = "http://unix"

var (
	agentProtocolsMu sync.RWMutex
	agentProtocols   = map[string]func(*url.URL) (*AgentTarget, error){}
)

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
	BaseURL   string
	Transport http.RoundTripper
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
		target := &AgentTarget{BaseURL: strings.TrimSuffix(agentURL, "/")}
		if isMDNSHost(parsed.Hostname()) {
			target.Transport = agentMDNSTransport()
		}
		return target, nil

	case AgentSchemeUnix:
		socketPath, err := unixSocketPath(parsed)
		if err != nil {
			return nil, fmt.Errorf("invalid agent url '%s': %w", agentURL, err)
		}
		return &AgentTarget{BaseURL: unixHost, Transport: unixTransport(socketPath)}, nil

	default:
		agentProtocolsMu.RLock()
		handler, ok := agentProtocols[parsed.Scheme]
		agentProtocolsMu.RUnlock()
		if ok {
			target, err := handler(parsed)
			if err != nil {
				return nil, fmt.Errorf("invalid agent url '%s': %w", agentURL, err)
			}
			return target, nil
		}
		return nil, fmt.Errorf("unsupported agent url scheme '%s'", parsed.Scheme)
	}
}

func (t *AgentTarget) URL(path string) string {
	return t.BaseURL + path
}

func (t *AgentTarget) Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: t.Transport}
}

func RegisterAgentProtocol(protocol string, handler func(*url.URL) (*AgentTarget, error)) {
	if handler == nil {
		panic("RegisterAgentProtocol: nil handler for protocol " + protocol)
	}

	switch protocol {
	case AgentSchemeHTTP, AgentSchemeHTTPS, AgentSchemeUnix:
		panic("RegisterAgentProtocol: cannot register reserved protocol '" + protocol + "'")
	}

	agentProtocolsMu.Lock()
	defer agentProtocolsMu.Unlock()

	if _, exists := agentProtocols[protocol]; exists {
		panic("RegisterAgentProtocol: protocol '" + protocol + "' already registered")
	}
	agentProtocols[protocol] = handler
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
