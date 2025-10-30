package db

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"

	"github.com/bitomia/realm/daemon/id"
	"github.com/bitomia/realm/internal/config"
)

const (
	containerPrefix = "/containers/"
	networkPrefix   = "/networks/"
	subnetPrefix    = "/subnets/"
	userPrefix      = "/users/"
	dnsPrefix       = "/dns/"
	healthPrefix    = "/health/"
	loadsPrefix     = "/loads/"
)

func getEtcdDataDir() string {
	dataDir := config.Get().Daemon.EtcdDataDir
	if dataDir == "" {
		dataDir = "/var/lib/realm/etcd"
	}
	return dataDir
}

func getEtcdConfig() *embed.Config {
	daemonCfg := config.Get().Daemon
	cfg := embed.NewConfig()

	// Basic configuration
	cfg.Dir = getEtcdDataDir()
	cfg.LogLevel = "error"

	// Set name - use daemon ID if not specified
	if daemonCfg.EtcdName != "" {
		cfg.Name = daemonCfg.EtcdName
	} else {
		cfg.Name = id.GetDaemonId()
	}

	// Parse and set client URL
	clientUrl, err := url.Parse(daemonCfg.EtcdListenClientUrl)
	if err != nil {
		slog.Error("Invalid etcd client URL", "url", daemonCfg.EtcdListenClientUrl, "error", err.Error())
		clientUrl, _ = url.Parse("http://127.0.0.1:2379")
	}
	cfg.ListenClientUrls = []url.URL{*clientUrl}
	cfg.AdvertiseClientUrls = []url.URL{*clientUrl}

	// Parse and set peer URL
	peerUrl, err := url.Parse(daemonCfg.EtcdListenPeerUrl)
	if err != nil {
		slog.Error("Invalid etcd peer URL", "url", daemonCfg.EtcdListenPeerUrl, "error", err.Error())
		peerUrl, _ = url.Parse("http://127.0.0.1:2380")
	}
	cfg.ListenPeerUrls = []url.URL{*peerUrl}
	cfg.AdvertisePeerUrls = []url.URL{*peerUrl}

	// Set initial cluster configuration
	if daemonCfg.EtcdInitialCluster != "" {
		cfg.InitialCluster = daemonCfg.EtcdInitialCluster
	} else {
		// Single node cluster by default
		cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, peerUrl.String())
	}

	// Set cluster state (new or existing)
	if daemonCfg.EtcdClusterState != "" {
		cfg.ClusterState = daemonCfg.EtcdClusterState
	} else {
		cfg.ClusterState = embed.ClusterStateFlagNew
	}

	return cfg
}

// Helper functions to build etcd keys
func (db *DaemonDB) containerKey(name string) string {
	return id.GetDaemonId() + containerPrefix + name
}

func (db *DaemonDB) networkKey(container string) string {
	return id.GetDaemonId() + networkPrefix + container
}

func (db *DaemonDB) subnetKey(network string) string {
	return id.GetDaemonId() + subnetPrefix + network
}

func (db *DaemonDB) userKey(username string) string {
	return id.GetDaemonId() + userPrefix + username
}

func (db *DaemonDB) dnsKey(dnsName string) string {
	return id.GetDaemonId() + dnsPrefix + dnsName
}

func (db *DaemonDB) healthKey(nodeId string) string {
	return id.GetDaemonId() + healthPrefix + nodeId
}

func (db *DaemonDB) loadsKey(name string) string {
	return id.GetDaemonId() + loadsPrefix + name
}

// Generic put operation
func (db *DaemonDB) put(key, value string) error {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	_, err := db.client.Put(ctx, key, value)
	if err != nil {
		slog.Error("Error putting key %s: %s", key, err.Error())
	}
	return err
}

// Generic get operation
func (db *DaemonDB) get(key string) (string, error) {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	resp, err := db.client.Get(ctx, key)
	if err != nil {
		slog.Error("Error getting key %s: %s", key, err.Error())
		return "", err
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("key %s not found", key)
	}

	return string(resp.Kvs[0].Value), nil
}

// Generic get with prefix
func (db *DaemonDB) getKey(prefix string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	resp, err := db.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		slog.Error("Error getting with prefix %s: %s", prefix, err.Error())
		return nil, err
	}

	result := make(map[string]string)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)] = string(kv.Value)
	}

	return result, nil
}

// Generic delete operation
func (db *DaemonDB) delete(key string) error {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	_, err := db.client.Delete(ctx, key)
	if err != nil {
		slog.Error("Error deleting key %s: %s", key, err.Error())
	}
	return err
}

// Atomic increment for subnet allocation
func (db *DaemonDB) getNextSubnet(network string) (int32, error) {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	// Try to get existing subnet for the network
	subnetKey := db.subnetKey(network)
	resp, err := db.client.Get(ctx, subnetKey)
	if err != nil {
		return 0, err
	}

	if len(resp.Kvs) > 0 {
		// Network already has a subnet assigned
		subnet, err := strconv.Atoi(string(resp.Kvs[0].Value))
		if err != nil {
			return 0, err
		}
		return int32(subnet), nil
	}

	// Need to assign a new subnet - use atomic transaction
	counterKey := "subnet_counter"

	for {
		// Get current counter
		resp, err := db.client.Get(ctx, counterKey)
		if err != nil {
			return 0, err
		}

		var currentVal int64 = 0
		var revision int64

		if len(resp.Kvs) > 0 {
			currentVal, err = strconv.ParseInt(string(resp.Kvs[0].Value), 10, 64)
			if err != nil {
				return 0, err
			}
			revision = resp.Kvs[0].ModRevision
		}

		newVal := currentVal + 1

		// Try to update counter and set subnet atomically
		txn := db.client.Txn(ctx)
		if len(resp.Kvs) == 0 {
			// Counter doesn't exist, create it
			txn = txn.If(clientv3.Compare(clientv3.CreateRevision(counterKey), "=", 0))
		} else {
			// Counter exists, check it hasn't changed
			txn = txn.If(clientv3.Compare(clientv3.ModRevision(counterKey), "=", revision))
		}

		txn = txn.Then(
			clientv3.OpPut(counterKey, strconv.FormatInt(newVal, 10)),
			clientv3.OpPut(subnetKey, strconv.FormatInt(newVal, 10)),
		)

		tresp, err := txn.Commit()
		if err != nil {
			return 0, err
		}

		if tresp.Succeeded {
			return int32(newVal), nil
		}

		// Transaction failed, retry
		time.Sleep(10 * time.Millisecond)
	}
}
