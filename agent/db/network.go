package db

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

type NetworkConfig struct {
	Network        string `json:"network"`
	Container      string `json:"container"`
	Config         string `json:"config"`
	CniResult      string `json:"cni_result"`
	GuestIfaceName string `json:"guest_ifname"`
	HostIfaceName  string `json:"host_ifname"`
}

// TODO replace usage with NetworkConfig type if possible
type NetConfig struct {
	Network        string `json:"network"`
	Config         string `json:"config"`
	CniResult      string `json:"cni_result"`
	GuestIfaceName string `json:"guest_ifname"`
	HostIfaceName  string `json:"host_ifname"`
}

func (db *AgentDB) NewOrRetrieveSubnetOffset(network string) (int32, error) {
	return db.getNextSubnet(network)
}

// We store guest and host ifnames because we are storing only bridge configs
func (db *AgentDB) AddNetConfig(network string, container string, config []byte, cniResult []byte, guest_ifname string, host_ifname string) error {
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
	networkKey, err := db.networkKey(container)
	if err != nil {
		slog.Error("Error getting network key", "error", err.Error())
		return err
	}
	key := fmt.Sprintf("%s%s", networkKey, network)
	err = db.put(key, string(value))
	if err != nil {
		slog.Error("Error on AddNetConfig", "error", err.Error())
		return err
	}
	return nil
}

func (db *AgentDB) IsHostIfaceUsedExceptForContainer(hostIface string, container string) (bool, error) {
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

func (db *AgentDB) GetNetConfigs(container string) ([]NetConfig, error) {
	// Get all network configs for this container
	containerNetPrefix, err := db.networkKey(container)
	if err != nil {
		slog.Error("Error getting network key", "error", err.Error())
		return nil, err
	}
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
			Network:        netConfig.Network,
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
func (db *AgentDB) DeleteAllNetConfigs(container string) error {
	containerNetPrefix, err := db.networkKey(container)
	if err != nil {
		slog.Error("Error getting network key", "error", err.Error())
		return err
	}
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

func (db *AgentDB) SetDNSRecord(key, ip string) error {
	dnsKey, err := db.dnsKey(key)
	if err != nil {
		return err
	}
	return db.put(dnsKey, ip)
}

func (db *AgentDB) GetDNSRecord(key string) (string, error) {
	dnsKey, err := db.dnsKey(key)
	if err != nil {
		return "", err
	}
	return db.get(dnsKey)
}

func (db *AgentDB) DeleteDNSRecord(key string) error {
	dnsKey, err := db.dnsKey(key)
	if err != nil {
		return err
	}
	return db.delete(dnsKey)
}

// ReleaseSubnet releases the subnet assignment for a network
func (db *AgentDB) ReleaseSubnet(network string) error {
	return db.deleteSubnetOffset(network)
}

// GetNetworkContainerCount returns the number of containers using a network
func (db *AgentDB) GetNetworkContainerCount(network string) (int, error) {
	data, err := db.getKey(networkPrefix)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, value := range data {
		var netConfig NetworkConfig
		if err := json.Unmarshal([]byte(value), &netConfig); err != nil {
			continue
		}
		if netConfig.Network == network {
			count++
		}
	}
	return count, nil
}
