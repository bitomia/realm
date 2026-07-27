package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"

	"github.com/bitomia/realm/agent/id"
	"github.com/bitomia/realm/common"
)

type BoltNodesRepository struct {
	db *AgentDB
}

type NodeValue struct {
	NodeName         string                  `json:"node_name"`
	NodeDriverConfig common.NodeDriverConfig `json:"node_driver_config"`
	Metadata         any                     `json:"metadata"`
}

func (r *BoltNodesRepository) SetSelf(nodeName string, driver common.NodeDriver, metadata any) error {
	slog.Info("BoltNodesRepository.SetSelf", "nodeName", nodeName)

	nodeValue := NodeValue{
		NodeName:         nodeName,
		NodeDriverConfig: driver.Config(),
		Metadata:         metadata,
	}

	nodeJson, err := json.Marshal(nodeValue)
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

	var nodeValue NodeValue
	if err := json.Unmarshal([]byte(nodeStr), &nodeValue); err != nil {
		slog.Error("BoltNodesRepository.GetByAgentId", "agentId", agentId, "msg", "unmarshalling node", "error", err.Error())
		return common.NodeEntry{}, err
	}

	nodeDriver, err := common.BuildNodeDriver(common.NewNodeContext(nodeValue.NodeName), nodeValue.NodeDriverConfig)
	if err != nil {
		slog.Error("BoltNodesRepository.GetByAgentId", "agentId", agentId, "msg", "building node driver", "error", err.Error())
		return common.NodeEntry{}, err
	}

	return common.NodeEntry{
		NodeName:   nodeValue.NodeName,
		NodeDriver: nodeDriver,
		Metadata:   nodeValue.Metadata,
	}, nil
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

func (r *BoltNodesRepository) SetGuestNode(guestNodeName string, guestDriver common.NodeDriver, metadata any) error {
	slog.Info("BoltNodesRepository.SetGuestNode", "guestNodeName", guestNodeName)

	guestNodeValue := NodeValue{
		NodeName:         guestNodeName,
		NodeDriverConfig: guestDriver.Config(),
		Metadata:         metadata,
	}

	guestNodeJson, err := json.Marshal(guestNodeValue)
	if err != nil {
		slog.Error("BoltNodesRepository.SetGuestNode", "guestNodeName", guestNodeName, "msg", "Marshaling node", "error", err.Error())
		return err
	}

	guestNodeKey, err := r.db.guestNodeKey(guestNodeName)
	if err != nil {
		slog.Error("BoltNodesRepository.SetGuestNode", "guestNodeName", guestNodeName, "msg", "creating node key", "error", err.Error())
		return err
	}

	if err := r.db.putIfNotExists(guestNodeKey, string(guestNodeJson)); err != nil {
		slog.Error("BoltNodesRepository.SetGuestNode", "guestNodeName", guestNodeName, "msg", "db put", "error", err.Error())
		if errors.Is(err, ErrKeyAlreadyExists) {
			return common.ErrNodeAlreadyConfigured
		} else {
			return err
		}
	}

	return nil
}

func (r *BoltNodesRepository) DeleteGuestNode(guestNodeName string, guestDriver common.NodeDriver, metadata any) error {
	slog.Info("BoltNodesRepository.DeleteGuestNode", "guestNodeName", guestNodeName)

	guestNodeKey, err := r.db.guestNodeKey(guestNodeName)
	if err != nil {
		slog.Error("BoltNodesRepository.DeleteGuestNode", "nodeKey", guestNodeKey, "msg", "nodeKey failed", "error", err.Error())
		return err
	}

	// Check if node exists first
	_, err = r.db.get(guestNodeKey)
	if err != nil {
		slog.Error("BoltNodesRepository.DeleteGuestNode", "nodeKey", guestNodeKey, "msg", "node not found", "error", err.Error())
		return common.ErrNodeNotConfigured
	}

	if err := r.db.delete(guestNodeKey); err != nil {
		slog.Error("BoltNodesRepository.DeleteGuestNode", "nodeKey", guestNodeKey, "msg", "deleting node", "error", err.Error())
		return err
	}

	return nil
}

func (r *BoltNodesRepository) GetGuestNode(guestNodeName string) (common.NodeEntry, error) {
	slog.Info("BoltNodesRepository.GetGuestNode", "guestNodeName", guestNodeName)

	guestNodeKey, err := r.db.guestNodeKey(guestNodeName)
	if err != nil {
		slog.Error("BoltNodesRepository.GetGuestNode", "guestNodeName", guestNodeName, "msg", "nodeKey failed", "error", err.Error())
		return common.NodeEntry{}, err
	}

	nodeStr, err := r.db.get(guestNodeKey)
	if err != nil {
		slog.Error("BoltNodesRepository.GetGuestNode", "guestNodeName", guestNodeName, "msg", "getting node", "error", err.Error())
		return common.NodeEntry{}, err
	}

	var nodeValue NodeValue
	if err := json.Unmarshal([]byte(nodeStr), &nodeValue); err != nil {
		slog.Error("BoltNodesRepository.GetGuestNode", "guestNodeName", guestNodeName, "msg", "unmarshalling node", "error", err.Error())
		return common.NodeEntry{}, err
	}

	nodeDriver, err := common.BuildNodeDriver(common.NewNodeContext(nodeValue.NodeName), nodeValue.NodeDriverConfig)
	if err != nil {
		slog.Error("BoltNodesRepository.GetGuestNode", "guestNodeName", guestNodeName, "msg", "building node driver", "error", err.Error())
		return common.NodeEntry{}, err
	}

	return common.NodeEntry{
		NodeName:   nodeValue.NodeName,
		NodeDriver: nodeDriver,
		Metadata:   nodeValue.Metadata,
	}, nil
}

func (r *BoltNodesRepository) GetAllGuestNodes() ([]common.NodeEntry, error) {
	slog.Debug("BoltNodesRepository.GetAllGuestNodes")

	prefix, err := r.db.guestNodesKeyPrefix()
	if err != nil {
		return nil, fmt.Errorf("failed to build guest nodes prefix: %w", err)
	}

	entries, err := r.db.getPrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list guest nodes: %w", err)
	}

	var nodes []common.NodeEntry
	for key := range entries {
		guestName := path.Base(key)
		node, err := r.GetGuestNode(guestName)
		if err != nil {
			return nil, fmt.Errorf("failed to get guest node %s: %w", guestName, err)
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (r *BoltNodesRepository) UpdateSelfMetadata(updateFn func(metadataPtr any) error) error {
	nodeKey, err := r.db.nodeKey()
	if err != nil {
		slog.Error("BoltNodesRepository.UpdateSelfMetadata", "msg", "creating node key", "error", err.Error())
		return err
	}

	return r.db.updateValue(nodeKey, func(valueData []byte) ([]byte, error) {
		var value NodeValue
		if err := json.Unmarshal(valueData, &value); err != nil {
			return nil, err
		}
		if err := updateFn(&value.Metadata); err != nil {
			return nil, err
		}
		return json.Marshal(value)
	})
}

func (r *BoltNodesRepository) UpdateGuestMetadata(guestNodeName string, updateFn func(metadataPtr any) error) error {
	slog.Info("BoltNodesRepository.UpdateGuestMetadata", "guestNodeName", guestNodeName)

	guestNodeKey, err := r.db.guestNodeKey(guestNodeName)
	if err != nil {
		slog.Error("BoltNodesRepository.UpdateGuestMetadata", "guestNodeName", guestNodeName, "msg", "nodeKey failed", "error", err.Error())
		return err
	}

	return r.db.updateValue(guestNodeKey, func(valueData []byte) ([]byte, error) {
		var value NodeValue
		if err := json.Unmarshal(valueData, &value); err != nil {
			return nil, err
		}
		if err := updateFn(&value.Metadata); err != nil {
			return nil, err
		}
		return json.Marshal(value)
	})
}
