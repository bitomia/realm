// Package logging wires third-party loggers into the agent's slog handler so
// that every log line the agent emits shares one destination and format.
package logging

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/containerd/log"
)

type slogHook struct{}

func (slogHook) Levels() []log.Level {
	return []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
		log.DebugLevel,
		log.TraceLevel,
	}
}

func (slogHook) Fire(entry *log.Entry) error {
	ctx := entry.Context
	if ctx == nil {
		ctx = context.Background()
	}

	logger := slog.Default()
	level := slogLevel(entry.Level)
	if !logger.Enabled(ctx, level) {
		return nil
	}

	attrs := make([]any, 0, (len(entry.Data)*2)+2)
	attrs = append(attrs, "component", "containerd")
	for key, value := range entry.Data {
		if err, ok := value.(error); ok {
			value = err.Error()
		}
		attrs = append(attrs, key, value)
	}

	logger.Log(ctx, level, entry.Message, attrs...)
	return nil
}

var bridgeOnce sync.Once

// BridgeContainerd routes containerd's logrus based output into the default
// slog logger at the given level. It must be called after slog.SetDefault, and
// is a no-op on subsequent calls.
func BridgeContainerd(level slog.Level) {
	bridgeOnce.Do(func() {
		logger := log.L.Logger
		// The hook is the only consumer now; silence logrus' own writer so
		// entries are not printed twice, in two different formats.
		logger.SetOutput(io.Discard)
		logger.AddHook(slogHook{})
	})

	log.L.Logger.SetLevel(logrusLevel(level))
}

func slogLevel(level log.Level) slog.Level {
	switch level {
	case log.TraceLevel, log.DebugLevel:
		return slog.LevelDebug
	case log.InfoLevel:
		return slog.LevelInfo
	case log.WarnLevel:
		return slog.LevelWarn
	default: // Error, Fatal and Panic
		return slog.LevelError
	}
}

func logrusLevel(level slog.Level) log.Level {
	switch {
	case level <= slog.LevelDebug:
		return log.DebugLevel
	case level <= slog.LevelInfo:
		return log.InfoLevel
	case level <= slog.LevelWarn:
		return log.WarnLevel
	default:
		return log.ErrorLevel
	}
}
