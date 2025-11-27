package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	loadsDriver "github.com/bitomia/realm/drivers/loads"
)

func (db *DaemonDB) GetAllNativeProcesses() ([]loadsDriver.ProcessConfig, error) {
	data, err := db.getKey(loadsPrefix)
	if err != nil {
		slog.Error("Error on GetAllNativeProcesses", "error", err.Error())
		return nil, err
	}

	var processes []loadsDriver.ProcessConfig
	for _, value := range data {
		var process loadsDriver.ProcessConfig
		if err := json.Unmarshal([]byte(value), &process); err != nil {
			slog.Error("Error unmarshaling native process", "error", err.Error())
			continue
		}
		processes = append(processes, process)
	}
	return processes, nil
}

func (db *DaemonDB) GetNativeProcess(processName string) (loadsDriver.ProcessConfig, error) {
	if processName == "" {
		return loadsDriver.ProcessConfig{}, errors.New("native process name cannot be empty")
	}

	loadsKey, err := db.loadsKey(processName)
	if err != nil {
		slog.Error("Error getting loads key", "error", err.Error())
		return loadsDriver.ProcessConfig{}, err
	}

	value, err := db.get(loadsKey)
	if err != nil {
		slog.Error("Error on GetProcess", "error", err.Error())
		return loadsDriver.ProcessConfig{}, fmt.Errorf("Container %s not found", processName)
	}

	var process loadsDriver.ProcessConfig
	if err := json.Unmarshal([]byte(value), &process); err != nil {
		slog.Error("Error unmarshaling native process", "error", err.Error())
		return loadsDriver.ProcessConfig{}, err
	}
	return process, nil
}
