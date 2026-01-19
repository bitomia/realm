package db

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/daemon/id"
)

type EtcdNodesRepository struct {
	db *DaemonDB
}

type NodeValue struct {
	NodeName         string                  `json:"node_name"`
	NodeDriverConfig common.NodeDriverConfig `json:"node_driver_config"`
	Metadata         any                     `json:"metadata"`
}

func (r *EtcdNodesRepository) Create(nodeName string, driver common.NodeDriver, metadata any) error {
	nodeValue := NodeValue{
		NodeName:         nodeName,
		NodeDriverConfig: driver.GetDriverConfig(),
		Metadata:         metadata,
	}

	slog.Info("EtcdNodesRepository.Create", "nodeName", nodeName)

	nodeJson, err := json.Marshal(nodeValue)
	if err != nil {
		slog.Error("EtcdNodesRepository.Create", "nodeName", nodeName, "msg", "Marshalling node", "error", err.Error())
		return err
	}

	nodeKey, err := r.db.nodeKey()
	if err != nil {
		slog.Error("EtcdNodesRepository.Create", "nodeName", nodeName, "msg", "creating node key", "error", err.Error())
		return err
	}

	if err := r.db.put(nodeKey, string(nodeJson)); err != nil {
		slog.Error("EtcdNodesRepository.Create", "nodeName", nodeName, "msg", "db put", "error", err.Error())
		return err
	}

	return nil
}

func (r *EtcdNodesRepository) GetSelf() (common.NodeEntry, error) {
	if daemonId, err := id.GetDaemonId(); err != nil {
		return common.NodeEntry{}, err
	} else {
		return r.GetByDaemonId(daemonId)
	}
}

func (r *EtcdNodesRepository) GetByDaemonId(daemonId string) (common.NodeEntry, error) {
	nodeKey, err := r.db.nodeKeyByDaemonId(daemonId)
	if err != nil {
		slog.Error("EtcdNodesRepository.GetByDaemonId", "daemonId", daemonId, "msg", "nodeKey failed", "error", err.Error())
		return common.NodeEntry{}, err
	}

	nodeStr, err := r.db.get(nodeKey)
	if err != nil {
		slog.Error("EtcdNodesRepository.GetByDaemonId", "daemonId", daemonId, "msg", "getting node", "error", err.Error())
		return common.NodeEntry{}, err
	}

	var nodeValue NodeValue
	if err := json.Unmarshal([]byte(nodeStr), &nodeValue); err != nil {
		slog.Error("EtcdNodesRepository.GetByName", "daemonId", daemonId, "msg", "unmarshalling node", "error", err.Error())
		return common.NodeEntry{}, err
	}

	nodeDriver, err := common.BuildNodeDriver(nodeValue.NodeDriverConfig)
	if err != nil {
		slog.Error("EtcdNodesRepository.GetByName", "daemonId", daemonId, "msg", "building node driver", "error", err.Error())
		return common.NodeEntry{}, err
	}

	return common.NodeEntry{
		NodeName:   nodeValue.NodeName,
		NodeDriver: nodeDriver,
		Metadata:   nodeValue.Metadata,
	}, nil
}

func (r *EtcdNodesRepository) Delete() error {
	nodeKey, err := r.db.nodeKey()
	if err != nil {
		slog.Error("EtcdNodesRepository.Delete", "nodeKey", nodeKey, "msg", "nodeKey failed", "error", err.Error())
		return err
	}

	// Check if node exists first
	_, err = r.db.get(nodeKey)
	if err != nil {
		slog.Error("EtcdNodesRepository.Delete", "nodeKey", nodeKey, "msg", "node not found", "error", err.Error())
		return fmt.Errorf("node key '%s' not found", nodeKey)
	}

	if err := r.db.delete(nodeKey); err != nil {
		slog.Error("EtcdNodesRepository.Delete", "nodeKey", nodeKey, "msg", "deleting node", "error", err.Error())
		return err
	}

	return nil
}
