package otel

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

// NewSlogHandler returns a slog handler that forwards records at or above the
// given level to the global OpenTelemetry logger provider
func NewSlogHandler(name string, level slog.Leveler) slog.Handler {
	return &levelHandler{level: level, handler: otelslog.NewHandler(name)}
}

// levelHandler filters out records below level, as the otelslog handler
// accepts every record the logger provider is enabled for
type levelHandler struct {
	level   slog.Leveler
	handler slog.Handler
}

func (h *levelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level.Level() && h.handler.Enabled(ctx, level)
}

func (h *levelHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.handler.Handle(ctx, record)
}

func (h *levelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelHandler{level: h.level, handler: h.handler.WithAttrs(attrs)}
}

func (h *levelHandler) WithGroup(name string) slog.Handler {
	return &levelHandler{level: h.level, handler: h.handler.WithGroup(name)}
}
