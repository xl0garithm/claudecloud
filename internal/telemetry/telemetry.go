package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Init bootstraps OpenTelemetry tracing and metrics.
// If otlpEndpoint is empty, traces are discarded (noop) in dev mode.
// Returns a shutdown function that flushes and shuts down providers.
func Init(ctx context.Context, serviceName, version, environment, otlpEndpoint string) (shutdown func(context.Context) error, err error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", version),
			attribute.String("deployment.environment.name", environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	// Trace provider
	var spanExporter sdktrace.SpanExporter
	if otlpEndpoint != "" {
		spanExporter, err = otlptracehttp.New(ctx,
			otlptracehttp.WithEndpointURL(otlpEndpoint),
		)
		if err != nil {
			return nil, fmt.Errorf("create OTLP exporter: %w", err)
		}
	}

	var tpOpts []sdktrace.TracerProviderOption
	tpOpts = append(tpOpts, sdktrace.WithResource(res))
	if spanExporter != nil {
		tpOpts = append(tpOpts, sdktrace.WithBatcher(spanExporter))
	}

	tp := sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(tp)

	// Propagator: W3C TraceContext + Baggage
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Metrics provider: Prometheus exporter
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)
	otel.SetMeterProvider(mp)

	shutdown = func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("trace provider shutdown: %w", err)
		}
		if err := mp.Shutdown(ctx); err != nil {
			return fmt.Errorf("meter provider shutdown: %w", err)
		}
		return nil
	}

	return shutdown, nil
}
