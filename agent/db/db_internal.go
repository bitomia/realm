package db

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"

	"github.com/bitomia/realm/agent/config"
	"github.com/bitomia/realm/agent/id"
	"github.com/bitomia/realm/common"
)

const (
	containerPrefix   = "containers"
	networkPrefix     = "networks"
	subnetPrefix      = "subnets"
	userPrefix        = "users"
	dnsPrefix         = "dns"
	healthPrefix      = "health"
	deploymentsPrefix = "deployments"
	loadsPrefix       = "loads"
	nodePrefix        = "node"
	guestNodesPrefix  = "guest_nodes"
)

func getEtcdDataDir() string {
	dataPath := config.Get().DataPath
	if dataPath == "" {
		dataPath = "/var/lib/realm"
	}
	return filepath.Join(dataPath, "etcd")
}

func getEtcdConfig() *embed.Config {
	agentCfg := config.Get().Agent
	cfg := embed.NewConfig()

	// Basic configuration
	cfg.Dir = getEtcdDataDir()
	cfg.LogLevel = "error"

	// Set name
	agentId, err := id.GetAgentId()
	if err != nil {
		slog.Error("Error getting agent ID", "error", err.Error())
		agentId = "default-agent"
	}
	cfg.Name = agentId

	// Parse and set client URL
	clientUrl, err := url.Parse(agentCfg.EtcdListenClientUrl)
	if err != nil {
		slog.Error("Invalid etcd client URL", "url", agentCfg.EtcdListenClientUrl, "error", err.Error())
		clientUrl, _ = url.Parse("http://127.0.0.1:2379")
	}
	cfg.ListenClientUrls = []url.URL{*clientUrl}
	cfg.AdvertiseClientUrls = []url.URL{*clientUrl}

	// Parse and set peer URL
	peerUrl, err := url.Parse(agentCfg.EtcdListenPeerUrl)
	if err != nil {
		slog.Error("Invalid etcd peer URL", "url", agentCfg.EtcdListenPeerUrl, "error", err.Error())
		peerUrl, _ = url.Parse("http://127.0.0.1:2380")
	}
	cfg.ListenPeerUrls = []url.URL{*peerUrl}
	cfg.AdvertisePeerUrls = []url.URL{*peerUrl}

	// Set initial cluster configuration
	if agentCfg.EtcdInitialCluster != "" {
		cfg.InitialCluster = agentCfg.EtcdInitialCluster
	} else {
		// Single node cluster by default
		cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, peerUrl.String())
	}

	cfg.ClusterState = embed.ClusterStateFlagNew

	return cfg
}

// Helper functions to build etcd keys
func (db *AgentDB) containerKey(name string) (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, containerPrefix, name), nil
}

func (db *AgentDB) networkKey(container string) (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, networkPrefix, container), nil
}

func (db *AgentDB) subnetKey(network string) (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, subnetPrefix, network), nil
}

func (db *AgentDB) userKey(username string) (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, userPrefix, username), nil
}

func (db *AgentDB) dnsKey(dnsName string) (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, dnsPrefix, dnsName), nil
}

func (db *AgentDB) healthKey(hostname string) (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, healthPrefix, hostname), nil
}

func (db *AgentDB) deploymentKey(deploymentId common.DeploymentID) (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, deploymentsPrefix, deploymentId.String()), nil
}

func (db *AgentDB) loadDeploymentKey(loadName string, deploymentId common.DeploymentID) (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, loadsPrefix, loadName, deploymentId.String()), nil
}

func (db *AgentDB) loadKey(loadName string) (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, loadsPrefix, loadName), nil
}

func (db *AgentDB) deploymentsKeyPrefix() (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, deploymentsPrefix), nil
}

func (db *AgentDB) nodeKey() (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, nodePrefix), nil
}

func (db *AgentDB) guestNodeKey(guestNodeName string) (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, guestNodesPrefix, guestNodeName), nil
}

func (db *AgentDB) guestNodesKeyPrefix() (string, error) {
	agentId, err := id.GetAgentId()
	if err != nil {
		return "", err
	}
	return path.Join(agentId, guestNodesPrefix) + "/", nil
}

func (db *AgentDB) nodeKeyByAgentId(agentId string) (string, error) {
	return path.Join(agentId, nodePrefix), nil
}

