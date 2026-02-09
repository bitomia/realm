package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

type Client struct {
	header http.Header
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

func checkStatus(rep *http.Response) error {
	if rep == nil {
		return errors.New("nil request")
	}

	switch rep.StatusCode {
	case 200:
		return nil
	case 401:
		return errors.New("Unauthorized")
	case 400:
		return errors.New("Bad request")
	default:
		return fmt.Errorf("Request failed: %d %s", rep.StatusCode, rep.Body)
	}
}

func NewClient() Client {
	token := getAuthToken()
	if token == "" {
		return Client{
			http.Header{
				"ContentType": []string{"application/json"},
			},
		}
	}
	return Client{
		http.Header{
			"ContentType":   []string{"application/json"},
			"Authorization": []string{fmt.Sprintf("Bearer %s", token)},
		},
	}
}

func NewUnauthClient() Client {
	return Client{
		http.Header{
			"ContentType": []string{"application/json"},
		},
	}
}

func (c *Client) GetAllImages() (dto.NodeImagesMapResponse, error) {
	nodes := GetNodes()
	var nodeImagesMap dto.NodeImagesMapResponse

	for _, node := range nodes {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		url := fmt.Sprintf("%s/images", node.Url)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			nodeImagesMap = append(nodeImagesMap, dto.NodeImagesResponse{Node: node.Name, Error: err.Error()})
			continue
		}
		req.Header = c.header

		resp, err := client.Do(req)
		if err != nil {
			nodeImagesMap = append(nodeImagesMap, dto.NodeImagesResponse{Node: node.Name, Error: err.Error()})
			continue
		}
		defer resp.Body.Close()

		if err := checkStatus(resp); err != nil {
			nodeImagesMap = append(nodeImagesMap, dto.NodeImagesResponse{Node: node.Name, Error: err.Error()})
			continue
		}

		body, err := io.ReadAll(resp.Body)
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
	nodes := GetNodes()
	containersPerNode := make(map[string]map[string]Container)

	for _, node := range nodes {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		url := fmt.Sprintf("%s/containers", node.Url)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Error("Failed to create request: %v", err)
			continue

		}
		req.Header = c.header

		resp, err := client.Do(req)
		if err != nil {
			log.Error("Failed to make request: %v", err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error("Failed to read response body: %v", err)
			continue
		}

		if err := checkStatus(resp); err != nil {
			return nil, errors.New(string(body))
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
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	nodes := GetNodes()
	networksPerNode := make(map[string]any)

	for _, node := range nodes {
		url := fmt.Sprintf("%s/network", node.Url)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatal("Failed to create request: %v", err)
		}
		req.Header = c.header

		resp, err := client.Do(req)
		if err != nil {
			log.Fatal("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal("Failed to read response body: %v", err)
		}

		if err := checkStatus(resp); err != nil {
			return nil, errors.New(string(body))
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

func (c *Client) GetNodeState(node string) (*dto.NodeStateResponse, error) {
	var status dto.NodeStateResponse

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/state", node)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("Failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response body: %v", err)
	}

	if err := checkStatus(resp); err != nil {
		return &status, errors.New(string(body))
	}

	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("Failed to parse JSON: %v", err)
	}

	return &status, nil
}

func (c *Client) GetSystemInfo(node string) (*dto.SystemInfo, error) {
	var info dto.SystemInfo

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/system", node)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("Failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response body: %v", err)
	}

	if err := checkStatus(resp); err != nil {
		return &info, errors.New(string(body))
	}

	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("Failed to parse JSON: %v", err)
	}

	return &info, nil
}

func (c *Client) GetContainerLogs(node string, container string) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/logs", node, container)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get logs: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
}

func (c *Client) GetProxyConfig(node string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/server", node, container)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get proxy config: %s", string(body))
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

func (c *Client) PlanLoad(load *common.Load) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	url := fmt.Sprintf("%s/loads/plan", load.Node.Url)

	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(load)
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed planning load: %s", string(body))
	}

	return nil
}

func (c *Client) RunLoad(load *common.Load) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	url := fmt.Sprintf("%s/loads/%s/run", load.Node.Url, load.Name)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", string(body))
	}

	return nil
}

func (c *Client) StopLoad(load *common.Load) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	url := fmt.Sprintf("%s/loads/%s/stop", load.Node.Url, load.Name)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", string(body))
	}

	return nil
}

func (c *Client) KillLoad(load *common.Load) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	url := fmt.Sprintf("%s/loads/%s/kill", load.Node.Url, load.Name)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", string(body))
	}

	return nil
}

func (c *Client) UnplanLoad(load *common.Load) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	url := fmt.Sprintf("%s/loads/%s/unplan", load.Node.Url, load.Name)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", string(body))
	}

	return nil
}

func (c *Client) GetLoadsDeployments(nodeUrl string) (dto.LoadsDeployments, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	url := fmt.Sprintf("%s/loads", nodeUrl)

	req, err := http.NewRequest("GET", url, nil)
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

	var loadDeployments dto.LoadsDeployments
	if err := json.Unmarshal(body, &loadDeployments); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return loadDeployments, nil
}

func (c *Client) ReadLoadStdout(load *common.Load) error {
	client := &http.Client{
		Timeout: 0, // No timeout for streaming
	}

	url := fmt.Sprintf("%s/loads/%s/stdout", load.Node.Url, load.Name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read stdout: %s", string(body))
	}

	// Stream the response body to stdout
	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to stream stdout: %v", err)
	}

	return nil
}

func (c *Client) ReadLoadStderr(load *common.Load) error {
	client := &http.Client{
		Timeout: 0, // No timeout for streaming
	}

	url := fmt.Sprintf("%s/loads/%s/stderr", load.Node.Url, load.Name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read stderr: %s", string(body))
	}

	// Stream the response body to stdout
	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to stream stderr: %v", err)
	}

	return nil
}

func (c *Client) PlanNode(node *common.Node) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	url := fmt.Sprintf("%s/node/plan", node.Url)

	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(node)
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed planning node: %s", string(body))
	}

	return nil
}

func (c *Client) StartupNode(node *common.Node) error {
	return node.Driver.Startup()
}

func (c *Client) ShutdownNode(node *common.Node, wallMessage string, offsetTime uint32) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	request := dto.ShutdownNodeRequest{WallMessage: wallMessage, Time: offsetTime}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

	url := fmt.Sprintf("%s/node/shutdown", node.Url)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", string(body))
	}

	return nil
}

func (c *Client) RestartNode(node *common.Node, wallMessage string, offsetTime uint32) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	request := dto.RestartNodeRequest{WallMessage: wallMessage, Time: offsetTime}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

	url := fmt.Sprintf("%s/node/restart", node.Url)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", string(body))
	}

	return nil
}
