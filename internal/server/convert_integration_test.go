package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thiagozs/go-exchange/internal/config"
	"github.com/thiagozs/go-exchange/internal/logger"
)

type httpMockProv struct{ url string }

func (m *httpMockProv) Convert(ctx context.Context, from, to string, amount int64) (int64, error) {
	resp, err := http.Get(m.url + "/convert")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var data struct {
		Result float64 `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}
	return int64(data.Result * 100.0), nil
}

// TestConvertIntegration starts a mock exchangerate server and verifies the
// /convert handler uses it end-to-end.
func TestConvertIntegration(t *testing.T) {
	// start mock provider
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// return result 42.42 for any request
		w.Write([]byte(`{"success":true,"result":42.42}`))
	}))
	defer mock.Close()

	cfg := &config.Config{HTTPAddr: ":0", RedisAddr: "", RedisDB: 0, CacheTTL: 0}
	lg := logger.New(logger.Options{Format: "text", Level: "debug"})
	srv := New(cfg, lg)

	// replace provider with a mock implementation that calls the mock server
	srv.prov = &httpMockProv{url: mock.URL}
	// use stub cache to avoid attempting Redis in tests
	srv.cache = &stubCache{}

	// perform request
	req := httptest.NewRequest("GET", "/convert?from=USD&to=BRL&amount=10.00", nil)
	w := httptest.NewRecorder()
	srv.handleConvert(w, req)
	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode err: %v", err)
	}
	if out["result"] != 42.42 {
		t.Fatalf("expected 42.42 got %v", out["result"])
	}
}
