package otel

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestErrorHandlerRateLimitsRepeatedErrors(t *testing.T) {
	var buf bytes.Buffer
	h := newErrorHandler(slog.New(slog.NewTextHandler(&buf, nil)), time.Minute)
	now := time.Now()
	h.now = func() time.Time { return now }

	exportErr := errors.New("connection refused")
	for range 5 {
		h.Handle(exportErr)
	}
	h.Handle(errors.New("other failure"))

	if got := strings.Count(buf.String(), "connection refused"); got != 1 {
		t.Fatalf("expected repeated error logged once, got %d: %s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "other failure") {
		t.Fatalf("expected distinct error to be logged, got: %s", buf.String())
	}

	buf.Reset()
	now = now.Add(time.Minute)
	h.Handle(exportErr)
	if out := buf.String(); !strings.Contains(out, "level=WARN") || !strings.Contains(out, "suppressed=4") {
		t.Fatalf("expected warning with suppressed count after interval, got: %s", out)
	}
}
