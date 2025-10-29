package db

import (
	"encoding/json"
	"log/slog"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type HealthStatus struct {
	NodeID    string                 `json:"node_id"`
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func (db *DaemonDB) PublishHealthStatus(nodeId string, leaseId clientv3.LeaseID, status string, metadata map[string]interface{}) error {
	healthStatus := HealthStatus{
		NodeID:    nodeId,
		Status:    status,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	value, err := json.Marshal(healthStatus)
	if err != nil {
		slog.Error("Error marshaling health status", "error", err.Error())
		return err
	}

	return db.PutWithLease(db.healthKey(nodeId), string(value), leaseId)
}

func (db *DaemonDB) GetHealthStatus(nodeId string) (HealthStatus, error) {
	value, err := db.get(db.healthKey(nodeId))
	if err != nil {
		return HealthStatus{}, err
	}

	var healthStatus HealthStatus
	if err := json.Unmarshal([]byte(value), &healthStatus); err != nil {
		slog.Error("Error unmarshaling health status", "error", err.Error())
		return HealthStatus{}, err
	}
	return healthStatus, nil
}

func (db *DaemonDB) GetAllHealthStatuses() ([]HealthStatus, error) {
	data, err := db.getKey(healthPrefix)
	if err != nil {
		slog.Error("Error on GetAllHealthStatuses", "error", err.Error())
		return nil, err
	}

	var healthStatuses []HealthStatus
	for _, value := range data {
		var healthStatus HealthStatus
		if err := json.Unmarshal([]byte(value), &healthStatus); err != nil {
			slog.Error("Error unmarshaling health status", "error", err.Error())
			continue
		}
		healthStatuses = append(healthStatuses, healthStatus)
	}
	return healthStatuses, nil
}

func (db *DaemonDB) DeleteHealthStatus(nodeId string) error {
	return db.delete(db.healthKey(nodeId))
}
