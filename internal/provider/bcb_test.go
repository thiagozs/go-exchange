package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// fakeCache is a simple in-memory cache for tests.
type fakeCache struct {
	mu sync.Mutex
	m  map[string]string
}

func newFakeCache() *fakeCache {
	return &fakeCache{m: make(map[string]string)}
}

func (f *fakeCache) Get(ctx context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m[key], nil
}

func (f *fakeCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[key] = value
	return nil
}

func TestBCBProvider_ParsePlainJSONAndCache(t *testing.T) {
	// prepare a test server that returns a plain JSON
	body := `{"value":[{"cotacaoCompra":4.0,"cotacaoVenda":4.2,"dataHoraCotacao":"2025-09-19T12:00:00"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	cache := newFakeCache()
	p := NewBCBProvider(nil, srv.URL+"/", 2*time.Second, 1, 1, cache)

	// convert from BRL to USD (BRL -> USD uses rate = BRL per unit of USD)
	// amount 10000 cents = 100 BRL. rate 4.2 BRL per USD => result = 100/4.2 = ~23.8095 USD -> 2381 cents
	got, err := p.Convert(context.Background(), "BRL", "USD", 10000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2381 && got != 2380 { // allow rounding either way at .5 boundary
		t.Fatalf("unexpected result cents: %d", got)
	}

	// ensure cache populated
	if v, _ := cache.Get(context.Background(), "rates:bcb:USD"); v == "" {
		t.Fatalf("expected cached body for USD")
	}
}

func TestBCBProvider_ParseWrappedJSONAndLookback(t *testing.T) {
	// simulate: first day returns 500, previous day returns wrapped JSON
	day0 := true
	wrapped := "/*" + `{"value":[{"cotacaoCompra":5.0,"cotacaoVenda":5.5,"dataHoraCotacao":"2025-09-18T12:00:00"}]}` + "*/"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if day0 {
			day0 = false
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server error"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(wrapped))
	}))
	defer srv.Close()

	cache := newFakeCache()
	p := NewBCBProvider(nil, srv.URL+"/", 2*time.Second, 1, 2, cache)

	// Convert USD -> BRL: amount 100 USD = 10000 cents; rate 5.5 BRL per USD => 100*5.5 = 550 BRL -> 55000 cents
	got, err := p.Convert(context.Background(), "USD", "BRL", 10000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 55000 {
		t.Fatalf("unexpected result: %d", got)
	}
}
