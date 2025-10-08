package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"golang.org/x/crypto/bcrypt"
)

type DaemonDB struct {
	client *clientv3.Client
	server *embed.Etcd
	ctx    context.Context
}

var (
	instance *DaemonDB
	once     sync.Once
)

const ETCD_TIMEOUT = 5 * time.Second

func GetDB() *DaemonDB {
	once.Do(func() {
		cfg := getEtcdConfig()
		slog.Info("Initializing embedded etcd",
			"data_dir", cfg.Dir,
			"name", cfg.Name,
			"cluster_state", cfg.ClusterState,
			"initial_cluster", cfg.InitialCluster)

		// Start embedded etcd server
		e, err := embed.StartEtcd(cfg)
		if err != nil {
			slog.Error("Error starting embedded etcd", "error", err.Error())
			os.Exit(1)
		}

		// Wait for etcd to be ready
		select {
		case <-e.Server.ReadyNotify():
			slog.Info("Embedded etcd server is ready")
		case <-time.After(60 * time.Second):
			e.Server.Stop()
			slog.Error("Embedded etcd server took too long to start")
			os.Exit(1)
		}

		// Create client for the embedded etcd
		client, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{cfg.ListenClientUrls[0].String()},
			DialTimeout: ETCD_TIMEOUT,
		})

		if err != nil {
			slog.Error("Error creating etcd client", "error", err.Error())
			e.Close()
			os.Exit(1)
		}

		ctx := context.Background()
		// Test connection
		ctxTimeout, cancel := context.WithTimeout(ctx, ETCD_TIMEOUT)
		defer cancel()
		_, err = client.Status(ctxTimeout, cfg.ListenClientUrls[0].String())
		if err != nil {
			slog.Error("Error connecting to embedded etcd", "error", err.Error())
			client.Close()
			e.Close()
			os.Exit(1)
		}

		instance = &DaemonDB{
			client: client,
			server: e,
			ctx:    ctx,
		}
		slog.Info("Database initialized with embedded etcd")
	})

	return instance
}

func (db *DaemonDB) Close() {
	if db.client != nil {
		db.client.Close()
	}
	if db.server != nil {
		db.server.Close()
	}
}

type Container struct {
	ContainerName string `json:"container_name"`
	Image         string `json:"image"`
	LastStatus    string `json:"last_status"`
}

func (db *DaemonDB) GetAllContainers() ([]Container, error) {
	data, err := db.getKey(containerPrefix)
	if err != nil {
		slog.Error("Error on GetAllContainers", "error", err.Error())
		return nil, err
	}

	var containers []Container
	for _, value := range data {
		var container Container
		if err := json.Unmarshal([]byte(value), &container); err != nil {
			slog.Error("Error unmarshaling container", "error", err.Error())
			continue
		}
		containers = append(containers, container)
	}
	return containers, nil
}

func (db *DaemonDB) GetContainer(containerName string) (Container, error) {
	if containerName == "" {
		return Container{}, errors.New("container name cannot be empty")
	}

	value, err := db.get(db.containerKey(containerName))
	if err != nil {
		slog.Error("Error on GetContainer", "error", err.Error())
		return Container{}, fmt.Errorf("Container %s not found", containerName)
	}

	var container Container
	if err := json.Unmarshal([]byte(value), &container); err != nil {
		slog.Error("Error unmarshaling container", "error", err.Error())
		return Container{}, err
	}
	return container, nil
}

func (db *DaemonDB) CreateContainer(containerName string, image string, owner string, status string) (Container, error) {
	container := Container{
		ContainerName: containerName,
		Image:         image,
		LastStatus:    status,
	}

	value, err := json.Marshal(container)
	if err != nil {
		slog.Error("Error marshaling container", "error", err.Error())
		return Container{}, err
	}

	err = db.put(db.containerKey(containerName), string(value))
	if err != nil {
		slog.Error("Error on CreateContainer", "error", err.Error())
		return Container{}, err
	}

	return container, nil
}

