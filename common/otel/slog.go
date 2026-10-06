package otel

import (
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"

	"github.com/bitomia/realm/common/logging"
)

// NewSlogHandler returns a slog handler that forwards records at or above the
// given level to the global OpenTelemetry logger provider. The level filter is
// needed as the otelslog handler accepts every record the logger provider is
// enabled for
func NewSlogHandler(name string, level slog.Leveler) slog.Handler {
	return logging.NewLevelHandler(level, otelslog.NewHandler(name))
}
