package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
)

type Client struct {
	header http.Header
	cfg    *config.Config
}

func getAuthToken() string {
	// First check environment variable for backwards compatibility
	if bearer, exists := os.LookupEnv("REALM_BEARER"); exists {
		return bearer
	}

	// Then check ~/.realmrc file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	realmrcPath := filepath.Join(homeDir, ".realmrc")
	tokenBytes, err := os.ReadFile(realmrcPath)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(tokenBytes))
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

func NewUnauthClient() Client {
	return Client{
		header: http.Header{
			"ContentType": []string{"application/json"},
		},
	}
}

// doRequest executes an HTTP request and returns the response body.
func (c *Client) doRequest(method, url string, payload io.Reader, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", string(body))
	}
	return body, nil
}

// doJSONRequest JSON-encodes the payload and executes an HTTP request.
func (c *Client) doJSONRequest(method, url string, payload any, timeout time.Duration) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		buf := new(bytes.Buffer)
		json.NewEncoder(buf).Encode(payload)
		body = buf
	}
	return c.doRequest(method, url, body, timeout)
}

// doStreamRequest executes an HTTP request and returns the raw response for streaming.
func (c *Client) doStreamRequest(method, url string) (*http.Response, error) {
	client := &http.Client{Timeout: 0}
	req, err := http.NewRequest(method, url, nil)
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

func (c *Client) GetAllImages() (dto.NodeImagesMapResponse, error) {
	nodes := GetNodes(c.cfg)
	var nodeImagesMap dto.NodeImagesMapResponse

	for _, node := range nodes {
		url := fmt.Sprintf("%s/images", node.Url)
		body, err := c.doRequest("GET", url, nil, 10*time.Second)
		if err != nil {
			nodeImagesMap = append(nodeImagesMap, dto.NodeImagesResponse{Node: node.Name, Error: err.Error()})
			continue
		}

		var images dto.ImagesResponse
		if err := json.Unmarshal(body, &images); err != nil {
			nodeImagesMap = append(nodeImagesMap, dto.NodeImagesResponse{Node: node.Name, Error: err.Error()})
			continue
		}
		nodeImagesMap = append(nodeImagesMap, dto.NodeImagesResponse{Node: node.Name, Images: images})
	}
	return nodeImagesMap, nil
}

type ContainerInfo struct {
	ID    string
	Image string
}

type Container struct {
	Container ContainerInfo `json:"container"`
	Status    string        `json:"status"`
}

func (c *Client) GetAllContainers() (map[string]map[string]Container, error) {
	nodes := GetNodes(c.cfg)
	containersPerNode := make(map[string]map[string]Container)

	for _, node := range nodes {
		url := fmt.Sprintf("%s/containers", node.Url)
		body, err := c.doRequest("GET", url, nil, 10*time.Second)
		if err != nil {
			log.Error("Failed to get containers from %s: %v", node.Name, err)
			continue
		}

		var containers map[string]Container
		if err := json.Unmarshal(body, &containers); err != nil {
			log.Error("Failed to parse JSON: %v", err)
			continue
		}
		containersPerNode[node.Name] = containers
	}
	return containersPerNode, nil
}

func (c *Client) ListNetworks() (map[string]any, error) {
	nodes := GetNodes(c.cfg)
	networksPerNode := make(map[string]any)

	for _, node := range nodes {
		url := fmt.Sprintf("%s/network", node.Url)
		body, err := c.doRequest("GET", url, nil, 10*time.Second)
		if err != nil {
			log.Fatal("Failed to get networks from %s: %v", node.Name, err)
		}

		var networkConfig any
		if err := json.Unmarshal(body, &networkConfig); err != nil {
			log.Error("Failed to parse JSON: %v", err)
			continue
		}

		networksPerNode[node.Name] = networkConfig
	}

	return networksPerNode, nil
}

func (c *Client) GetNode(node string) (*dto.NodeResponse, error) {
	url := fmt.Sprintf("%s/node", node)
	body, err := c.doRequest("GET", url, nil, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var status dto.NodeResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("Failed to parse JSON: %v", err)
	}

	return &status, nil
}

func (c *Client) GetSystemInfo(node string) (*dto.SystemInfo, error) {
	url := fmt.Sprintf("%s/system", node)
	body, err := c.doRequest("GET", url, nil, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var info dto.SystemInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("Failed to parse JSON: %v", err)
	}

	return &info, nil
}

func (c *Client) GetContainerLogs(node string, container string) error {
	url := fmt.Sprintf("%s/containers/%s/logs", node, container)
	body, err := c.doRequest("GET", url, nil, 30*time.Second)
	if err != nil {
		return err
	}

	fmt.Println(string(body))
	return nil
}

func (c *Client) GetProxyConfig(node string, container string) error {
	url := fmt.Sprintf("%s/containers/%s/server", node, container)
	body, err := c.doRequest("GET", url, nil, 10*time.Second)
	if err != nil {
		return err
	}

	fmt.Println(string(body))
	return nil
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
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/login", node)

	request := LoginRequest{
		Username: username,
		Password: password,
	}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

	req, err := http.NewRequest("POST", url, payload)
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

func (c *Client) ProvisionLoad(load *common.Load) error {
	url := fmt.Sprintf("%s/loads/provision", load.Node.Url)
	_, err := c.doJSONRequest("POST", url, load, 60*time.Second)
	return err
}

func (c *Client) StartLoad(load *common.Load) error {
	url := fmt.Sprintf("%s/loads/%s/start", load.Node.Url, load.Name)
	_, err := c.doRequest("POST", url, nil, 60*time.Second)
	return err
}

func (c *Client) StopLoad(load *common.Load) error {
	url := fmt.Sprintf("%s/loads/%s/stop", load.Node.Url, load.Name)
	_, err := c.doRequest("POST", url, nil, 60*time.Second)
	return err
}

func (c *Client) KillLoad(load *common.Load) error {
	url := fmt.Sprintf("%s/loads/%s/kill", load.Node.Url, load.Name)
	_, err := c.doRequest("POST", url, nil, 60*time.Second)
	return err
}

func (c *Client) DeprovisionLoad(load *common.Load) error {
	url := fmt.Sprintf("%s/loads/%s/deprovision", load.Node.Url, load.Name)
	_, err := c.doRequest("POST", url, nil, 60*time.Second)
	return err
}

func (c *Client) GetLoadsDeployments(nodeUrl string) (dto.LoadsDeployments, error) {
	url := fmt.Sprintf("%s/loads", nodeUrl)
	body, err := c.doRequest("GET", url, nil, 60*time.Second)
	if err != nil {
		return nil, err
	}

	var loadDeployments dto.LoadsDeployments
	if err := json.Unmarshal(body, &loadDeployments); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return loadDeployments, nil
}

func (c *Client) ReadLoadStdout(load *common.Load) error {
	url := fmt.Sprintf("%s/loads/%s/stdout", load.Node.Url, load.Name)
	resp, err := c.doStreamRequest("GET", url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to stream stdout: %v", err)
	}
	return nil
}

func (c *Client) ReadLoadStderr(load *common.Load) error {
	url := fmt.Sprintf("%s/loads/%s/stderr", load.Node.Url, load.Name)
	resp, err := c.doStreamRequest("GET", url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to stream stderr: %v", err)
	}
	return nil
}

func (c *Client) ProvisionNode(node *common.Node) error {
	url := fmt.Sprintf("%s/node/provision", node.Url)
	_, err := c.doJSONRequest("POST", url, node, 60*time.Second)
	return err
}

func (c *Client) DeprovisionNode(node *common.Node) error {
	url := fmt.Sprintf("%s/node/deprovision", node.Url)
	_, err := c.doRequest("POST", url, nil, 60*time.Second)
	return err
}

func (c *Client) StartNode(node *common.Node) error {
	url := fmt.Sprintf("%s/node/start", node.Url)
	_, err := c.doJSONRequest("POST", url, node, 60*time.Second)
	return err
}

func (c *Client) StopNode(node *common.Node, wallMessage string, offsetTime uint32, force bool) error {
	url := fmt.Sprintf("%s/node/stop", node.Url)
	request := dto.StopNodeRequest{WallMessage: wallMessage, Time: offsetTime, NodeName: &node.Name, Force: force}
	_, err := c.doJSONRequest("POST", url, request, 60*time.Second)
	return err
}

func (c *Client) RestartNode(node *common.Node, wallMessage string, offsetTime uint32) error {
	url := fmt.Sprintf("%s/node/restart", node.Url)
	request := dto.RestartNodeRequest{WallMessage: wallMessage, Time: offsetTime, NodeName: &node.Name}
	_, err := c.doJSONRequest("POST", url, request, 60*time.Second)
	return err
}
