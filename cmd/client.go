package main

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
	"github.com/bitomia/realm/internal/requests"
)

type Image struct {
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

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
		return fmt.Errorf("Request failed: %d", rep.StatusCode)
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

func (c *Client) GetAllImages() (map[string][]Image, error) {
	daemons := GetDaemonAddresses()
	imagesPerHost := make(map[string][]Image)

	for _, daemon := range daemons {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		url := fmt.Sprintf("%s/images", daemon.Url)
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

		if err := checkStatus(resp); err != nil {
			log.Error("Failed requesting image: %s %s", daemon.Url, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error("Failed to read response body: %v", err)
			continue
		}

		var images []Image
		if err := json.Unmarshal(body, &images); err != nil {
			log.Error("Failed to parse JSON: %v %s", err, body)
			continue
		}
		imagesPerHost[daemon.Name] = images
	}
	return imagesPerHost, nil
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
	daemons := GetDaemonAddresses()
	containersPerHost := make(map[string]map[string]Container)

	for _, daemon := range daemons {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		url := fmt.Sprintf("%s/containers", daemon.Url)
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
		containersPerHost[daemon.Name] = containers
	}
	return containersPerHost, nil
}

func (c *Client) CreateContainer(host string, name string, image string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s", host, name)

	request := requests.CreateContainerOpts{Image: image}
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

func (c *Client) StartContainer(host string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/state", host, container)

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

func (c *Client) StopContainer(host string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/state", host, container)

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

func (c *Client) DeleteContainer(host string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s", host, container)
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

func (c *Client) CreateNetwork(host string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	portMaps := []requests.PortmapOpts{{
		HostPort:      12000,
		ContainerPort: 80,
		Protocol:      "tcp",
	}}
	request := requests.StartNetworkOpts{Network: container, IPMasq: true, DNS: false, PortMap: portMaps}
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(request)

	url := fmt.Sprintf("%s/containers/%s/network", host, container)
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

func (c *Client) DeleteNetwork(host string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf("%s/containers/%s/network", host, container)
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

	daemons := GetDaemonAddresses()
	networksPerHost := make(map[string]any)

	for _, daemon := range daemons {
		url := fmt.Sprintf("%s/network", daemon.Url)
		req, err := http.NewRequest("GET", url, nil)
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

		networksPerHost[daemon.Name] = networkConfig
	}

	return networksPerHost, nil
}

type Stats struct {
	ContainerID   string  `json:"container_id"`
	CPUUsage      float64 `json:"cpu_usage"`
	CPUSystem     float64 `json:"cpu_system"`
	CPUUser       float64 `json:"cpu_user"`
	MemoryUsage   float64 `json:"mem_usage"`
	MemoryLimit   float64 `json:"mem_limit"`
	MemoryPercent float64 `json:"mem_percentage"`
}

type HostStatus struct {
	NumCPU          int     `json:"ncpu"`
	UserCPU         uint64  `json:"cpu_user"`
	IdleCPU         uint64  `json:"cpu_idle"`
	SystemCPU       uint64  `json:"cpu_system"`
	TotalCPU        uint64  `json:"cpu_total"`
	UsageCPUPercent float64 `json:"cpu_usage_percentage"`

	TotalMem       uint64  `json:"mem_total"`
	UsedMem        uint64  `json:"mem_used"`
	InactiveMem    uint64  `json:"mem_inactive"`
	CachedMem      uint64  `json:"mem_cached"`
	FreeMem        uint64  `json:"mem_free"`
	AvailableMem   uint64  `json:"mem_available"`
	FreeMemPercent float64 `json:"mem_free_percentage"`

	FreeStorage uint64 `json:"free_storage"`

	Containers []Stats `json:"containers"`
}

func (c *Client) GetHostStatus(host string) (HostStatus, error) {
	var status HostStatus

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/host", host)

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

	if err := checkStatus(resp); err != nil {
		return status, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read response body: %v", err)
	}

	if err := json.Unmarshal(body, &status); err != nil {
		log.Fatal("Failed to parse JSON: %v", err)
	}
	return status, nil
}

// Image operations
type PullImageRequest struct {
	Image string `json:"image"`
}

func (c *Client) PullImage(host string, image string) error {
	client := &http.Client{
		Timeout: 300 * time.Second, // Extended timeout for image pulls
	}
	url := fmt.Sprintf("%s/images", host)

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

func (c *Client) UpdateContainerQuotas(host string, container string, cpuQuota, memoryLimit, volumeSize int64) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/quotas", host, container)

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

func (c *Client) RepairContainer(host string, container string) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/repair", host, container)

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

func (c *Client) SendContainerSignal(host string, container string, signal string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/signal", host, container)

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

func (c *Client) MigrateContainer(host string, container string, newImage string) error {
	client := &http.Client{
		Timeout: 120 * time.Second, // Extended timeout for migration
	}
	url := fmt.Sprintf("%s/containers/%s/migrate", host, container)

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

func (c *Client) GetContainerLogs(host string, container string) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/logs", host, container)

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
func (c *Client) RepairNetwork(host string, container string) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	url := fmt.Sprintf("%s/network/%s/repair", host, container)

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
func (c *Client) GetProxyConfig(host string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/server", host, container)

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

func (c *Client) SetProxy(host string, container string, hosts []string, upstream string, httpUpstream, httpsUpstream bool) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/proxy", host, container)

	request := SetProxyRequest{
		Hosts:         hosts,
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

func (c *Client) DeleteProxy(host string, container string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/containers/%s/proxy", host, container)

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

func (c *Client) Login(host string, username string, password string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	url := fmt.Sprintf("%s/login", host)

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

// Recipe operations
func (c *Client) LaunchRecipe(host string, recipeData map[string]interface{}) error {
	client := &http.Client{
		Timeout: 120 * time.Second, // Extended timeout for recipe deployment
	}
	url := fmt.Sprintf("%s/recipes", host)

	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(recipeData)

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
		return fmt.Errorf("failed to launch recipe: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
}

func (c *Client) RecipeAction(host string, recipeId string, actionData map[string]interface{}) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	url := fmt.Sprintf("%s/recipes/%s", host, recipeId)

	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(actionData)

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
		return fmt.Errorf("failed to execute recipe action: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
}

func (c *Client) RollbackRecipe(host string, recipeId string) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	url := fmt.Sprintf("%s/recipes/%s", host, recipeId)

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
		return fmt.Errorf("failed to rollback recipe: %s", string(body))
	}

	fmt.Println(string(body))
	return nil
}
