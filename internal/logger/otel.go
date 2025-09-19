package logger

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// InitTracer initializes an OTLP HTTP exporter pointing to collectorURL (e.g. http://collector:4318)
// returns a shutdown func that should be called on application stop.
func (lg *Logger) InitTracer(ctx context.Context, collectorURL string) (func(context.Context) error, error) {
	if collectorURL == "" {
		return func(context.Context) error { return nil }, nil
	}
	// parse URL and extract host (endpoint) and insecure if scheme is http
	u, err := url.Parse(collectorURL)
	if err != nil {
		return nil, err
	}
	endpoint := u.Host
	insecure := false
	if u.Scheme == "http" {
		insecure = true
	}

	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	// allow some timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(lg.name),
		),
	)
	if err != nil {
		return nil, err
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tp)

	shutdown := func(ctx context.Context) error {
		// give exporter up to 5s to flush
		ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(ctx2)
	}
	return shutdown, nil
}

// StartSpan is a helper to start a span using the global tracer and returns ctx, span
func (lg *Logger) StartSpan(ctx context.Context, name string) (context.Context, func()) {
	tracer := otel.Tracer(lg.name)
	ctx2, span := tracer.Start(ctx, name)
	return ctx2, func() { span.End() }
}
