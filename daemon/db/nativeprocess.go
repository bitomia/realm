package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bitomia/realm/internal/loads/drivers"
)

func (db *DaemonDB) GetAllNativeProcesses() ([]drivers.ProcessConfig, error) {
	data, err := db.getKey(loadsPrefix)
	if err != nil {
		slog.Error("Error on GetAllNativeProcesses", "error", err.Error())
		return nil, err
	}

	var processes []drivers.ProcessConfig
	for _, value := range data {
		var process drivers.ProcessConfig
		if err := json.Unmarshal([]byte(value), &process); err != nil {
			slog.Error("Error unmarshaling native process", "error", err.Error())
			continue
		}
		processes = append(processes, process)
	}
	return processes, nil
}

func (db *DaemonDB) GetNativeProcess(processName string) (drivers.ProcessConfig, error) {
	if processName == "" {
		return drivers.ProcessConfig{}, errors.New("native process name cannot be empty")
	}

	value, err := db.get(db.loadsKey(processName))
	if err != nil {
		slog.Error("Error on GetProcess", "error", err.Error())
		return drivers.ProcessConfig{}, fmt.Errorf("Container %s not found", processName)
	}

	var process drivers.ProcessConfig
	if err := json.Unmarshal([]byte(value), &process); err != nil {
		slog.Error("Error unmarshaling native process", "error", err.Error())
		return drivers.ProcessConfig{}, err
	}
	return process, nil
}

func (db *DaemonDB) CreateLoadEntry(processName string, pid int, driver drivers.LoadDriver) error {
	value, err := json.Marshal(driver)
	if err != nil {
		slog.Error("Error marshaling native process", "error", err.Error())
		return err
	}

	err = db.put(db.loadsKey(processName), string(value))
	if err != nil {
		slog.Error("Error on CreateNativeProcess", "error", err.Error())
		return err
	}

	return nil
}

func (db *DaemonDB) DeleteNativeProcess(processName string) error {
	return db.delete(db.loadsKey(processName))
}
