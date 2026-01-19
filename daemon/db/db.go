package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"golang.org/x/crypto/bcrypt"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/daemon/id"
)

type DaemonDB struct {
	client                *clientv3.Client
	server                *embed.Etcd
	ctx                   context.Context
	DeploymentsRepository common.DeploymentsRepository
	NodesRepository       common.NodesRepository
}

var (
	instance *DaemonDB
	once     sync.Once
)

const ETCD_TIMEOUT = 5 * time.Second

func GetDB() *DaemonDB {
	once.Do(func() {
		daemonCfg := config.Get().Daemon
		etcdMode := daemonCfg.EtcdMode
		if etcdMode == "" {
			etcdMode = "server"
		}

		var client *clientv3.Client
		var server *embed.Etcd
		var err error
		var endpoints []string

		ctx := context.Background()

		switch etcdMode {
		case "server":
			// Server mode: start embedded etcd server
			cfg := getEtcdConfig()
			slog.Info("Initializing etcd server",
				"data_dir", cfg.Dir,
				"name", cfg.Name,
				"cluster_state", cfg.ClusterState,
				"initial_cluster", cfg.InitialCluster)

			// Start embedded etcd server
			server, err = embed.StartEtcd(cfg)
			if err != nil {
				slog.Error("Error starting etcd server", "error", err.Error())
				os.Exit(1)
			}

			// Wait for etcd to be ready
			select {
			case <-server.Server.ReadyNotify():
				slog.Info("Etcd server is ready")
			case <-time.After(60 * time.Second):
				server.Server.Stop()
				slog.Error("Etcd server took too long to start")
				os.Exit(1)
			}

			endpoints = []string{cfg.ListenClientUrls[0].String()}
		case "client":
			// Client mode: connect to external etcd
			endpoints = daemonCfg.EtcdEndpoints
			if len(endpoints) == 0 {
				slog.Error("Etcd mode is 'client' but no etcd_endpoints configured")
				os.Exit(1)
			}
			slog.Info("Connecting to external etcd", "endpoints", endpoints)
		default:
			slog.Error("Invalid etcd_mode", "mode", etcdMode)
			os.Exit(1)
		}

		// Create etcd client
		client, err = clientv3.New(clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: ETCD_TIMEOUT,
		})

		if err != nil {
			slog.Error("Error creating etcd client", "error", err.Error())
			if server != nil {
				server.Close()
			}
			os.Exit(1)
		}

		// Test connection
		ctxTimeout, cancel := context.WithTimeout(ctx, ETCD_TIMEOUT)
		defer cancel()
		status, err := client.Status(ctxTimeout, endpoints[0])
		if err != nil {
			slog.Error("Error connecting to etcd", "endpoints", endpoints, "error", err.Error())
			client.Close()
			if server != nil {
				server.Close()
			}
			os.Exit(1)
		}
		slog.Info("Etcd test connection done", "status", status)

		instance = &DaemonDB{
			client: client,
			server: server,
			ctx:    ctx,
		}
		instance.DeploymentsRepository = &EtcdDeploymentsRepository{instance}
		instance.NodesRepository = &EtcdNodesRepository{instance}

		if etcdMode == "server" {
			slog.Info("Database initialized with etcd server")
		} else {
			slog.Info("Database initialized with external etcd client")
		}
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

// PurgeDB deletes all data for the current daemon from the database
func (db *DaemonDB) PurgeDB() error {
	daemonId, err := id.GetDaemonId()
	if err != nil {
		slog.Error("Error getting daemon ID for purge", "error", err.Error())
		return err
	}

	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	// Get all keys with daemon ID prefix to count them before deletion
	getAllResp, err := db.client.Get(ctx, daemonId, clientv3.WithPrefix())
	if err != nil {
		slog.Error("Error getting keys for purge", "error", err.Error())
		return err
	}

	keysCount := len(getAllResp.Kvs)
	if keysCount == 0 {
		slog.Info("Database is already empty for this daemon, nothing to purge", "daemon_id", daemonId)
		return nil
	}

	slog.Warn("Purging all database contents for daemon", "daemon_id", daemonId)

	// Delete all keys for this daemon
	resp, err := db.client.Delete(ctx, daemonId, clientv3.WithPrefix())
	if err != nil {
		slog.Error("Error purging database", "error", err.Error())
		return err
	}

	slog.Info("Database purged successfully", "daemon_id", daemonId, "keys_deleted", resp.Deleted)
	return nil
}

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
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

// createIfNotExists creates a key only if it doesn't already exist.
// Returns an error if the key already exists or if there's an etcd error.
func (db *DaemonDB) createIfNotExists(key, value string) error {
	ctx, cancel := context.WithTimeout(db.ctx, ETCD_TIMEOUT)
	defer cancel()

	// Use transaction to check if key doesn't exist (CreateRevision == 0)
	txn := db.client.Txn(ctx)
	txn = txn.If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0))
	txn = txn.Then(clientv3.OpPut(key, value))

	tresp, err := txn.Commit()
	if err != nil {
		slog.Error("createIfNotExists: error committing transaction", "key", key, "error", err.Error())
		return err
	}

	if !tresp.Succeeded {
		return fmt.Errorf("key '%s' already exists", key)
	}

	return nil
}

// OptimisticUpdate performs an optimistic lock update on a key.
// The updateFn receives the current value and should return the updated value.
// Returns an error if the key doesn't exist or if there's an etcd error.
func (db *DaemonDB) OptimisticUpdate(key string, updateFn func(currentValue []byte) ([]byte, error)) error {
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
