package logger

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// otelLogHook sends logrus entries as events to the current span (if any).
type otelLogHook struct{}

func (otelLogHook) Levels() []logrus.Level { return logrus.AllLevels }

func (otelLogHook) Fire(e *logrus.Entry) error {
	ctx := e.Context
	if ctx == nil {
		return nil
	}
	sp := trace.SpanFromContext(ctx)
	if sp == nil {
		return nil
	}
	// avoid paying the cost when span is not recording (not sampled)
	if !sp.IsRecording() {
		return nil
	}

	// build attributes from entry.Data with typed attributes when possible
	attrs := make([]attribute.KeyValue, 0, len(e.Data)+2)
	attrs = append(attrs, attribute.String("level", e.Level.String()))
	attrs = append(attrs, attribute.String("msg", e.Message))
	for k, v := range e.Data {
		if k == "trace_id" || k == "span_id" || k == "logger_name" {
			continue
		}
		switch x := v.(type) {
		case string:
			attrs = append(attrs, attribute.String(k, x))
		case bool:
			attrs = append(attrs, attribute.Bool(k, x))
		case int:
			attrs = append(attrs, attribute.Int(k, x))
		case int64:
			attrs = append(attrs, attribute.Int64(k, x))
		case float64:
			attrs = append(attrs, attribute.Float64(k, x))
		case error:
			attrs = append(attrs, attribute.String(k, x.Error()))
		default:
			attrs = append(attrs, attribute.String(k, fmt.Sprint(v)))
		}
	}

	sp.AddEvent("log", trace.WithAttributes(attrs...))
	return nil
}
