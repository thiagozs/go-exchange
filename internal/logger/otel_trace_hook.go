package logger

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

type spanFieldsHook struct{}

func (spanFieldsHook) Levels() []logrus.Level { return logrus.AllLevels }

func (spanFieldsHook) Fire(e *logrus.Entry) error {
	ctx := e.Context
	if ctx == nil {
		ctx = context.Background()
	}
	sp := trace.SpanFromContext(ctx)
	if sp == nil {
		return nil
	}
	sc := sp.SpanContext()
	// If span context is valid, expose trace_id/span_id. Otherwise mark span presence
	// so formatters can show a [SPAN] badge even for noop/non-recording tracers.
	if sc.IsValid() {
		e.Data["trace_id"] = sc.TraceID().String()
		e.Data["span_id"] = sc.SpanID().String()
	} else {
		// mark span presence by adding span_id key (empty string). Formatters
		// will detect span_id presence and show the span badge even if IDs
		// are not valid (e.g., noop tracer).
		e.Data["span_id"] = ""
	}
	return nil
}
