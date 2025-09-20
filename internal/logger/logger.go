package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/thiagozs/go-exchange/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type KeyValue = attribute.KeyValue
type Key = attribute.Key
type Value = attribute.Value
type Span = trace.Span

// Logger is a thin wrapper around logrus.Logger
type Logger struct {
	logrus    *logrus.Logger
	tracer    trace.Tracer
	name      string
	formatter string
	otelHook  *otelLogHook
}

// Options for initializing the logger
type Options struct {
	Format string // "text" or "json"
	Level  string // info, debug, warn, error
	Name   string // application name
	Out    io.Writer
}

func NewLogger(name, format string) (*Logger, error) {
	lg := logrus.New()

	formatter := getFormatter(format, name)
	lg.SetFormatter(formatter)

	// ensure span->fields hook is always present so formatters can show span badges
	lg.AddHook(spanFieldsHook{})
	// create OTEL hook upfront so it can be wired once exporters are configured
	otelHook := &otelLogHook{loggerName: name}
	lg.AddHook(otelHook)

	return &Logger{logrus: lg,
		tracer:    otel.Tracer(name),
		name:      name,
		formatter: format,
		otelHook:  otelHook,
	}, nil
}

func New(opts Options) *Logger {
	l, err := NewLogger(opts.Name, opts.Format)
	if err != nil {
		tmp := logrus.New()
		if opts.Out != nil {
			tmp.Out = opts.Out
		}
		lvl, perr := logrus.ParseLevel(opts.Level)
		if perr == nil {
			tmp.SetLevel(lvl)
		}
		tmp.AddHook(spanFieldsHook{})
		hook := &otelLogHook{loggerName: opts.Name}
		tmp.AddHook(hook)
		return &Logger{logrus: tmp,
			tracer:    otel.Tracer(opts.Name),
			name:      opts.Name,
			formatter: opts.Format,
			otelHook:  hook,
		}
	}
	if opts.Out != nil {
		l.logrus.SetOutput(opts.Out)
	}
	if opts.Level != "" {
		if lvl, perr := logrus.ParseLevel(opts.Level); perr == nil {
			l.logrus.SetLevel(lvl)
		}
	}
	return l
}

func getFormatter(format string, name string) logrus.Formatter {
	if format == "json" {
		return &OTelAwareJSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			AppName:         name,
			ShowTraceIDs:    false,
			EnableSpanBadge: true,
			SpanBadgeText:   "SPAN",
		}
	}

	return &OTelAwareTextFormatter{
		TimestampFormat: time.RFC3339,
		EnableColors:    true,
		AppName:         name,
		ShowTraceIDs:    false,
		EnableSpanBadge: true,
		SpanBadgeText:   "SPAN",
	}
}

func (l *Logger) SetAppNameSlog(name string) {
	if l.logrus == nil {
		l.logrus = logrus.New()
	}
	if f, ok := l.logrus.Formatter.(*OTelAwareTextFormatter); ok {
		f.AppName = name
	} else if f, ok := l.logrus.Formatter.(*OTelAwareJSONFormatter); ok {
		f.AppName = name
	} else {
		l.logrus.SetFormatter(getFormatter(l.formatter, name))
	}
}

func (l *Logger) SetAppModeSlog(mode string) {
	if l.logrus == nil {
		l.logrus = logrus.New()
	}

	if f, ok := l.logrus.Formatter.(*OTelAwareTextFormatter); ok {
		f.AppMode = mode
	} else if f, ok := l.logrus.Formatter.(*OTelAwareJSONFormatter); ok {
		f.AppMode = mode
	}
}

