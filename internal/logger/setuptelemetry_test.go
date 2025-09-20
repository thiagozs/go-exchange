package logger

import (
	"bytes"
	"context"
	"testing"

	"github.com/thiagozs/go-exchange/internal/config"
)

func TestSetupTelemetry_PopulatesFormatterFromConfig(t *testing.T) {
	cfg := &config.Config{
		AppName:    "test-app",
		AppVersion: "1.2.3",
		AppEnv:     "staging",
	}

	// create logger with JSON formatter to inspect fields
	l := New(Options{Format: "json", Level: "debug", Name: cfg.AppName, Out: &bytes.Buffer{}})

	if err := l.SetupTelemetry(context.Background(), cfg); err != nil {
		t.Fatalf("setup telemetry failed: %v", err)
	}

	// check underlying formatter values
	if f, ok := l.logrus.Formatter.(*OTelAwareJSONFormatter); ok {
		if f.AppVersion != cfg.AppVersion {
			t.Fatalf("expected AppVersion=%s on formatter; got=%s", cfg.AppVersion, f.AppVersion)
		}
		if f.AppMode != cfg.AppEnv {
			t.Fatalf("expected AppMode=%s on formatter; got=%s", cfg.AppEnv, f.AppMode)
		}
	} else {
		t.Fatalf("expected JSON formatter to be set")
	}
}
