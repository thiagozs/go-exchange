package logger

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// testExporter is a minimal span exporter that records ended spans and their events
type testExporter struct {
	// store events per span name
	events map[string][]sdktrace.Event
}

func (t *testExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if t.events == nil {
		t.events = map[string][]sdktrace.Event{}
	}
	for _, s := range spans {
		t.events[s.Name()] = append(t.events[s.Name()], s.Events()...)
	}
	return nil
}

func (t *testExporter) Shutdown(ctx context.Context) error { return nil }

func TestOtelLogHookAddsEvent(t *testing.T) {
	// setup tracer provider with exporter and always sample so spans are recording
	exp := &testExporter{}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(resource.NewWithAttributes("", attribute.String("test", "otelhook"))),
		sdktrace.WithSyncer(exp),
	)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// create a logger and register telemetry hook
	l := New(Options{Format: "text", Level: "debug", Out: &bytes.Buffer{}})
	// point logger tracer to our test provider
	l.tracer = tp.Tracer("test")
	if err := l.SetupTelemetry(context.Background(), nil); err != nil {
		t.Fatalf("setup telemetry: %v", err)
	}

	// create a context with a recording span
	ctx, span := l.Span(context.Background(), "test-span")

	// create a logrus entry using logger's WithContext
	entry := logrus.NewEntry(l.logrus).WithContext(ctx)
	entry.Info("hello test")

	// end span so exporter is invoked and events are exported
	span.End()

	// give a tiny moment for sync exporter (should be immediate)
	time.Sleep(5 * time.Millisecond)

	// look for an exported span with an event named "log"
	evs := exp.events["test-span"]
	found := false
	for _, ev := range evs {
		if ev.Name == "log" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a span event named 'log' to be exported; got events=%v", evs)
	}
}

type testLogExporter struct {
	records []sdklog.Record
}

func (t *testLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	for _, r := range records {
		t.records = append(t.records, r.Clone())
	}
	return nil
}

func (t *testLogExporter) Shutdown(context.Context) error { return nil }

func (t *testLogExporter) ForceFlush(context.Context) error { return nil }

func TestOtelLogHookEmitsLogRecord(t *testing.T) {
	ctx := context.Background()
	exp := &testLogExporter{}
	lgProvider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)))
	t.Cleanup(func() { _ = lgProvider.Shutdown(context.Background()) })

	l := New(Options{Format: "text", Level: "debug", Name: "test-logs", Out: &bytes.Buffer{}})
	if err := l.SetupTelemetry(ctx, nil); err != nil {
		t.Fatalf("setup telemetry failed: %v", err)
	}
	if l.otelHook == nil {
		t.Fatalf("otel hook not initialized on logger")
	}
	l.otelHook.setEmitter(lgProvider.Logger("test-logs"))

	l.WithContext(ctx).WithField("user_id", 42).Warn("emit record")

	if len(exp.records) == 0 {
		t.Fatalf("expected log records to be exported")
	}
	rec := exp.records[len(exp.records)-1]
	if got := rec.Body().AsString(); got != "emit record" {
		t.Fatalf("unexpected log body: %s", got)
	}
	if rec.Severity() != otellog.SeverityWarn {
		t.Fatalf("expected severity warn, got %v", rec.Severity())
	}
	var hasUserID bool
	rec.WalkAttributes(func(kv otellog.KeyValue) bool {
		if kv.Key == "user_id" && kv.Value.AsInt64() == 42 {
			hasUserID = true
		}
		return true
	})
	if !hasUserID {
		t.Fatalf("expected user_id attribute in exported record")
	}
}
