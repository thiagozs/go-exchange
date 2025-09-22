package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
)

func TestJSONIncludesTraceAndSpan(t *testing.T) {
	var buf bytes.Buffer
	writer := io.MultiWriter(&buf, os.Stdout)
	lg := New(Options{Format: "json", Level: "debug", Out: writer})

	tracer := noop.NewTracerProvider().Tracer("test")
	ctx, span := tracer.Start(context.Background(), "span")
	_ = span

	lg.WithContext(ctx).Infof("hello")
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to unmarshal json log: %v", err)
	}
	// either span marker or trace_id/span_id
	if _, ok := out["span"]; !ok {
		if _, ok2 := out["trace_id"]; !ok2 {
			t.Fatalf("expected span or trace_id in json log, got: %v", out)
		}
	}
}
