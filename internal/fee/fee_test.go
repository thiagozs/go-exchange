package fee

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thiagozs/go-exchange/internal/logger"
)

func TestEnvFeeProvider(t *testing.T) {
	p := NewEnvFeeProviderWithPercent(0.005)
	v, _ := p.FeePercent("USD", "BRL")
	if v != 0.005 {
		t.Fatalf("expected 0.005 got %v", v)
	}
}

func TestFeeAPIProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"percent":0.01}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	lg := logger.New(logger.Options{Format: "text", Level: "debug", Out: &buf})
	p := NewFeeAPIProvider(srv.URL, lg)
	v, err := p.FeePercent("USD", "BRL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 0.01 {
		t.Fatalf("expected 0.01 got %v", v)
	}
}
