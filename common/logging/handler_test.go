package logging_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/bitomia/realm/common/logging"
)

func TestSetBaseHandler(t *testing.T) {
	t.Cleanup(func() { logging.SetBaseHandler(nil) })

	if logging.BaseHandler() != nil {
		t.Fatal("expected no base handler by default")
	}

	h := slog.NewTextHandler(&bytes.Buffer{}, nil)
	logging.SetBaseHandler(h)
	if logging.BaseHandler() != h {
		t.Fatal("expected the base handler that was set")
	}

	logging.SetBaseHandler(nil)
	if logging.BaseHandler() != nil {
		t.Fatal("expected base handler to be cleared")
	}
}

func TestLevelHandlerFilters(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(logging.NewLevelHandler(slog.LevelWarn, inner)).With("component", "test")

	logger.Info("dropped")
	logger.Warn("kept")

	out := buf.String()
	if strings.Contains(out, "dropped") {
		t.Errorf("expected info record to be dropped, got: %s", out)
	}
	if !strings.Contains(out, "kept") || !strings.Contains(out, "component=test") {
		t.Errorf("expected warn record with attrs, got: %s", out)
	}
}
