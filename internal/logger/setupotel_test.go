package logger

import (
	"context"
	"testing"

	"github.com/thiagozs/go-exchange/internal/config"
)

func TestSetupOTel_SkipEnvironment(t *testing.T) {
	l := New(Options{Format: "text", Level: "debug", Name: "test"})
	cfg := &config.Config{Environment: "test"}
	sd, infos, err := l.SetupOTel(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected no error for skip env; got=%v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("expected no exporter infos for skipped env, got %v", infos)
	}
	if sd == nil {
		// ok: nil shutdown returned for skip
		return
	}
	// call shutdown if not nil
	_ = sd(context.Background())
}

func TestSetupOTel_HTTP_NoTLS_Headers(t *testing.T) {
	l := New(Options{Format: "text", Level: "debug", Name: "test-http"})
	cfg := &config.Config{
		AppName:      "test-http",
		AppVersion:   "v0",
		AppEnv:       "ci",
		OTLPEndpoint: "http://localhost:4318",
		OTLPHeaders:  "X-Api-Key=abcdef",
		OTLPUseTLS:   false,
	}
	sd, infos, err := l.SetupOTel(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected no error setting up http otel: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("expected 3 exporter infos, got %v", infos)
	}
	if sd != nil {
		_ = sd(context.Background())
	}
}

func TestSetupOTel_Grpc_TLS(t *testing.T) {
	l := New(Options{Format: "text", Level: "debug", Name: "test-grpc"})
	cfg := &config.Config{
		AppName:      "test-grpc",
		AppVersion:   "v0",
		AppEnv:       "ci",
		OTLPEndpoint: "localhost:4317",
		OTLPUseTLS:   true,
	}
	sd, infos, err := l.SetupOTel(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected no error setting up grpc otel: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("expected 3 exporter infos, got %v", infos)
	}
	if sd != nil {
		_ = sd(context.Background())
	}
}
