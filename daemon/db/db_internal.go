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

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/daemon/config"
	"github.com/bitomia/realm/daemon/id"
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
	daemonCfg := config.Get().Daemon
	cfg := embed.NewConfig()

	// Basic configuration
	cfg.Dir = getEtcdDataDir()
	cfg.LogLevel = "error"

	// Set name
	daemonId, err := id.GetDaemonId()
	if err != nil {
		slog.Error("Error getting daemon ID", "error", err.Error())
		daemonId = "default-daemon"
	}
	cfg.Name = daemonId

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

	cfg.ClusterState = embed.ClusterStateFlagNew

	return cfg
}

// Helper functions to build etcd keys
func (db *DaemonDB) containerKey(name string) (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, containerPrefix, name), nil
}

func (db *DaemonDB) networkKey(container string) (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, networkPrefix, container), nil
}

func (db *DaemonDB) subnetKey(network string) (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, subnetPrefix, network), nil
}

func (db *DaemonDB) userKey(username string) (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, userPrefix, username), nil
}

func (db *DaemonDB) dnsKey(dnsName string) (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, dnsPrefix, dnsName), nil
}

func (db *DaemonDB) healthKey(hostname string) (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, healthPrefix, hostname), nil
}

func (db *DaemonDB) deploymentKey(deploymentId common.DeploymentID) (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, deploymentsPrefix, deploymentId.String()), nil
}

func (db *DaemonDB) loadDeploymentKey(loadName string, deploymentId common.DeploymentID) (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, loadsPrefix, loadName, deploymentId.String()), nil
}

func (db *DaemonDB) loadKey(loadName string) (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, loadsPrefix, loadName), nil
}

func (db *DaemonDB) deploymentsKeyPrefix() (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, deploymentsPrefix), nil
}

func (db *DaemonDB) nodeKey() (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, nodePrefix), nil
}

func (db *DaemonDB) guestNodeKey(guestNodeName string) (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, guestNodesPrefix, guestNodeName), nil
}

func (db *DaemonDB) guestNodesKeyPrefix() (string, error) {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		return "", err
	}
	return path.Join(daemonId, guestNodesPrefix) + "/", nil
}

func (db *DaemonDB) nodeKeyByDaemonId(daemonId string) (string, error) {
	return path.Join(daemonId, nodePrefix), nil
}

func (db *DaemonDB) txn(ops ...clientv3.Op) (*clientv3.TxnResponse, error) {
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
func (db *DaemonDB) put(key, value string) error {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	_, err := db.client.Put(ctx, key, value)
	if err != nil {
		slog.Error("Error key put %s: %s", key, err.Error())
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

// DeleteSubnetOffset removes the subnet assignment for a network
func (db *DaemonDB) DeleteSubnetOffset(network string) error {
	subnetKey, err := db.subnetKey(network)
	if err != nil {
		return err
	}
	return db.delete(subnetKey)
}
