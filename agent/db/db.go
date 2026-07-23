package db

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"

	"github.com/bitomia/realm/agent/id"
	"github.com/bitomia/realm/common"
)

var (
	ErrKeyNotFound      = errors.New("key not found")
	ErrKeyAlreadyExists = errors.New("key already exists")
)

type AgentDB struct {
	bolt                  *bolt.DB
	DeploymentsRepository common.DeploymentsRepository
	NodesRepository       common.NodesRepository
}

var (
	instance   *AgentDB
	once       sync.Once
	bucketName = []byte("agent")
)

const BOLT_TIMEOUT = 5 * time.Second

func GetDB() *AgentDB {
	once.Do(func() {
		dbPath := getDBPath()
		slog.Info("Initializing bbolt database", "path", dbPath)

		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			slog.Error("Error creating database directory", "path", filepath.Dir(dbPath), "error", err.Error())
			os.Exit(1)
		}

		database, err := bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: BOLT_TIMEOUT})
		if err != nil {
			slog.Error("Error opening bbolt database", "path", dbPath, "error", err.Error())
			os.Exit(1)
		}

		if err := database.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists(bucketName)
			return err
		}); err != nil {
			slog.Error("Error creating database bucket", "error", err.Error())
			database.Close()
			os.Exit(1)
		}

		instance = &AgentDB{
			bolt: database,
		}
		instance.DeploymentsRepository = &BoltDeploymentsRepository{instance}
		instance.NodesRepository = &BoltNodesRepository{instance}

		slog.Info("Database initialized with bbolt")
	})

	return instance
}

func (db *AgentDB) Close() {
	if db.bolt != nil {
		db.bolt.Close()
	}
}

// PurgeDB deletes all data for the current agent from the database
func (db *AgentDB) PurgeDB() error {
	agentId, err := id.GetAgentId()
	if err != nil {
		slog.Error("Error getting agent ID for purge", "error", err.Error())
		return err
	}

	deleted, err := db.deletePrefix(agentId)
	if err != nil {
		slog.Error("Error purging database", "error", err.Error())
		return err
	}

	if deleted == 0 {
		slog.Info("Database is already empty for this agent, nothing to purge", "agent_id", agentId)
		return nil
	}

	slog.Info("Database purged successfully", "agent_id", agentId, "keys_deleted", deleted)
	return nil
}

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}
