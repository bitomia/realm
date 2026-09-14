package logging_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/containerd/log"

	"github.com/bitomia/realm/common/logging"
)

func TestBridgeContainerdForwardsToSlog(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	logging.BridgeContainerd(slog.LevelDebug)

	log.G(context.Background()).
		WithField("error", errors.New("boom")).
		WithField("container", "web").
		Warn("pulling image failed")

	out := buf.String()
	for _, want := range []string{
		`level=WARN`,
		`msg="pulling image failed"`,
		`component=containerd`,
		`error=boom`,
		`container=web`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("slog output missing %q, got: %s", want, out)
		}
	}
}

func TestBridgeContainerdRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	logging.BridgeContainerd(slog.LevelWarn)
	t.Cleanup(func() { logging.BridgeContainerd(slog.LevelDebug) })

	log.L.Info("chatty containerd message")
	if buf.Len() != 0 {
		t.Errorf("expected info entry to be dropped at warn level, got: %s", buf.String())
	}

	log.L.Error("containerd failure")
	if !strings.Contains(buf.String(), "containerd failure") {
		t.Errorf("expected error entry to pass, got: %s", buf.String())
	}
}
