package otel

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"

	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

func newHTTPTracerProvider(ctx context.Context, res *resource.Resource) (*trace.TracerProvider, error) {
	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}

	// Change default values unless OTEL_BSP_SCHEDULE_DELAY is set
	var batchOptions []trace.BatchSpanProcessorOption
	if os.Getenv("OTEL_BSP_SCHEDULE_DELAY") == "" {
		batchOptions = append(batchOptions, trace.WithBatchTimeout(time.Second))
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(traceExporter, batchOptions...),
	)
	return tracerProvider, nil
}

func newHTTPMeterProvider(ctx context.Context, res *resource.Resource) (*metric.MeterProvider, error) {
	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}

	// Change default values unless OTEL_METRIC_EXPORT_INTERVAL is set
	var readerOptions []metric.PeriodicReaderOption
	if os.Getenv("OTEL_METRIC_EXPORT_INTERVAL") == "" {
		readerOptions = append(readerOptions, metric.WithInterval(3*time.Second))
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter, readerOptions...)),
	)
	return meterProvider, nil
}

func newHTTPLoggerProvider(ctx context.Context, res *resource.Resource) (*log.LoggerProvider, error) {
	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}
	loggerProvider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
	)

	return loggerProvider, nil
}