func (db *DaemonDB) UpdateContainerStatus(containerName string, status string) (string, error) {
	slog.Info("db.UpdateContainerStatus", "container", containerName, "status", status)

	// Get existing container
	container, err := db.GetContainer(containerName)
	if err != nil {
		return "", err
	}

	// Update status
	container.LastStatus = status

	// Save back to etcd
	value, err := json.Marshal(container)
	if err != nil {
		slog.Error("Error marshaling container", "error", err.Error())
		return "", err
	}

	err = db.put(db.containerKey(containerName), string(value))
	if err != nil {
		slog.Error("Error on UpdateContainerStatus", "error", err.Error())
		return "", err
	}

	return status, nil
}

func (db *DaemonDB) UpdateContainerImage(containerName string, image string) (string, error) {
	slog.Info("db.UpdateContainerImage", "container", containerName, "image", image)

	// Get existing container
	container, err := db.GetContainer(containerName)
	if err != nil {
		return "", err
	}

	// Update image
	container.Image = image

	// Save back to etcd
	value, err := json.Marshal(container)
	if err != nil {
		slog.Error("Error marshaling container", "error", err.Error())
		return "", err
	}

	err = db.put(db.containerKey(containerName), string(value))
	if err != nil {
		slog.Error("Error on UpdateContainerImage", "error", err.Error())
		return "", err
	}

	return image, nil
}

func (db *DaemonDB) DeleteContainer(containerName string) error {
	return db.delete(db.containerKey(containerName))
}

func (db *DaemonDB) NewOrRetrieveSubnetOffset(network string) (int32, error) {
	return db.getNextSubnet(network)
}

type NetworkConfig struct {
	Network        string `json:"network"`
	Container      string `json:"container"`
	Config         string `json:"config"`
	CniResult      string `json:"cni_result"`
	GuestIfaceName string `json:"guest_ifname"`
	HostIfaceName  string `json:"host_ifname"`
}

// We store guest and host ifnames because we are storing only bridge configs
func (db *DaemonDB) AddNetConfig(network string, container string, config []byte, cniResult []byte, guest_ifname string, host_ifname string) error {
	netConfig := NetworkConfig{
		Network:        network,
		Container:      container,
		Config:         string(config),
		CniResult:      string(cniResult),
		GuestIfaceName: guest_ifname,
		HostIfaceName:  host_ifname,
	}

	value, err := json.Marshal(netConfig)
	if err != nil {
		slog.Error("Error marshaling network config", "error", err.Error())
		return err
	}

	// Use container as key since each container can have multiple network configs
	// We'll store them as container/network_name for uniqueness
	key := fmt.Sprintf("%s%s", db.networkKey(container), network)
	err = db.put(key, string(value))
	if err != nil {
		slog.Error("Error on AddNetConfig", "error", err.Error())
		return err
	}
	return nil
}

type NetConfig struct {
	Config         string `json:"config"`
	CniResult      string `json:"cni_result"`
	GuestIfaceName string `json:"guest_ifname"`
	HostIfaceName  string `json:"host_ifname"`
}

func (db *DaemonDB) IsHostIfaceUsedExceptForContainer(hostIface string, container string) (bool, error) {
	data, err := db.getKey(networkPrefix)
	if err != nil {
		return false, err
	}

	for _, value := range data {
		var netConfig NetworkConfig
		if err := json.Unmarshal([]byte(value), &netConfig); err != nil {
			slog.Error("Error unmarshaling network config", "error", err.Error())
			continue
		}
		// Check if host interface is used by a different container
		if netConfig.Container != container && netConfig.HostIfaceName == hostIface {
			return true, nil
		}
	}
	return false, nil
}

func (db *DaemonDB) GetNetConfigs(container string) ([]NetConfig, error) {
	// Get all network configs for this container
	containerNetPrefix := db.networkKey(container)
	data, err := db.getKey(containerNetPrefix)
	if err != nil {
		slog.Error("Error on GetNetConfigs", "error", err.Error())
		return nil, err
	}

	var cniConfigs []NetConfig
	for _, value := range data {
		var netConfig NetworkConfig
		if err := json.Unmarshal([]byte(value), &netConfig); err != nil {
			slog.Error("Error unmarshaling network config", "error", err.Error())
			continue
		}
		// Convert to NetConfig format
		config := NetConfig{
			Config:         netConfig.Config,
			CniResult:      netConfig.CniResult,
			GuestIfaceName: netConfig.GuestIfaceName,
			HostIfaceName:  netConfig.HostIfaceName,
		}
		cniConfigs = append(cniConfigs, config)
	}
	return cniConfigs, nil
}

