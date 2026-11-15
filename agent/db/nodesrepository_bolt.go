package db

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/bitomia/realm/agent/id"
	"github.com/bitomia/realm/common"
)

type BoltNodesRepository struct {
	db *AgentDB
}

func (r *BoltNodesRepository) SetSelf(node common.Node) error {
	nodeName := node.Name
	slog.Info("BoltNodesRepository.SetSelf", "nodeName", nodeName, "registries", len(node.Registries))

	nodeJson, err := json.Marshal(common.NodeEntry{Node: node})
	if err != nil {
		slog.Error("BoltNodesRepository.SetSelf", "nodeName", nodeName, "msg", "Marshaling node", "error", err.Error())
		return err
	}

	nodeKey, err := r.db.nodeKey()
	if err != nil {
		slog.Error("BoltNodesRepository.SetSelf", "nodeName", nodeName, "msg", "creating node key", "error", err.Error())
		return err
	}

	if err := r.db.putIfNotExists(nodeKey, string(nodeJson)); err != nil {
		slog.Error("BoltNodesRepository.SetSelf", "nodeName", nodeName, "msg", "db put", "error", err.Error())
		if errors.Is(err, ErrKeyAlreadyExists) {
			return common.ErrNodeAlreadyConfigured
		} else {
			return err
		}
	}

	return nil
}

func (r *BoltNodesRepository) UpdateSelf(node common.Node) error {
	nodeName := node.Name
	slog.Info("BoltNodesRepository.UpdateSelf", "nodeName", nodeName)

	nodeKey, err := r.db.nodeKey()
	if err != nil {
		slog.Error("BoltNodesRepository.UpdateSelf", "nodeName", nodeName, "msg", "creating node key", "error", err.Error())
		return err
	}

	return r.updateNode(nodeKey, node)
}

// updateNode replaces the node stored on key, keeping its metadata
func (r *BoltNodesRepository) updateNode(key string, node common.Node) error {
	if _, err := r.db.get(key); err != nil {
		return common.ErrNodeNotConfigured
	}

	return r.db.updateValue(key, func(valueData []byte) ([]byte, error) {
		// Only the metadata is decoded so the stored node driver is not rebuilt
		var stored struct {
			Metadata json.RawMessage `json:"metadata"`
		}
		if err := json.Unmarshal(valueData, &stored); err != nil {
			return nil, err
		}
		entry := common.NodeEntry{Node: node}
		if len(stored.Metadata) > 0 {
			entry.Metadata = stored.Metadata
		}
		return json.Marshal(entry)
	})
}

func (r *BoltNodesRepository) GetSelf() (common.NodeEntry, error) {
	slog.Debug("BoltNodesRepository.GetSelf")

	if agentId, err := id.GetAgentId(); err != nil {
		return common.NodeEntry{}, err
	} else {
		return r.GetByAgentId(agentId)
	}
}

func (r *BoltNodesRepository) GetByAgentId(agentId string) (common.NodeEntry, error) {
	slog.Debug("BoltNodesRepository.GetByAgentId", "agentId", agentId)

	nodeKey, err := r.db.nodeKeyByAgentId(agentId)
	if err != nil {
		slog.Error("BoltNodesRepository.GetByAgentId", "agentId", agentId, "msg", "nodeKey failed", "error", err.Error())
		return common.NodeEntry{}, err
	}

	nodeStr, err := r.db.get(nodeKey)
	if err != nil {
		slog.Debug("BoltNodesRepository.GetByAgentId", "agentId", agentId, "msg", "getting node", "error", err.Error())
		if errors.Is(err, ErrKeyNotFound) {
			return common.NodeEntry{}, common.ErrNodeNotConfigured
		} else {
			return common.NodeEntry{}, err
		}
	}

	var nodeEntry common.NodeEntry
	if err := json.Unmarshal([]byte(nodeStr), &nodeEntry); err != nil {
		slog.Error("BoltNodesRepository.GetByAgentId", "agentId", agentId, "msg", "unmarshalling node", "error", err.Error())
		return common.NodeEntry{}, err
	}

	return nodeEntry, nil
}

func (r *BoltNodesRepository) DeleteSelf() error {
	slog.Info("BoltNodesRepository.DeleteSelf")

	nodeKey, err := r.db.nodeKey()
	if err != nil {
		slog.Error("BoltNodesRepository.DeleteSelf", "nodeKey", nodeKey, "msg", "nodeKey failed", "error", err.Error())
		return err
	}

	// Check if node exists first
	_, err = r.db.get(nodeKey)
	if err != nil {
		slog.Error("BoltNodesRepository.DeleteSelf", "nodeKey", nodeKey, "msg", "node not found", "error", err.Error())
		return common.ErrNodeNotConfigured
	}

	if err := r.db.delete(nodeKey); err != nil {
		slog.Error("BoltNodesRepository.DeleteSelf", "nodeKey", nodeKey, "msg", "deleting node", "error", err.Error())
		return err
	}

	return nil
}

func (r *BoltNodesRepository) UpdateSelfMetadata(updateFn func(metadataPtr any) error) error {
	nodeKey, err := r.db.nodeKey()
	if err != nil {
		slog.Error("BoltNodesRepository.UpdateSelfMetadata", "msg", "creating node key", "error", err.Error())
		return err
	}

	return r.db.updateValue(nodeKey, func(valueData []byte) ([]byte, error) {
		var value common.NodeEntry
		if err := json.Unmarshal(valueData, &value); err != nil {
			return nil, err
		}
		if err := updateFn(&value.Metadata); err != nil {
			return nil, err
		}
		return json.Marshal(value)
	})
}
