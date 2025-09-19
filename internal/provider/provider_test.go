package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchangerateHost_Convert(t *testing.T) {
	// mock exchangerate.host /convert response
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// return latest rates shape with BRL rate (12.345 so 10.00 * 12.345 = 123.45)
		w.Write([]byte(`{"success":true,"rates":{"BRL":12.345}}`))
	}))
	defer srv.Close()
	p := &ExchangerateHost{baseURL: srv.URL, log: nil, apiKey: "", cache: nil}
	// amount 10.00 => 1000 cents
	res, err := p.Convert(context.Background(), "USD", "BRL", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != 12345 {
		t.Fatalf("expected 12345 got %v", res)
	}
}
