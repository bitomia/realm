package id

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/bitomia/realm/internal/fs"
	"github.com/google/uuid"
)

func GetDaemonId() string {
	idFilePath := filepath.Join(fs.BinDir(), "realm.id")

	if fs.FileExists(idFilePath) {
		data, err := os.ReadFile(idFilePath)
		if err != nil {
			slog.Error("Error reading daemon ID", "error", err)
			debug.PrintStack()
			os.Exit(1)
		}
		return strings.TrimSpace(string(data))
	}

	newUUID := uuid.New().String()
	err := os.WriteFile(idFilePath, []byte(newUUID), 0644)
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

	return newUUID
}
