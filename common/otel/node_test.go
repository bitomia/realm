package otel

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// recordingLogProcessor keeps the node name attribute of every emitted record
type recordingLogProcessor struct {
	names []string
}

func (p *recordingLogProcessor) Enabled(context.Context, log.EnabledParameters) bool { return true }

func (p *recordingLogProcessor) OnEmit(_ context.Context, record *log.Record) error {
	name := ""
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		if kv.Key == NodeNameKey {
			name = kv.Value.AsString()
		}
		return true
	})
	p.names = append(p.names, name)
	return nil
}

func (p *recordingLogProcessor) Shutdown(context.Context) error   { return nil }
func (p *recordingLogProcessor) ForceFlush(context.Context) error { return nil }

func TestNodeNameOnLogs(t *testing.T) {
	t.Cleanup(func() { SetNodeName("") })
	recorder := &recordingLogProcessor{}
	provider := log.NewLoggerProvider(
		log.WithProcessor(nodeLogProcessor{}),
		log.WithProcessor(recorder),
	)
	logger := provider.Logger("test")

	emit := func() {
		var r otellog.Record
		r.SetBody(attribute.StringValue("hello"))
		logger.Emit(context.Background(), r)
	}

	emit()
	SetNodeName("node1")
	emit()
	SetNodeName("")
	emit()

	want := []string{"", "node1", ""}
	if len(recorder.names) != len(want) {
		t.Fatalf("got %d records, want %d", len(recorder.names), len(want))
	}
	for i := range want {
		if recorder.names[i] != want[i] {
			t.Errorf("record %d: node name %q, want %q", i, recorder.names[i], want[i])
		}
	}
}

func TestNodeNameOnSpans(t *testing.T) {
	t.Cleanup(func() { SetNodeName("") })
	recorder := tracetest.NewSpanRecorder()
	provider := trace.NewTracerProvider(
		trace.WithSpanProcessor(nodeSpanProcessor{}),
		trace.WithSpanProcessor(recorder),
	)
	tracer := provider.Tracer("test")

	_, span := tracer.Start(context.Background(), "before")
	span.End()
	SetNodeName("node1")
	_, span = tracer.Start(context.Background(), "named")
	span.End()
	SetNodeName("")
	_, span = tracer.Start(context.Background(), "after")
	span.End()

	want := []string{"", "node1", ""}
	spans := recorder.Ended()
	if len(spans) != len(want) {
		t.Fatalf("got %d spans, want %d", len(spans), len(want))
	}
	for i, s := range spans {
		name := ""
		for _, kv := range s.Attributes() {
			if kv.Key == NodeNameKey {
				name = kv.Value.AsString()
			}
		}
		if name != want[i] {
			t.Errorf("span %q: node name %q, want %q", s.Name(), name, want[i])
		}
	}
}
