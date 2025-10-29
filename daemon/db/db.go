package db

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
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
