// logger/otel_aware_formatter.go
package logger

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type OTelAwareTextFormatter struct {
	TimestampFormat     string // ex: time.RFC3339Nano
	DisableUppercaseLvl bool
	EnableColors        bool
	AppName             string // mostrado como [AppName]
	AppMode             string // Incomming, Outgoing, All

	// NOVO: controle de span/ids
	ShowTraceIDs    bool   // se true, imprime trace_id/span_id; se false, oculta
	EnableSpanBadge bool   // se true, mostra [SPAN] quando houver trace_id/span_id
	SpanBadgeText   string // texto do badge, ex: "SPAN"
}

// ANSI cores
var levelColors = map[logrus.Level]string{
	logrus.DebugLevel: "\033[36m",
	logrus.InfoLevel:  "\033[32m",
	logrus.WarnLevel:  "\033[33m",
	logrus.ErrorLevel: "\033[31m",
	logrus.FatalLevel: "\033[35m",
	logrus.PanicLevel: "\033[41m",
}

const resetColor = "\033[0m"

func (f *OTelAwareTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b bytes.Buffer

	ts := entry.Time.Format(f.ts())

	// App name (do formatter ou do campo)
	app := f.AppName
	if app == "" {
		if v, ok := entry.Data["logger_name"]; ok {
			app = toString(v)
		}
	}
	appStr := ""
	if app != "" {
		appStr = fmt.Sprintf("[%s]", app)
	}

	level := entry.Level.String()
	if !f.DisableUppercaseLvl {
		level = strings.ToUpper(level)
	}
	levelStr := fmt.Sprintf("[%s]", level)
	if f.EnableColors {
		if color, ok := levelColors[entry.Level]; ok {
			levelStr = color + levelStr + resetColor
		}
	}

	// timestamp [App] [LEVEL] msg
	if appStr != "" {
		b.WriteString(ts + " " + appStr + " " + levelStr + " ")
	} else {
		b.WriteString(ts + " " + levelStr + " ")
	}
	b.WriteString(stripContextPrefix(entry.Message)) // protege contra msgs que já venham com ctx como texto

	// trace/span
	traceID, hasTrace := entry.Data["trace_id"]
	spanID, hasSpan := entry.Data["span_id"]

	if f.ShowTraceIDs {
		if hasTrace {
			b.WriteString(" trace_id=" + toString(traceID))
		}
		if hasSpan {
			b.WriteString(" span_id=" + toString(spanID))
		}
	} else if f.EnableSpanBadge && (hasTrace || hasSpan) {
		b.WriteString(" [" + f.spanBadge() + "]")
	}

	// demais campos (ordenados), exceto logger_name/trace_id/span_id
	keys := make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		if k == "logger_name" || k == "trace_id" || k == "span_id" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(" " + k + "=" + toString(entry.Data[k]))
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *OTelAwareTextFormatter) ts() string {
	if f.TimestampFormat != "" {
		return f.TimestampFormat
	}
	return time.RFC3339Nano
}

func (f *OTelAwareTextFormatter) spanBadge() string {
	if strings.TrimSpace(f.SpanBadgeText) == "" {
		return "SPAN"
	}
	return f.SpanBadgeText
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		return x.Error()
	default:
		return strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf("%v", v), "\n", " "))
	}
}

// remove prefixos feios quando alguém passou o ctx como argumento por engano
func stripContextPrefix(msg string) string {
	if strings.HasPrefix(msg, "context.") || strings.Contains(msg, "trace.traceContextKeyType") {
		// tenta pegar a última parte após o ')'
		if p := strings.LastIndex(msg, ")"); p >= 0 && p+1 < len(msg) {
			m := strings.TrimSpace(msg[p+1:])
			if m != "" {
				return m
			}
		}
	}
	return msg
}
