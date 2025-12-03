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
	"github.com/bitomia/realm/internal/dto"
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

		var containers map[string]Container
		if err := json.Unmarshal(body, &containers); err != nil {
			log.Error("Failed to parse JSON: %v", err)
			continue
		}
		containersPerNode[node.Name] = containers
	}
	return containersPerNode, nil
}

func (c *Client) CreateContainer(node string, name string, image string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s", node, name)

	request := dto.CreateContainerRequest{Image: image}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return fmt.Errorf("Failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response body: %v", err)
	}

	if err := checkStatus(resp); err != nil {
		return errors.New(string(body))
	}

	return nil
}

type UpdateContainerState struct {
	State string `json:"state"`
}

func (c *Client) StartContainer(node string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/state", node, container)

	request := UpdateContainerState{"start"}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

	req, err := http.NewRequest("PUT", url, payload)
	if err != nil {
		return fmt.Errorf("Failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response body: %v", err)
	}

	if err := checkStatus(resp); err != nil {
		return errors.New(string(body))
	}

	return nil
}

func (c *Client) StopContainer(node string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/state", node, container)

	request := UpdateContainerState{"stop"}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)
	req, err := http.NewRequest("PUT", url, payload)
	if err != nil {
		return fmt.Errorf("Failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response body: %v", err)
	}

	if err := checkStatus(resp); err != nil {
		return errors.New(string(body))
	}

	return nil
}

func (c *Client) DeleteContainer(node string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s", node, container)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("Failed to create request: %v", err)
	}
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response body: %v", err)
	}

	if err := checkStatus(resp); err != nil {
		return errors.New(string(body))
	}
	return nil
}

func (c *Client) CreateNetwork(node string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	portMaps := []dto.Portmap{{
		HostPort:      12000,
		ContainerPort: 80,
		Protocol:      "tcp",
	}}
	request := dto.StartNetworkRequest{Network: container, IPMasq: true, DNS: false, PortMap: portMaps}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

	url := fmt.Sprintf("%s/containers/%s/network", node, container)
	req, err := http.NewRequest("POST", url, payload)
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response body: %v", err)
	}

	if err := checkStatus(resp); err != nil {
		return errors.New(string(body))
	}

	return nil
}

func (c *Client) DeleteNetwork(node string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf("%s/containers/%s/network", node, container)
	req, err := http.NewRequest("DELETE", url, nil)
	req.Header = c.header

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response body: %v", err)
	}

	if err := checkStatus(resp); err != nil {
		return errors.New(string(body))
	}

	return nil
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
	url := fmt.Sprintf("%s/node", node)

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

// Image operations
type PullImageRequest struct {
	Image string `json:"image"`
}

func (c *Client) PullImage(node string, image string) error {
	client := &http.Client{
		Timeout: 300 * time.Second, // Extended timeout for image pulls
	}
	url := fmt.Sprintf("%s/images", node)

	request := PullImageRequest{Image: image}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

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

	if err := checkStatus(resp); err != nil {
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to pull image: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
}

// Container operations
type UpdateQuotasRequest struct {
	CPUQuota    int64 `json:"cpu_quota,omitempty"`
	MemoryLimit int64 `json:"memory_limit,omitempty"`
	VolumeSize  int64 `json:"volume_size,omitempty"`
}

func (c *Client) UpdateContainerQuotas(node string, container string, cpuQuota, memoryLimit, volumeSize int64) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/quotas", node, container)

	request := UpdateQuotasRequest{
		CPUQuota:    cpuQuota,
		MemoryLimit: memoryLimit,
		VolumeSize:  volumeSize,
	}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

	req, err := http.NewRequest("PUT", url, payload)
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
		return fmt.Errorf("failed to update quotas: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
}

func (c *Client) RepairContainer(node string, container string) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/repair", node, container)

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
		return fmt.Errorf("failed to repair container: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
}

type SendSignalRequest struct {
	Signal string `json:"signal"`
}

func (c *Client) SendContainerSignal(node string, container string, signal string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/signal", node, container)

	request := SendSignalRequest{Signal: signal}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

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
		return fmt.Errorf("failed to send signal: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
}

type MigrateContainerRequest struct {
	NewImage string `json:"new_image"`
}

func (c *Client) MigrateContainer(node string, container string, newImage string) error {
	client := &http.Client{
		Timeout: 120 * time.Second, // Extended timeout for migration
	}
	url := fmt.Sprintf("%s/containers/%s/migrate", node, container)

	request := MigrateContainerRequest{NewImage: newImage}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

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
		return fmt.Errorf("failed to migrate container: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
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

// Network operations
func (c *Client) RepairNetwork(node string, container string) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	url := fmt.Sprintf("%s/network/%s/repair", node, container)

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
		return fmt.Errorf("failed to repair network: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
}

// Server/Proxy operations
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

type SetProxyRequest struct {
	Hosts         []string `json:"hosts"`
	Upstream      string   `json:"upstream"`
	HttpUpstream  bool     `json:"http_upstream"`
	HttpsUpstream bool     `json:"https_upstream"`
}

func (c *Client) SetProxy(node string, container string, nodes []string, upstream string, httpUpstream, httpsUpstream bool) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/proxy", node, container)

	request := SetProxyRequest{
		Hosts:         nodes,
		Upstream:      upstream,
		HttpUpstream:  httpUpstream,
		HttpsUpstream: httpsUpstream,
	}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

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
		return fmt.Errorf("failed to set proxy: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
}

func (c *Client) DeleteProxy(node string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/proxy", node, container)

	req, err := http.NewRequest("DELETE", url, nil)
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
		return fmt.Errorf("failed to delete proxy: %s", string(body))
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

func (c *Client) StartLoad(load *common.Load) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	url := fmt.Sprintf("%s/loads", load.Node.Url)

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
		return fmt.Errorf("%s", string(body))
	}

	return nil
}
