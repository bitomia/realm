package otel

import (
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
)

const maxTrackedErrors = 64

func SetErrorLogger(logger *slog.Logger, interval time.Duration) {
	otel.SetErrorHandler(newErrorHandler(logger, interval))
}

type errorEntry struct {
	lastLogged time.Time
	suppressed int
}

type errorHandler struct {
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time

	mu     sync.Mutex
	errors map[string]*errorEntry
}

func newErrorHandler(logger *slog.Logger, interval time.Duration) *errorHandler {
	return &errorHandler{
		logger:   logger,
		interval: interval,
		now:      time.Now,
		errors:   map[string]*errorEntry{},
	}
}

func (h *errorHandler) Handle(err error) {
	msg := err.Error()
	now := h.now()

	h.mu.Lock()
	entry, ok := h.errors[msg]
	if ok && now.Sub(entry.lastLogged) < h.interval {
		entry.suppressed++
		h.mu.Unlock()
		return
	}
	if !ok {
		if len(h.errors) >= maxTrackedErrors {
			clear(h.errors)
		}
		entry = &errorEntry{}
		h.errors[msg] = entry
	}
	suppressed := entry.suppressed
	entry.lastLogged = now
	entry.suppressed = 0
	h.mu.Unlock()

	attrs := []any{"error", msg}
	if suppressed > 0 {
		attrs = append(attrs, "suppressed", suppressed)
	}
	h.logger.Warn("OpenTelemetry error", attrs...)
}
