package id

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/google/uuid"

	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/internal"
)

func GetDaemonId() (string, error) {
	dataPath := config.Get().Daemon.DataPath
	if dataPath == "" {
		return "", fmt.Errorf("Invalid data path")
	}
	idPath := filepath.Join(dataPath, "node.id")

	if internal.FileExists(idPath) {
		data, err := os.ReadFile(idPath)
		if err != nil {
			slog.Error("Error reading daemon ID", "error", err)
			debug.PrintStack()
			os.Exit(1)
		}
		return strings.TrimSpace(string(data)), nil
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		slog.Error("Error creating data directory", "error", err)
		debug.PrintStack()
		os.Exit(1)
	}

	newUUID := uuid.New().String()
	err := os.WriteFile(idPath, []byte(newUUID), 0644)
	if err != nil {
		slog.Error("Error writing daemon ID", "error", err)
		debug.PrintStack()
		os.Exit(1)
	}

	if err != nil {
		slog.Error("Error retrieving daemon ID", "error", err)
		debug.PrintStack()
		os.Exit(1)
	}

	return newUUID, nil
}
