package db

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"strconv"

	bolt "go.etcd.io/bbolt"

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

func getDBPath() string {
	dataPath := config.Get().DataPath
	if dataPath == "" {
		dataPath = "/var/lib/realm"
	}
	return filepath.Join(dataPath, "realm.db")
}

// Helper functions to build db keys
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

// Generic put operation
func (db *AgentDB) put(key, value string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Put([]byte(key), []byte(value))
	})
}

// putMulti writes several key/value pairs atomically in a single transaction
func (db *AgentDB) putMulti(kvs map[string]string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		for key, value := range kvs {
			if err := bucket.Put([]byte(key), []byte(value)); err != nil {
				return err
			}
		}
		return nil
	})
}

// Generic get operation
func (db *AgentDB) get(key string) (string, error) {
	var value []byte
	err := db.bolt.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketName).Get([]byte(key))
		if v == nil {
			return ErrKeyNotFound
		}
		// Copy: bbolt values are only valid inside the transaction
		value = append([]byte(nil), v...)
		return nil
	})
	if err != nil {
		return "", err
	}
	return string(value), nil
}

// Generic get with prefix
func (db *AgentDB) getPrefix(prefix string) (map[string]string, error) {
	result := make(map[string]string)
	err := db.bolt.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketName).Cursor()
		p := []byte(prefix)
		for k, v := c.Seek(p); k != nil && bytes.HasPrefix(k, p); k, v = c.Next() {
			result[string(k)] = string(v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Generic delete operation
func (db *AgentDB) delete(key string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Delete([]byte(key))
	})
}

// deleteKeys deletes several keys atomically in a single transaction
func (db *AgentDB) deleteKeys(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		for _, key := range keys {
			if err := bucket.Delete([]byte(key)); err != nil {
				return err
			}
		}
		return nil
	})
}

// deletePrefix deletes all keys with the given prefix, returning the number deleted
func (db *AgentDB) deletePrefix(prefix string) (int, error) {
	deleted := 0
	err := db.bolt.Update(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketName).Cursor()
		p := []byte(prefix)
		for k, _ := c.Seek(p); k != nil && bytes.HasPrefix(k, p); k, _ = c.Next() {
			if err := c.Delete(); err != nil {
				return err
			}
			deleted++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

// getNextSubnet returns the subnet assigned to a network, allocating the next
// one from the counter if the network doesn't have one yet
func (db *AgentDB) getNextSubnet(network string) (int32, error) {
	subnetKey, err := db.subnetKey(network)
	if err != nil {
		return 0, err
	}

	counterKey := "subnet_counter"

	var subnet int32
	err = db.bolt.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)

		// Network already has a subnet assigned
		if v := bucket.Get([]byte(subnetKey)); v != nil {
			parsed, err := strconv.Atoi(string(v))
			if err != nil {
				return err
			}
			subnet = int32(parsed)
			return nil
		}

		// Assign a new subnet from the counter
		var currentVal int64 = 0
		if v := bucket.Get([]byte(counterKey)); v != nil {
			currentVal, err = strconv.ParseInt(string(v), 10, 64)
			if err != nil {
				return err
			}
		}

		newVal := strconv.FormatInt(currentVal+1, 10)
		if err := bucket.Put([]byte(counterKey), []byte(newVal)); err != nil {
			return err
		}
		if err := bucket.Put([]byte(subnetKey), []byte(newVal)); err != nil {
			return err
		}
		subnet = int32(currentVal + 1)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return subnet, nil
}

// deleteSubnetOffset removes the subnet assignment for a network
func (db *AgentDB) deleteSubnetOffset(network string) error {
	subnetKey, err := db.subnetKey(network)
	if err != nil {
		return err
	}
	return db.delete(subnetKey)
}

// putIfNotExists atomically creates a key only if it doesn't already exist.
// Returns ErrKeyAlreadyExists if the key already exists.
func (db *AgentDB) putIfNotExists(key, value string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket.Get([]byte(key)) != nil {
			return ErrKeyAlreadyExists
		}
		return bucket.Put([]byte(key), []byte(value))
	})
}

// updateValue performs a read-modify-write update on a key in a single transaction.
// The updateFn receives the current value and should return the updated value.
// Returns an error if the key doesn't exist.
func (db *AgentDB) updateValue(key string, updateFn func(currentValue []byte) ([]byte, error)) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		currentValue := bucket.Get([]byte(key))
		if currentValue == nil {
			return fmt.Errorf("key '%s' not found", key)
		}

		newValue, err := updateFn(currentValue)
		if err != nil {
			return err
		}

		return bucket.Put([]byte(key), newValue)
	})
}