func (db *AgentDB) txn(ops ...clientv3.Op) (*clientv3.TxnResponse, error) {
	if len(ops) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	txnRes, err := db.client.Txn(ctx).Then(ops...).Commit()
	if err != nil {
		slog.Error("Error on transaction", "ops", ops, "error", err.Error())
		return nil, err
	}

	return txnRes, nil
}

// Generic put operation
func (db *AgentDB) put(key, value string) error {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	_, err := db.client.Put(ctx, key, value)
	if err != nil {
		slog.Error("Error key put %s: %s", key, err.Error())
	}
	return err
}

// Generic get operation
func (db *AgentDB) get(key string) (string, error) {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	resp, err := db.client.Get(ctx, key)
	if err != nil {
		slog.Error("Error getting key %s: %s", key, err.Error())
		return "", err
	}

	if len(resp.Kvs) == 0 {
		return "", ErrKeyNotFound
	}

	return string(resp.Kvs[0].Value), nil
}

// Generic get with prefix
func (db *AgentDB) getKey(prefix string) (map[string]string, error) {
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
func (db *AgentDB) delete(key string) error {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	_, err := db.client.Delete(ctx, key)
	if err != nil {
		slog.Error("Error deleting key %s: %s", key, err.Error())
	}
	return err
}

// Atomic increment for subnet allocation
func (db *AgentDB) getNextSubnet(network string) (int32, error) {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	// Try to get existing subnet for the network
	subnetKey, err := db.subnetKey(network)
	if err != nil {
		return 0, err
	}
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

// deleteSubnetOffset removes the subnet assignment for a network
func (db *AgentDB) deleteSubnetOffset(network string) error {
	subnetKey, err := db.subnetKey(network)
	if err != nil {
		return err
	}
	return db.delete(subnetKey)
}

// putIfNotExists atomically creates a key only if it doesn't already exist,
// applying any extra put options (e.g. a lease). Returns ErrKeyAlreadyExists if
// the key already exists, or an etcd error.
func (db *AgentDB) putIfNotExists(key, value string, opts ...clientv3.OpOption) error {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	// Use transaction to check if key doesn't exist (CreateRevision == 0)
	txn := db.client.Txn(ctx)
	txn = txn.If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0))
	txn = txn.Then(clientv3.OpPut(key, value, opts...))

	tresp, err := txn.Commit()
	if err != nil {
		slog.Error("putIfNotExists: error committing transaction", "key", key, "error", err.Error())
		return err
	}

	if !tresp.Succeeded {
		return ErrKeyAlreadyExists
	}

	return nil
}

// optimisticUpdate performs an optimistic lock update on a key.
// The updateFn receives the current value and should return the updated value.
// Returns an error if the key doesn't exist or if there's an etcd error.
func (db *AgentDB) optimisticUpdate(key string, updateFn func(currentValue []byte) ([]byte, error)) error {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	// Retry loop for optimistic locking
	for {
		// Get current value and revision
		resp, err := db.client.Get(ctx, key)
		if err != nil {
			slog.Error("OptimisticUpdate: error getting key", "key", key, "error", err.Error())
			return err
		}

		if len(resp.Kvs) == 0 {
			return fmt.Errorf("key '%s' not found", key)
		}

		currentValue := resp.Kvs[0].Value
		currentRevision := resp.Kvs[0].ModRevision

		// Apply the update function
		newValue, err := updateFn(currentValue)
		if err != nil {
			return err
		}

		// Use transaction with compare-and-swap to ensure atomicity
		txn := db.client.Txn(ctx)
		txn = txn.If(clientv3.Compare(clientv3.ModRevision(key), "=", currentRevision))
		txn = txn.Then(clientv3.OpPut(key, string(newValue)))

		tresp, err := txn.Commit()
		if err != nil {
			slog.Error("OptimisticUpdate: error committing transaction", "key", key, "error", err.Error())
			return err
		}

		if tresp.Succeeded {
			return nil
		}

		// Transaction failed due to concurrent modification, retry
		slog.Debug("OptimisticUpdate: concurrent modification detected, retrying", "key", key)
		time.Sleep(10 * time.Millisecond)
	}
}

func (db *AgentDB) putWithLease(key, value string, leaseID clientv3.LeaseID) error {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	_, err := db.client.Put(ctx, key, value, clientv3.WithLease(leaseID))
	if err != nil {
		slog.Error("Error putting key %s with lease", "key", key, "error", err.Error())
	}
	return err
}