// Delete all network configs for a container
func (db *DaemonDB) DeleteAllNetConfigs(container string) error {
	containerNetPrefix := db.networkKey(container)
	data, err := db.getKey(containerNetPrefix)
	if err != nil {
		return err
	}

	for key := range data {
		if err := db.delete(key); err != nil {
			slog.Error("Error deleting network config", "key", key, "error", err.Error())
			return err
		}
	}
	return nil
}

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     int32  `json:"role"`
}

func (db *DaemonDB) GetVerifiedUser(username string, password string) (int32, error) {
	slog.Info("Login request", "username", username)
	value, err := db.get(db.userKey(username))
	if err != nil {
		slog.Error("Error on GetVerifiedUser", "error", err.Error())
		return -1, nil // User not found
	}

	var user User
	if err := json.Unmarshal([]byte(value), &user); err != nil {
		slog.Error("Error unmarshaling user", "error", err.Error())
		return -1, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return -1, err
	}
	return user.Role, nil
}

func (db *DaemonDB) CreateUser(username string, password string, role int32) error {
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return err
	}

	user := User{
		Username: username,
		Password: hashedPassword,
		Role:     role,
	}

	value, err := json.Marshal(user)
	if err != nil {
		slog.Error("Error marshaling user", "error", err.Error())
		return err
	}

	return db.put(db.userKey(username), string(value))
}

func (db *DaemonDB) SetDNSRecord(key, ip string) error {
	return db.put(db.dnsKey(key), ip)
}

func (db *DaemonDB) GetDNSRecord(key string) (string, error) {
	return db.get(db.dnsKey(key))
}

func (db *DaemonDB) DeleteDNSRecord(key string) error {
	return db.delete(db.dnsKey(key))
}

type HealthStatus struct {
	NodeID    string                 `json:"node_id"`
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func (db *DaemonDB) PublishHealthStatus(nodeId string, leaseId clientv3.LeaseID, status string, metadata map[string]interface{}) error {
	healthStatus := HealthStatus{
		NodeID:    nodeId,
		Status:    status,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	value, err := json.Marshal(healthStatus)
	if err != nil {
		slog.Error("Error marshaling health status", "error", err.Error())
		return err
	}

	return db.PutWithLease(db.healthKey(nodeId), string(value), leaseId)
}

func (db *DaemonDB) GetHealthStatus(nodeId string) (HealthStatus, error) {
	value, err := db.get(db.healthKey(nodeId))
	if err != nil {
		return HealthStatus{}, err
	}

	var healthStatus HealthStatus
	if err := json.Unmarshal([]byte(value), &healthStatus); err != nil {
		slog.Error("Error unmarshaling health status", "error", err.Error())
		return HealthStatus{}, err
	}
	return healthStatus, nil
}

func (db *DaemonDB) GetAllHealthStatuses() ([]HealthStatus, error) {
	data, err := db.getKey(healthPrefix)
	if err != nil {
		slog.Error("Error on GetAllHealthStatuses", "error", err.Error())
		return nil, err
	}

	var healthStatuses []HealthStatus
	for _, value := range data {
		var healthStatus HealthStatus
		if err := json.Unmarshal([]byte(value), &healthStatus); err != nil {
			slog.Error("Error unmarshaling health status", "error", err.Error())
			continue
		}
		healthStatuses = append(healthStatuses, healthStatus)
	}
	return healthStatuses, nil
}

func (db *DaemonDB) DeleteHealthStatus(nodeId string) error {
	return db.delete(db.healthKey(nodeId))
}

func (db *DaemonDB) PutWithLease(key, value string, leaseID clientv3.LeaseID) error {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	_, err := db.client.Put(ctx, key, value, clientv3.WithLease(leaseID))
	if err != nil {
		slog.Error("Error putting key %s with lease", "key", key, "error", err.Error())
	}
	return err
}

func (db *DaemonDB) CreateLease(ttl int64) (clientv3.LeaseID, error) {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	resp, err := db.client.Grant(ctx, ttl)
	if err != nil {
		slog.Error("Error creating lease", "error", err.Error())
		return 0, err
	}
	return resp.ID, nil
}

func (db *DaemonDB) KeepAlive(leaseID clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	return db.client.KeepAlive(db.ctx, leaseID)
}
