package db

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bitomia/realm/drivers/loads"
)

type EtcdLoadsRepository struct {
	db *DaemonDB
}

func (r *EtcdLoadsRepository) CreateLoad(loadName string, pid int, driver loads.LoadDriver) error {
	value, err := json.Marshal(driver)
	if err != nil {
		slog.Error("Error marshaling load", "error", err.Error())
		return err
	}

	loadsKey, err := r.db.loadsKey(loadName)
	if err != nil {
		slog.Error("Error getting loads key", "error", err.Error())
		return err
	}

	err = r.db.put(loadsKey, string(value))
	if err != nil {
		slog.Error("Error on CreateLoad", "error", err.Error())
		return err
	}

	return nil
}

func (r *EtcdLoadsRepository) DeleteLoad(loadName string) error {
	loadsKey, err := r.db.loadsKey(loadName)
	if err != nil {
		return err
	}
	return r.db.delete(loadsKey)
}

// GetActiveLoad retrieves the an active load record from the database.
//
// Behavior:
//   - If the database contains no data for the key, it returns (nil, nil).
//   - If multiple entries are found (which should not happen), it returns an error.
//   - If JSON unmarshaling fails, it logs the error and returns it.
//
// Returns:
//   - *loads.Load: Pointer to the active load data, or nil if no data exists.
//   - error: Non-nil if any error occurs during retrieval or unmarshaling.
func (r *EtcdLoadsRepository) GetLoad(loadName string) (*loads.Load, error) {
	loadsKey, err := r.db.loadsKey(loadName)
	if err != nil {
		slog.Error("Error getting loads key", "error", err.Error())
		return nil, err
	}

	data, err := r.db.getKey(loadsKey)
	if err != nil {
		slog.Error("Error on GetActiveLoad", "error", err.Error())
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}
	if len(data) > 1 {
		return nil, fmt.Errorf("Unexpected 'more than one load' condition on GetActiveLoad")
	}

	for _, v := range data {
		var currentLoad loads.Load
		if err := json.Unmarshal([]byte(v), &currentLoad); err != nil {
			slog.Error("Error unmarshaling load", "error", err.Error())
			return nil, err
		}
		return &currentLoad, nil
	}
	return nil, fmt.Errorf("Unreachable point on GetActiveLoad")
}
