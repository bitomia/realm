package db

import (
	"encoding/json"
	"log/slog"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type HealthStatus struct {
	Hostname  string         `json:"hostname"`
	Status    string         `json:"status"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func (db *DaemonDB) PublishHealthStatus(hostname string, leaseId clientv3.LeaseID, status string, metadata map[string]any) error {
	healthStatus := HealthStatus{
		Hostname:  hostname,
		Status:    status,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	value, err := json.Marshal(healthStatus)
	if err != nil {
		slog.Error("Error marshaling health status", "error", err.Error())
		return err
	}

	healthKey, err := db.healthKey(hostname)
	if err != nil {
		slog.Error("Error getting health key", "error", err.Error())
		return err
	}

	return db.PutWithLease(healthKey, string(value), leaseId)
}

func (db *DaemonDB) GetHealthStatus(hostname string) (HealthStatus, error) {
	healthKey, err := db.healthKey(hostname)
	if err != nil {
		return HealthStatus{}, err
	}

	value, err := db.get(healthKey)
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

func (db *DaemonDB) DeleteHealthStatus(hostname string) error {
	healthKey, err := db.healthKey(hostname)
	if err != nil {
		return err
	}
	return db.delete(healthKey)
}
