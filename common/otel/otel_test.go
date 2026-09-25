package otel_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/bitomia/realm/common/otel"
)

func TestInitializeHTTPExportsSignals(t *testing.T) {
	var mu sync.Mutex
	received := map[string]bool{}
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received[r.URL.Path] = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	if !otel.Enabled() {
		t.Fatal("expected OpenTelemetry to be enabled")
	}

	ctx := context.Background()
	shutdown, err := otel.InitializeHTTP(ctx, "agent-test")
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	slog.New(otel.NewSlogHandler("agent-test", slog.LevelInfo)).Info("hello")
	handler := otel.NewHTTPHandler(http.NotFoundHandler(), "test")
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	for _, path := range []string{"/v1/logs", "/v1/traces", "/v1/metrics"} {
		if !received[path] {
			t.Errorf("collector did not receive %s, got %v", path, received)
		}
	}
}

func TestEnabledRequiresEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if otel.Enabled() {
		t.Fatal("expected OpenTelemetry to be disabled without an endpoint")
	}
}
