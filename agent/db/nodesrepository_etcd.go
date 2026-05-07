package db

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path"

	"github.com/bitomia/realm/agent/id"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/cloudinit"
)

type EtcdNodesRepository struct {
	db *AgentDB
}

type NodeValue struct {
	NodeName         string                  `json:"node_name"`
	NodeDriverConfig common.NodeDriverConfig `json:"node_driver_config"`
	CloudInit        *cloudinit.CloudInit    `json:"cloud_init,omitempty"`
	Metadata         any                     `json:"metadata"`
}

func (r *EtcdNodesRepository) SetSelf(nodeName string, driver common.NodeDriver, cloudInit *cloudinit.CloudInit, metadata any) error {
	slog.Info("EtcdNodesRepository.SetSelf", "nodeName", nodeName)

	nodeValue := NodeValue{
		NodeName:         nodeName,
		NodeDriverConfig: driver.GetDriverConfig(),
		CloudInit:        cloudInit,
		Metadata:         metadata,
	}

	nodeJson, err := json.Marshal(nodeValue)
	if err != nil {
		slog.Error("EtcdNodesRepository.SetSelf", "nodeName", nodeName, "msg", "Marshaling node", "error", err.Error())
		return err
	}

	nodeKey, err := r.db.nodeKey()
	if err != nil {
		slog.Error("EtcdNodesRepository.SetSelf", "nodeName", nodeName, "msg", "creating node key", "error", err.Error())
		return err
	}

	if err := r.db.put(nodeKey, string(nodeJson)); err != nil {
		slog.Error("EtcdNodesRepository.SetSelf", "nodeName", nodeName, "msg", "db put", "error", err.Error())
		return err
	}

	return nil
}

func (r *EtcdNodesRepository) GetSelf() (common.NodeEntry, error) {
	slog.Debug("EtcdNodesRepository.GetSelf")

	if agentId, err := id.GetAgentId(); err != nil {
		return common.NodeEntry{}, err
	} else {
		return r.GetByAgentId(agentId)
	}
}

func (r *EtcdNodesRepository) GetByAgentId(agentId string) (common.NodeEntry, error) {
	slog.Debug("EtcdNodesRepository.GetByAgentId", "agentId", agentId)

	nodeKey, err := r.db.nodeKeyByAgentId(agentId)
	if err != nil {
		slog.Error("EtcdNodesRepository.GetByAgentId", "agentId", agentId, "msg", "nodeKey failed", "error", err.Error())
		return common.NodeEntry{}, err
	}

	nodeStr, err := r.db.get(nodeKey)
	if err != nil {
		slog.Debug("EtcdNodesRepository.GetByAgentId", "agentId", agentId, "msg", "getting node", "error", err.Error())
		return common.NodeEntry{}, err
	}

	var nodeValue NodeValue
	if err := json.Unmarshal([]byte(nodeStr), &nodeValue); err != nil {
		slog.Error("EtcdNodesRepository.GetByAgentId", "agentId", agentId, "msg", "unmarshalling node", "error", err.Error())
		return common.NodeEntry{}, err
	}

	nodeDriver, err := common.BuildNodeDriver(nodeValue.NodeDriverConfig)
	if err != nil {
		slog.Error("EtcdNodesRepository.GetByAgentId", "agentId", agentId, "msg", "building node driver", "error", err.Error())
		return common.NodeEntry{}, err
	}

	return common.NodeEntry{
		NodeName:   nodeValue.NodeName,
		NodeDriver: nodeDriver,
		Metadata:   nodeValue.Metadata,
	}, nil
}

func (r *EtcdNodesRepository) DeleteSelf() error {
	slog.Info("EtcdNodesRepository.DeleteSelf")

	nodeKey, err := r.db.nodeKey()
	if err != nil {
		slog.Error("EtcdNodesRepository.DeleteSelf", "nodeKey", nodeKey, "msg", "nodeKey failed", "error", err.Error())
		return err
	}

	// Check if node exists first
	_, err = r.db.get(nodeKey)
	if err != nil {
		slog.Error("EtcdNodesRepository.DeleteSelf", "nodeKey", nodeKey, "msg", "node not found", "error", err.Error())
		return fmt.Errorf("node key '%s' not found", nodeKey)
	}

	if err := r.db.delete(nodeKey); err != nil {
		slog.Error("EtcdNodesRepository.DeleteSelf", "nodeKey", nodeKey, "msg", "deleting node", "error", err.Error())
		return err
	}

	return nil
}

