package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thiagozs/go-exchange/internal/config"
	"github.com/thiagozs/go-exchange/internal/logger"
)

type mockProv struct{}

func (m *mockProv) Convert(ctx context.Context, from, to string, amount int64) (int64, error) {
	// return 200.00 as cents
	return 20000, nil
}

type stubCache struct{}

func (s *stubCache) Get(ctx context.Context, key string) (string, error) { return "", nil }
func (s *stubCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return nil
}

func TestHandleConvert(t *testing.T) {
	cfg := &config.Config{HTTPAddr: ":0", RedisAddr: "", RedisDB: 0, CacheTTL: 0}
	var buf bytes.Buffer
	lg := logger.New(logger.Options{Format: "text", Level: "debug", Out: &buf})
	srv := New(cfg, lg)
	// inject mocks
	srv.prov = &mockProv{}
	// create request
	// amount=1000 (10.00 units)
	req := httptest.NewRequest("GET", "/convert?from=USD&to=BRL&amount=1000", nil)
	w := httptest.NewRecorder()
	srv.handleConvert(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode err: %v", err)
	}
	if out["result"] != 200.0 {
		t.Fatalf("expected result 200.0 got %v", out["result"])
	}
}
