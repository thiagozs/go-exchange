package logger

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

// otelLogHook forwards logrus entries to the active span (if recording) and, when configured,
// emits OpenTelemetry log records through the configured OTLP logger exporter.
type otelLogHook struct {
	mu         sync.RWMutex
	emitter    otellog.Logger
	loggerName string
}

func (h *otelLogHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *otelLogHook) setEmitter(logger otellog.Logger) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.emitter = logger
}

func (h *otelLogHook) getEmitter() otellog.Logger {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.emitter
}

func (h *otelLogHook) Fire(e *logrus.Entry) error {
	ctx := e.Context
	if ctx == nil {
		ctx = context.Background()
	}

	if logger := h.getEmitter(); logger != nil {
		sev := toLogSeverity(e.Level)
		if logger.Enabled(ctx, otellog.EnabledParameters{Severity: sev}) {
			record := buildLogRecord(e, h.loggerName, sev)
			logger.Emit(ctx, record)
		}
	}

	sp := trace.SpanFromContext(ctx)
	if sp == nil {
		return nil
	}
	// avoid paying the cost when span is not recording (not sampled)
	if !sp.IsRecording() {
		return nil
	}

	attrs := make([]attribute.KeyValue, 0, len(e.Data)+2)
	attrs = append(attrs, attribute.String("level", e.Level.String()))
	attrs = append(attrs, attribute.String("msg", e.Message))
	for _, k := range sortedKeys(e.Data) {
		if k == "trace_id" || k == "span_id" || k == "logger_name" {
			continue
		}
		v := e.Data[k]
		switch x := v.(type) {
		case string:
			attrs = append(attrs, attribute.String(k, x))
		case bool:
			attrs = append(attrs, attribute.Bool(k, x))
		case int:
			attrs = append(attrs, attribute.Int(k, x))
		case int8:
			attrs = append(attrs, attribute.Int(k, int(x)))
		case int16:
			attrs = append(attrs, attribute.Int(k, int(x)))
		case int32:
			attrs = append(attrs, attribute.Int(k, int(x)))
		case int64:
			attrs = append(attrs, attribute.Int64(k, x))
		case uint:
			attrs = append(attrs, attribute.Int64(k, int64(x)))
		case uint8:
			attrs = append(attrs, attribute.Int(k, int(x)))
		case uint16:
			attrs = append(attrs, attribute.Int(k, int(x)))
		case uint32:
			attrs = append(attrs, attribute.Int64(k, int64(x)))
		case uint64:
			if x <= math.MaxInt64 {
				attrs = append(attrs, attribute.Int64(k, int64(x)))
			} else {
				attrs = append(attrs, attribute.String(k, strconv.FormatUint(x, 10)))
			}
		case float32:
			attrs = append(attrs, attribute.Float64(k, float64(x)))
		case float64:
			attrs = append(attrs, attribute.Float64(k, x))
		case time.Duration:
			attrs = append(attrs, attribute.Int64(k, int64(x)))
		case error:
			attrs = append(attrs, attribute.String(k, x.Error()))
		case fmt.Stringer:
			attrs = append(attrs, attribute.String(k, x.String()))
		default:
			attrs = append(attrs, attribute.String(k, fmt.Sprint(v)))
		}
	}

	sp.AddEvent("log", trace.WithAttributes(attrs...))
	return nil
}

func buildLogRecord(entry *logrus.Entry, loggerName string, sev otellog.Severity) otellog.Record {
	record := otellog.Record{}
	record.SetTimestamp(entry.Time)
	record.SetObservedTimestamp(time.Now())
	record.SetSeverity(sev)
	record.SetSeverityText(strings.ToUpper(entry.Level.String()))
	record.SetBody(otellog.StringValue(entry.Message))

	attrs := make([]otellog.KeyValue, 0, len(entry.Data)+2)
	if loggerName != "" {
		attrs = append(attrs, otellog.String("logger.name", loggerName))
	}
	for _, k := range sortedKeys(entry.Data) {
		if kv, ok := toLogKeyValue(k, entry.Data[k]); ok {
			attrs = append(attrs, kv)
		}
	}
	if len(attrs) > 0 {
		record.AddAttributes(attrs...)
	}
	return record
}

func toLogSeverity(level logrus.Level) otellog.Severity {
	switch level {
	case logrus.PanicLevel, logrus.FatalLevel:
		return otellog.SeverityFatal
	case logrus.ErrorLevel:
		return otellog.SeverityError
	case logrus.WarnLevel:
		return otellog.SeverityWarn
	case logrus.InfoLevel:
		return otellog.SeverityInfo
	case logrus.DebugLevel:
		return otellog.SeverityDebug
	case logrus.TraceLevel:
		return otellog.SeverityTrace
	default:
		return otellog.SeverityInfo
	}
}

func toLogKeyValue(key string, value any) (otellog.KeyValue, bool) {
	switch v := value.(type) {
	case string:
		return otellog.String(key, v), true
	case bool:
		return otellog.Bool(key, v), true
	case int:
		return otellog.Int64(key, int64(v)), true
	case int8:
		return otellog.Int64(key, int64(v)), true
	case int16:
		return otellog.Int64(key, int64(v)), true
	case int32:
		return otellog.Int64(key, int64(v)), true
	case int64:
		return otellog.Int64(key, v), true
	case uint:
		return otellog.Int64(key, int64(v)), true
	case uint8:
		return otellog.Int64(key, int64(v)), true
	case uint16:
		return otellog.Int64(key, int64(v)), true
	case uint32:
		return otellog.Int64(key, int64(v)), true
	case uint64:
		if v <= math.MaxInt64 {
			return otellog.Int64(key, int64(v)), true
		}
		return otellog.String(key, strconv.FormatUint(v, 10)), true
	case float32:
		return otellog.Float64(key, float64(v)), true
	case float64:
		return otellog.Float64(key, v), true
	case time.Duration:
		return otellog.Int64(key, int64(v)), true
	case time.Time:
		return otellog.String(key, v.Format(time.RFC3339Nano)), true
	case []byte:
		return otellog.Bytes(key, append([]byte(nil), v...)), true
	case fmt.Stringer:
		return otellog.String(key, v.String()), true
	case error:
		return otellog.String(key, v.Error()), true
	case nil:
		return otellog.String(key, ""), true
	default:
		return otellog.String(key, fmt.Sprint(value)), true
	}
}

func sortedKeys(fields logrus.Fields) []string {
	if len(fields) == 0 {
		return nil
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