func (l *Logger) SetupTelemetry(ctx context.Context, cfg *config.Config) error {
	// register hooks so formatters and span integrations work for subsequent logs
	l.logrus.AddHook(spanFieldsHook{})
	if l.otelHook == nil {
		h := &otelLogHook{loggerName: l.name}
		l.otelHook = h
		l.logrus.AddHook(h)
	}

	// populate formatter metadata (version, mode) from config if provided,
	// fallback to environment variables.
	appVersion := strings.TrimSpace(getenvDefault("APP_VERSION", getenvDefault("VERSION", "")))
	appMode := strings.TrimSpace(getenvDefault("APP_ENV", getenvDefault("ENVIRONMENT", "")))
	if cfg != nil {
		if cfg.AppVersion != "" {
			appVersion = strings.TrimSpace(cfg.AppVersion)
		}
		if cfg.AppEnv != "" {
			appMode = strings.TrimSpace(cfg.AppEnv)
		}
	}

	if f, ok := l.logrus.Formatter.(*OTelAwareJSONFormatter); ok {
		if appVersion != "" {
			f.AppVersion = appVersion
		}
		if appMode != "" {
			f.AppMode = appMode
		}
	}
	if f, ok := l.logrus.Formatter.(*OTelAwareTextFormatter); ok {
		if appVersion != "" {
			f.AppVersion = appVersion
		}
		if appMode != "" {
			f.AppMode = appMode
		}
	}

	// annotate logs with hostname if available
	if hn, err := os.Hostname(); err == nil && hn != "" {
		l.logrus.WithField("host", hn)
	}

	// emit a debug log so startup logs make it clear what's configured
	if l.logrus != nil {
		collector := ""
		if cfg != nil {
			collector = strings.TrimSpace(cfg.OTelCollector)
		}
		if collector == "" {
			collector = strings.TrimSpace(os.Getenv("OTEL_COLLECTOR_URL"))
		}
		if collector == "" {
			l.WithContext(ctx).Debugf("telemetry hooks registered: spanFieldsHook, otelLogHook; OTEL collector not configured; app=%s version=%s env=%s", l.name, appVersion, appMode)
		} else {
			l.WithContext(ctx).Debugf("telemetry hooks registered: spanFieldsHook, otelLogHook; OTEL_COLLECTOR_URL=%s; app=%s version=%s env=%s", collector, l.name, appVersion, appMode)
		}
	}

	return nil
}

// getenvDefault returns the environment variable value or the provided default when empty.
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func getFileName() string {
	// skip 3 frames to point to the caller of the logger helper
	pc, file, line, ok := runtime.Caller(3)

	filename := "unknown"
	if ok {
		parts := strings.Split(file, "/")
		funcName := "unknown"
		if f := runtime.FuncForPC(pc); f != nil {
			// return only the last part of the function path
			fnParts := strings.Split(f.Name(), ".")
			funcName = fnParts[len(fnParts)-1]
		}
		filename = fmt.Sprintf("%s:%d:%s", parts[len(parts)-1], line, funcName)
	}
	return filename
}

func (l *Logger) fillFields(fields logrus.Fields) logrus.Fields {
	fields["file"] = getFileName()

	if f, ok := l.logrus.Formatter.(*OTelAwareJSONFormatter); ok {
		fields["origin"] = f.AppName
		fields["mode"] = f.AppMode
	}
	if f, ok := l.logrus.Formatter.(*OTelAwareTextFormatter); ok {
		fields["origin"] = f.AppName
		fields["mode"] = f.AppMode
	}

	return fields
}

func (l *Logger) Slog() *logrus.Entry {
	return l.logrus.WithFields(l.fillFields(logrus.Fields{}))
}

func (l *Logger) WithFields(fields logrus.Fields) *logrus.Entry {
	return l.logrus.WithFields(l.fillFields(fields))
}

func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
	// include default fields (file/origin/mode) on entries created with context
	return l.logrus.WithContext(ctx).WithFields(l.fillFields(logrus.Fields{}))
}

func (l *Logger) SlogWithFields(ctx context.Context, fields logrus.Fields) *logrus.Entry {
	// use WithContext which already includes the standard fields and then add user fields
	return l.WithContext(ctx).WithFields(fields)
}

func (l *Logger) SetTracer(name string) {
	l.tracer = otel.Tracer(name)
}

func (l *Logger) Infof(format string, args ...any) {
	l.WithContext(context.Background()).Infof(format, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	l.WithContext(context.Background()).Errorf(format, args...)
}

func (l *Logger) Debugf(format string, args ...any) {
	l.WithContext(context.Background()).Debugf(format, args...)
}

func (l *Logger) Span(ctx context.Context, name string, attr ...KeyValue) (context.Context, Span) {
	if l.tracer == nil {
		l.tracer = otel.Tracer("logger")
		l.WithContext(context.Background()).Warn("tracer not set; using default 'logger'")
	}

	if len(attr) > 0 {
		keys := make([]string, len(attr))
		for i := range attr {
			keys[i] = string(attr[i].Key)
		}

		l.WithContext(context.Background()).Debugf("starting span %q with %d attrs: %v", name, len(attr), keys)
	}

	return l.tracer.Start(ctx, name, trace.WithAttributes(attr...))
}

func (l *Logger) Start(ctx context.Context, name string, m map[string]any) (context.Context, Span) {
	return l.Span(ctx, name, WithAttrs(m)...)
}

func (l *Logger) AddAttrs(span Span, m map[string]any) {
	if span == nil || len(m) == 0 {
		return
	}

	span.SetAttributes(WithAttrs(m)...)
}

func WithAttrs(m map[string]any) []KeyValue {
	if len(m) == 0 {
		return nil
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	attrs := make([]KeyValue, 0, len(keys))
	for _, k := range keys {
		v := m[k]
		if v == nil {
			continue
		}
		if kv, ok := toKV(k, v); ok {
			attrs = append(attrs, kv)
		}
	}
	return attrs
}

func normalizeKey(k string) string {
	return strings.ReplaceAll(strings.TrimSpace(k), " ", "_")
}

func sanitizeFloat64(f float64) (float64, bool, string) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		switch {
		case math.IsNaN(f):
			return 0, false, "NaN"
		case math.IsInf(f, +1):
			return 0, false, "+Inf"
		default:
			return 0, false, "-Inf"
		}
	}
	return f, true, ""
}

