package otel

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace"
)

// NodeNameKey is the attribute carrying the node name on exported logs and spans
const NodeNameKey = "realm.node.name"

var nodeName atomic.Pointer[string]

// SetNodeName sets the node name added to every log record and span from now
// on. The node name is only known once a node config is loaded so it can not
// be part of the resource. An empty name stops adding it
func SetNodeName(name string) {
	if name == "" {
		nodeName.Store(nil)
		return
	}
	nodeName.Store(&name)
}

func currentNodeName() string {
	if n := nodeName.Load(); n != nil {
		return *n
	}
	return ""
}

// nodeLogProcessor adds the node name to log records. It must be registered
// before the exporting processor so the exported record includes it
type nodeLogProcessor struct{}

// Enabled returns false as this processor does not export records by itself
func (nodeLogProcessor) Enabled(context.Context, log.EnabledParameters) bool { return false }

func (nodeLogProcessor) OnEmit(_ context.Context, record *log.Record) error {
	if n := currentNodeName(); n != "" {
		record.AddAttributes(attribute.String(NodeNameKey, n))
	}
	return nil
}

func (nodeLogProcessor) Shutdown(context.Context) error   { return nil }
func (nodeLogProcessor) ForceFlush(context.Context) error { return nil }

// nodeSpanProcessor adds the node name to spans when they start
type nodeSpanProcessor struct{}

func (nodeSpanProcessor) OnStart(_ context.Context, span trace.ReadWriteSpan) {
	if n := currentNodeName(); n != "" {
		span.SetAttributes(attribute.String(NodeNameKey, n))
	}
}

func (nodeSpanProcessor) OnEnd(trace.ReadOnlySpan)         {}
func (nodeSpanProcessor) Shutdown(context.Context) error   { return nil }
func (nodeSpanProcessor) ForceFlush(context.Context) error { return nil }