func (r *EtcdNodesRepository) SetGuestNode(guestNodeName string, guestDriver common.NodeDriver, cloudInit *cloudinit.CloudInit, metadata any) error {
	slog.Info("EtcdNodesRepository.SetGuestNode", "guestNodeName", guestNodeName)

	guestNodeValue := NodeValue{
		NodeName:         guestNodeName,
		NodeDriverConfig: guestDriver.GetDriverConfig(),
		CloudInit:        cloudInit,
		Metadata:         metadata,
	}

	guestNodeJson, err := json.Marshal(guestNodeValue)
	if err != nil {
		slog.Error("EtcdNodesRepository.SetGuestNode", "guestNodeName", guestNodeName, "msg", "Marshaling node", "error", err.Error())
		return err
	}

	guestNodeKey, err := r.db.guestNodeKey(guestNodeName)
	if err != nil {
		slog.Error("EtcdNodesRepository.SetGuestNode", "guestNodeName", guestNodeName, "msg", "creating node key", "error", err.Error())
		return err
	}

	if err := r.db.put(guestNodeKey, string(guestNodeJson)); err != nil {
		slog.Error("EtcdNodesRepository.SetGuestNode", "guestNodeName", guestNodeName, "msg", "db put", "error", err.Error())
		return err
	}

	return nil
}

func (r *EtcdNodesRepository) DeleteGuestNode(guestNodeName string, guestDriver common.NodeDriver, metadata any) error {
	slog.Info("EtcdNodesRepository.DeleteGuestNode", "guestNodeName", guestNodeName)

	guestNodeKey, err := r.db.guestNodeKey(guestNodeName)
	if err != nil {
		slog.Error("EtcdNodesRepository.DeleteGuestNode", "nodeKey", guestNodeKey, "msg", "nodeKey failed", "error", err.Error())
		return err
	}

	// Check if node exists first
	_, err = r.db.get(guestNodeKey)
	if err != nil {
		slog.Error("EtcdNodesRepository.DeleteGuestNode", "nodeKey", guestNodeKey, "msg", "node not found", "error", err.Error())
		return fmt.Errorf("node key '%s' not found", guestNodeKey)
	}

	if err := r.db.delete(guestNodeKey); err != nil {
		slog.Error("EtcdNodesRepository.DeleteGuestNode", "nodeKey", guestNodeKey, "msg", "deleting node", "error", err.Error())
		return err
	}

	return nil
}

func (r *EtcdNodesRepository) GetGuestNode(guestNodeName string) (common.NodeEntry, error) {
	slog.Info("EtcdNodesRepository.GetGuestNode", "guestNodeName", guestNodeName)

	guestNodeKey, err := r.db.guestNodeKey(guestNodeName)
	if err != nil {
		slog.Error("EtcdNodesRepository.GetGuestNode", "guestNodeName", guestNodeName, "msg", "nodeKey failed", "error", err.Error())
		return common.NodeEntry{}, err
	}

	nodeStr, err := r.db.get(guestNodeKey)
	if err != nil {
		slog.Error("EtcdNodesRepository.GetGuestNode", "guestNodeName", guestNodeName, "msg", "getting node", "error", err.Error())
		return common.NodeEntry{}, err
	}

	var nodeValue NodeValue
	if err := json.Unmarshal([]byte(nodeStr), &nodeValue); err != nil {
		slog.Error("EtcdNodesRepository.GetGuestNode", "guestNodeName", guestNodeName, "msg", "unmarshalling node", "error", err.Error())
		return common.NodeEntry{}, err
	}

	nodeDriver, err := common.BuildNodeDriver(nodeValue.NodeDriverConfig)
	if err != nil {
		slog.Error("EtcdNodesRepository.GetGuestNode", "guestNodeName", guestNodeName, "msg", "building node driver", "error", err.Error())
		return common.NodeEntry{}, err
	}

	return common.NodeEntry{
		NodeName:   nodeValue.NodeName,
		NodeDriver: nodeDriver,
		Metadata:   nodeValue.Metadata,
	}, nil
}

func (r *EtcdNodesRepository) GetAllGuestNodes() ([]common.NodeEntry, error) {
	slog.Debug("EtcdNodesRepository.GetAllGuestNodes")

	prefix, err := r.db.guestNodesKeyPrefix()
	if err != nil {
		return nil, fmt.Errorf("failed to build guest nodes prefix: %w", err)
	}

	entries, err := r.db.getKey(prefix)
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

func (r *EtcdNodesRepository) UpdateSelfMetadata(updateFn func(metadataPtr any) error) error {
	nodeKey, err := r.db.nodeKey()
	if err != nil {
		slog.Error("EtcdNodesRepository.UpdateSelfMetadata", "msg", "creating node key", "error", err.Error())
		return err
	}

	return r.db.OptimisticUpdate(nodeKey, func(valueData []byte) ([]byte, error) {
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

func (r *EtcdNodesRepository) UpdateGuestMetadata(guestNodeName string, updateFn func(metadataPtr any) error) error {
	slog.Info("EtcdNodesRepository.UpdateGuestMetadata", "guestNodeName", guestNodeName)

	guestNodeKey, err := r.db.guestNodeKey(guestNodeName)
	if err != nil {
		slog.Error("EtcdNodesRepository.UpdateGuestMetadata", "guestNodeName", guestNodeName, "msg", "nodeKey failed", "error", err.Error())
		return err
	}

	return r.db.OptimisticUpdate(guestNodeKey, func(valueData []byte) ([]byte, error) {
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
