package id

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	"github.com/google/uuid"

	"github.com/bitomia/realm/config"

	"github.com/bitomia/realm/internal"
)

func GetDaemonId() (string, error) {
	idPath := config.Get().Daemon.IdPath
	if idPath == "" {
		return "", fmt.Errorf("Invalid ID file path")
	}

	if internal.FileExists(idPath) {
		data, err := os.ReadFile(idPath)
		if err != nil {
			slog.Error("Error reading daemon ID", "error", err)
			debug.PrintStack()
			os.Exit(1)
		}
		return strings.TrimSpace(string(data)), nil
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
