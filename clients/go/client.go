package golang

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
)

const (
	requestTimeout         = 120 * time.Second
	discoveryLookupTimeout = 2 * time.Second
)

type Client struct {
	header         http.Header
	cfg            *config.Config
	discovered     []common.DiscoveredAgent
	discoverCached bool
}

func NewClient(cfg *config.Config) Client {
	token := getAuthToken()
	if token == "" {
		return Client{
			header: http.Header{
				"ContentType": []string{"application/json"},
			},
			cfg: cfg,
		}
	}
	return Client{
		header: http.Header{
			"ContentType":   []string{"application/json"},
			"Authorization": []string{fmt.Sprintf("Bearer %s", token)},
		},
		cfg: cfg,
	}
}

func (c *Client) agentAddress(agentURL string) string {
	filterByMeshIP := func(ips []net.IP, meshNets []*net.IPNet) net.IP {
		candidates := ips
		var mesh []net.IP
		for _, ip := range ips {
			if slices.ContainsFunc(meshNets, func(n *net.IPNet) bool { return n.Contains(ip) }) {
				mesh = append(mesh, ip)
			}
		}
		if len(mesh) > 0 {
			candidates = mesh
		}
		for _, ip := range candidates {
			if ip4 := ip.To4(); ip4 != nil {
				return ip4
			}
		}
		return nil
	}

	parsedURL, err := url.Parse(agentURL)
	if err != nil || !strings.HasSuffix(strings.ToLower(parsedURL.Hostname()), ".local") {
		return agentURL
	}

	if !c.discoverCached {
		c.discovered, _ = common.DiscoverAgents(context.Background(), discoveryLookupTimeout)
		c.discoverCached = true
	}

	for _, agent := range c.discovered {
		if !matchesDiscoveredAgent(agentURL, agent) {
			continue
		}

		ip := filterByMeshIP(agent.IPs, meshNetworks())
		if ip == nil {
			return agentURL
		}

		if port := parsedURL.Port(); port != "" {
			parsedURL.Host = net.JoinHostPort(ip.String(), port)
		} else {
			parsedURL.Host = ip.String()
		}

		return parsedURL.String()
	}

	return agentURL
}

// doRequest executes an HTTP request and returns the response body along with
// the HTTP status code. If the request fails before a response is received
// (e.g. transport error or malformed request), the returned status code is -1.
// A non-200 status yields an error whose message is the raw response body.
func (c *Client) doRequest(method, agentURL, path string, payload io.Reader, timeout time.Duration) ([]byte, int, error) {
	target, err := common.ResolveAgentTarget(c.agentAddress(agentURL))
	if err != nil {
		return nil, -1, err
	}

	client := target.Client(timeout)
	req, err := http.NewRequest(method, target.URL(path), payload)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("%s", string(body))
	}
	return body, resp.StatusCode, nil
}

// doJSONRequest JSON-encodes the payload and executes an HTTP request,
// returning the response body along with the HTTP status code. If the request
// fails before a response is received (e.g. transport error or malformed
// request), the returned status code is -1. A non-200 status yields an error
// whose message is the raw response body.
func (c *Client) doJSONRequest(method, agentURL, path string, payload any, timeout time.Duration) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(payload); err != nil {
			return nil, -1, err
		}
		body = buf
	}
	return c.doRequest(method, agentURL, path, body, timeout)
}

// doStreamRequest executes an HTTP request and returns the raw response for streaming.
func (c *Client) doStreamRequest(method, agentURL, path string, body io.Reader) (*http.Response, error) {
	target, err := common.ResolveAgentTarget(c.agentAddress(agentURL))
	if err != nil {
		return nil, err
	}

	client := target.Client(0)
	req, err := http.NewRequest(method, target.URL(path), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("%s", string(body))
	}
	return resp, nil
}

func (c *Client) GetSystemInfo(node string) (*dto.SystemInfo, error) {
	body, _, err := c.doRequest("GET", node, "/system", nil, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var info dto.SystemInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return &info, nil
}

func (c *Client) GetVersion(node string) (string, error) {
	body, _, err := c.doRequest("GET", node, "/version", nil, 5*time.Second)
	if err != nil {
		return "", err
	}

	var res struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %v", err)
	}

	return res.Version, nil
}

// Authentication
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func (c *Client) Login(node string, username string, password string) (string, error) {
	target, err := common.ResolveAgentTarget(node)
	if err != nil {
		return "", err
	}
	client := target.Client(10 * time.Second)

	request := LoginRequest{
		Username: username,
		Password: password,
	}
	payload := new(bytes.Buffer)
	if err := json.NewEncoder(payload).Encode(request); err != nil {
		return "", fmt.Errorf("failed to encode request: %v", err)
	}

	req, err := http.NewRequest("POST", target.URL("/login"), payload)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	// Don't use authorization header for login
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed: %s", string(body))
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return "", fmt.Errorf("failed to parse login response: %v", err)
	}

	return loginResp.Token, nil
}
