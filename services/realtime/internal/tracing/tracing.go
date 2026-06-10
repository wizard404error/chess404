package tracing

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Config struct {
	ServiceName string
	Environment string
	Exporter    string
}

func InitTracer(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "chess404"
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	switch cfg.Exporter {
	case "otlp":
	 exporter, err = newOTLPExporter(ctx)
	default:
		exporter, err = newStdoutExporter()
	}
	if err != nil {
		return nil, fmt.Errorf("create exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(parseSampleRate()))),
	)

	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

func newStdoutExporter() (sdktrace.SpanExporter, error) {
	return stdouttrace.New(stdouttrace.WithPrettyPrint())
}

func newOTLPExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	return newStdoutExporter()
}

func parseSampleRate() float64 {
	rate := os.Getenv("OTEL_SAMPLE_RATE")
	if rate == "" {
		return 1.0
	}
	var r float64
	fmt.Sscanf(rate, "%f", &r)
	if r <= 0 || r > 1 {
		return 1.0
	}
	return r
}
