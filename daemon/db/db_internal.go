package db

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"go.etcd.io/etcd/client/v3"

	"github.com/bitomia/realm/internal/config"
)

const (
	containerPrefix = "containers/"
	networkPrefix   = "networks/"
	subnetPrefix    = "subnets/"
	userPrefix      = "users/"
	dnsPrefix       = "dns/"
	healthPrefix    = "health/"
)

func getEtcdEndpoints() []string {
	return config.Get().Daemon.EtcdEndpoints
}

// Helper functions to build etcd keys
func (db *DaemonDB) containerKey(name string) string {
	return containerPrefix + name
}

func (db *DaemonDB) networkKey(container string) string {
	return networkPrefix + container
}

func (db *DaemonDB) subnetKey(network string) string {
	return subnetPrefix + network
}

func (db *DaemonDB) userKey(username string) string {
	return userPrefix + username
}

func (db *DaemonDB) dnsKey(dnsName string) string {
	return dnsPrefix + dnsName
}

func (db *DaemonDB) healthKey(nodeId string) string {
	return healthPrefix + nodeId
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
