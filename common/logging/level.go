package logging

import (
	"log/slog"
	"os"
	"strings"
)

func ResolveLevel(configured string) (level slog.Level, name string, ok bool) {
	name = configured
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		name = envLevel
	}
	switch strings.ToLower(name) {
	case "debug":
		return slog.LevelDebug, name, true
	case "info", "":
		return slog.LevelInfo, name, true
	case "warn", "warning":
		return slog.LevelWarn, name, true
	case "error":
		return slog.LevelError, name, true
	default:
		return slog.LevelInfo, name, false
	}
}
