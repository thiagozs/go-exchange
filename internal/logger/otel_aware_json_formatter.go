package logger

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type OTelAwareJSONFormatter struct {
	TimestampFormat     string
	DisableUppercaseLvl bool
	AppName             string
	AppMode             string // Incomming, Outgoing, All
	ShowTraceIDs        bool
	EnableSpanBadge     bool
	SpanBadgeText       string
}

func (f *OTelAwareJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	data := make(map[string]any, len(entry.Data)+6)

	tsFmt := f.TimestampFormat

	if tsFmt == "" {
		tsFmt = time.RFC3339Nano
	}

	data["timestamp"] = entry.Time.Format(tsFmt)

	level := entry.Level.String()

	if !f.DisableUppercaseLvl {
		level = strings.ToUpper(level)
	}

	data["level"] = level

	app := f.AppName

	if app == "" {
		if v, ok := entry.Data["logger_name"]; ok {
			app = toString(v)
		}
	}

	if app != "" {
		data["app"] = app
	}

	data["msg"] = stripContextPrefix(entry.Message)

	traceID, hasTrace := entry.Data["trace_id"]
	spanID, hasSpan := entry.Data["span_id"]

	if f.ShowTraceIDs {
		if hasTrace {
			data["trace_id"] = toString(traceID)
		}

		if hasSpan {
			data["span_id"] = toString(spanID)
		}
	} else if f.EnableSpanBadge && (hasTrace || hasSpan) {
		badge := f.spanBadge()
		data["span"] = badge
	}

	keys := make([]string, 0, len(entry.Data))

	skips := []string{"logger_name", "trace_id", "span_id"}

	for k := range entry.Data {
		if slices.Contains(skips, k) {
			continue
		}

		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		data[k] = toString(entry.Data[k])
	}

	b, err := json.Marshal(data)

	if err != nil {
		return nil, err
	}

	return append(b, '\n'), nil
}

func (f *OTelAwareJSONFormatter) spanBadge() string {
	if strings.TrimSpace(f.SpanBadgeText) == "" {
		return "SPAN"
	}

	return f.SpanBadgeText
}
