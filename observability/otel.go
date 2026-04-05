package observability

import (
	"context"
	"fmt"
	"log/slog"
)

// Config holds the OTel configuration for production.
type Config struct {
	Environment  string
	Endpoint     string
	ServiceName  string
	SamplerRatio float64
}

// Init bootstraps all three OTel signal pipelines (traces, metrics, logs)
// when running in production mode. In local/dev, Encore handles observability
// automatically, so this returns a no-op shutdown.
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	noop := func(context.Context) error { return nil }

	if cfg.Environment == "development" || cfg.Environment == "" {
		slog.Info("observability: skipping OTel init (Encore handles tracing in dev)")
		return noop, nil
	}

	slog.Info("observability: initializing OTel pipeline",
		slog.String("endpoint", cfg.Endpoint),
		slog.String("service", cfg.ServiceName),
		slog.Float64("sampler_ratio", cfg.SamplerRatio),
	)

	// TODO: Initialize TracerProvider with OTLP gRPC exporter
	// TODO: Initialize MeterProvider with OTLP gRPC exporter
	// TODO: Initialize LoggerProvider with OTLP gRPC exporter
	// TODO: Register global providers
	//
	// Production implementation will use:
	//   go.opentelemetry.io/otel/sdk/trace
	//   go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
	//   go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc

	return func(ctx context.Context) error {
		slog.Info("observability: shutting down OTel pipeline")
		// TODO: Flush and close all providers
		return nil
	}, fmt.Errorf("OTel production init: not yet fully implemented (add OTel SDK deps)")
}
