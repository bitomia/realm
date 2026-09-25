package otel

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// NewHTTPHandler wraps handler to record a span and metrics for every request
func NewHTTPHandler(handler http.Handler, operation string) http.Handler {
	return otelhttp.NewHandler(handler, operation)
}

// RouteMiddleware names the request span after the matched mux route template
// (e.g. "GET /nodes/{name}"), keeping span names low cardinality
func RouteMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route := mux.CurrentRoute(r); route != nil {
			if tmpl, err := route.GetPathTemplate(); err == nil {
				span := trace.SpanFromContext(r.Context())
				span.SetName(r.Method + " " + tmpl)
				span.SetAttributes(semconv.HTTPRoute(tmpl))
				if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
					labeler.Add(semconv.HTTPRoute(tmpl))
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
