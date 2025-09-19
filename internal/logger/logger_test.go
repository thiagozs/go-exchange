package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
)

// create a dummy span context by creating a non-recording span
func TestInfofWithSpanAddsSpanTag(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Format: "text", Level: "debug", Out: &buf})

	// create a context with a non-recording span
	tracer := noop.NewTracerProvider().Tracer("test")
	ctx, span := tracer.Start(context.Background(), "span")
	// ensure span context has values (noop tracer returns invalid ids but ok for API usage)
	_ = span

	lg.WithContext(ctx).Infof("hello %s", "world")
	out := buf.String()
	if !strings.Contains(out, "hello world") {
		t.Fatalf("expected output to contain message, got: %s", out)
	}
	// when using noop tracer the span ids may be zero, but our logger still adds a [SPAN] prefix if span present
	if !strings.Contains(out, "[SPAN]") && !strings.Contains(out, "span_id") {
		t.Fatalf("expected span marker or span_id in log, got: %s", out)
	}
}
