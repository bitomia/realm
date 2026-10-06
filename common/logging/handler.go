package logging

import (
	"context"
	"log/slog"
	"sync/atomic"
)

var baseHandler atomic.Pointer[slog.Handler]

// SetBaseHandler makes the agent log through h instead of its own stdout
// handler. It is meant for embedders that route logs into the host application
// logger.
// Passing nil restores the default stdout handler.
func SetBaseHandler(h slog.Handler) {
	if h == nil {
		baseHandler.Store(nil)
		return
	}
	baseHandler.Store(&h)
}

// BaseHandler returns the handler set with SetBaseHandler, or nil
func BaseHandler() slog.Handler {
	if h := baseHandler.Load(); h != nil {
		return *h
	}
	return nil
}

// NewLevelHandler returns a handler that drops records below level before
// passing them to handler
func NewLevelHandler(level slog.Leveler, handler slog.Handler) slog.Handler {
	return &levelHandler{level: level, handler: handler}
}

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