func toKV(key string, v any) (KeyValue, bool) {
	key = normalizeKey(key)

	switch x := v.(type) {
	case string:
		return attribute.String(key, x), true
	case bool:
		return attribute.Bool(key, x), true

	case int:
		return attribute.Int(key, x), true
	case int8, int16, int32:
		return attribute.Int(key, int(reflect.ValueOf(x).Int())), true
	case int64:
		return attribute.Int64(key, x), true

	case uint, uint8, uint16, uint32:
		return attribute.Int64(key, int64(reflect.ValueOf(x).Uint())), true
	case uint64:
		if x <= math.MaxInt64 {
			return attribute.Int64(key, int64(x)), true
		}
		return attribute.String(key, strconv.FormatUint(x, 10)), true
	case float32:
		f2, ok, s := sanitizeFloat64(float64(x))
		if ok {
			return attribute.Float64(key, f2), true
		}
		return attribute.String(key, s), true
	case float64:
		f2, ok, s := sanitizeFloat64(x)
		if ok {
			return attribute.Float64(key, f2), true
		}
		return attribute.String(key, s), true
	case time.Time:
		return attribute.String(key, x.Format(time.RFC3339Nano)), true
	case time.Duration:
		return attribute.Int64(key, x.Milliseconds()), true
	case fmt.Stringer:
		return attribute.String(key, x.String()), true
	case error:
		return attribute.String(key, x.Error()), true

	case []string:
		return attribute.StringSlice(key, x), true
	case []bool:
		return attribute.BoolSlice(key, x), true
	case []int:
		return attribute.IntSlice(key, x), true
	case []int32:
		tmp := make([]int, len(x))
		for i, n := range x {
			tmp[i] = int(n)
		}
		return attribute.IntSlice(key, tmp), true
	case []int64:
		return attribute.Int64Slice(key, x), true
	case []float32:
		tmp := make([]float64, len(x))
		for i, f := range x {
			if f2, ok, _ := sanitizeFloat64(float64(f)); ok {
				tmp[i] = f2
			} else {
				ss := make([]string, len(x))
				for j := range x {
					ss[j] = fmt.Sprint(x[j])
				}
				return attribute.StringSlice(key, ss), true
			}
		}
		return attribute.Float64Slice(key, tmp), true
	case []float64:
		tmp := make([]float64, len(x))
		for i, f := range x {
			if f2, ok, _ := sanitizeFloat64(f); ok {
				tmp[i] = f2
			} else {
				ss := make([]string, len(x))
				for j := range x {
					ss[j] = fmt.Sprint(x[j])
				}
				return attribute.StringSlice(key, ss), true
			}
		}
		return attribute.Float64Slice(key, tmp), true
	case []time.Time:
		ss := make([]string, len(x))
		for i := range x {
			ss[i] = x[i].Format(time.RFC3339Nano)
		}
		return attribute.StringSlice(key, ss), true

	case []any:
		ss := make([]string, len(x))
		for i := range x {
			ss[i] = fmt.Sprint(x[i])
		}
		return attribute.StringSlice(key, ss), true

	case map[string]any:
		if b, err := json.Marshal(x); err == nil {
			return attribute.String(key, string(b)), true
		}
		return attribute.String(key, fmt.Sprint(x)), true
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		return toKV(key, rv.Elem().Interface())
	}

	if b, err := json.Marshal(v); err == nil {
		return attribute.String(key, string(b)), true
	}
	return attribute.String(key, fmt.Sprint(v)), true
}

func FlatAttrs(m map[string]any) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		v := m[k]
		if v == nil {
			continue
		}
		out = append(out, k)
		switch x := v.(type) {
		case fmt.Stringer:
			out = append(out, x.String())
		case error:
			out = append(out, x.Error())
		default:
			out = append(out, fmt.Sprint(x))
		}
	}
	return out
}

func FlatAttrsKV(m map[string]any) []any {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]any, 0, len(keys)*2)
	for _, k := range keys {
		v := m[k]
		if v == nil {
			continue
		}
		out = append(out, k)
		switch x := v.(type) {
		case fmt.Stringer:
			out = append(out, x.String())
		case error:
			out = append(out, x.Error())
		default:
			out = append(out, fmt.Sprint(x))
		}
	}
	return out
}
