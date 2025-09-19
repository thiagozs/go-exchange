package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/thiagozs/go-exchange/internal/config"
	"github.com/thiagozs/go-exchange/internal/logger"
)

type Provider interface {
	// amount is expressed in cents (int64). Return value is cents as int64.
	Convert(ctx context.Context, from, to string, amount int64) (int64, error)
}

// Cache is the minimal cache interface used by providers to store raw responses.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

// ExchangerateHost implements Provider using exchangerate.host
type ExchangerateHost struct {
	baseURL string
	log     *logger.Logger
	apiKey  string
	cache   Cache
}

func NewExchangerateHost(lg *logger.Logger, apiKey string, c Cache) *ExchangerateHost {
	return &ExchangerateHost{baseURL: "https://api.exchangerate.host", log: lg, apiKey: apiKey, cache: c}
}

type erResponse struct {
	Success bool           `json:"success"`
	Query   map[string]any `json:"query"`
	Info    map[string]any `json:"info"`
	Error   map[string]any `json:"error"`
	Result  float64        `json:"result"`
}

// MissingAPIKeyError indicates the upstream provider requires an API key.
type MissingAPIKeyError struct {
	Info string
}

func (e MissingAPIKeyError) Error() string { return "missing exchange provider API key: " + e.Info }

func (p *ExchangerateHost) Convert(ctx context.Context, from, to string, amount int64) (int64, error) {
	// Use cached rates per base currency when possible to reduce upstream calls.
	cacheKey := "rates:exchangerate.host:" + from
	var raw []byte
	if p.cache != nil {
		if cached, err := p.cache.Get(ctx, cacheKey); err == nil && cached != "" {
			if p.log != nil {
				p.log.WithContext(ctx).Debugf("using cached rates for base=%s", from)
			}
			raw = []byte(cached)
		}
	}
	if raw == nil {
		// fetch latest rates for base currency
		url := fmt.Sprintf("%s/latest?base=%s", p.baseURL, from)
		if p.apiKey != "" {
			url = url + fmt.Sprintf("&access_key=%s", p.apiKey)
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if p.log != nil {
				p.log.WithContext(ctx).Errorf("exchange request error: %v", err)
			}
			return 0, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if p.log != nil {
				p.log.WithContext(ctx).Errorf("exchange request failed status=%d body=%s", resp.StatusCode, string(body))
			}
			return 0, fmt.Errorf("exchange request failed status=%d", resp.StatusCode)
		}
		r, err := io.ReadAll(resp.Body)
		if err != nil {
			if p.log != nil {
				p.log.WithContext(ctx).Errorf("failed reading exchange response body: %v", err)
			}
			return 0, err
		}
		raw = r
		if p.cache != nil {
			_ = p.cache.Set(ctx, cacheKey, string(raw), 20*time.Minute)
		}
		if p.log != nil {
			p.log.WithContext(ctx).Debugf("exchange raw response for url=%s body=%s", url, string(raw))
		}
	}

	// parse response - reuse erResponse structure but note the latest endpoint
	var er struct {
		Success bool               `json:"success"`
		Rates   map[string]float64 `json:"rates"`
		Error   map[string]any     `json:"error"`
	}
	if err := json.Unmarshal(raw, &er); err != nil {
		if p.log != nil {
			p.log.WithContext(ctx).Errorf("decode exchange response error: %v", err)
		}
		return 0, err
	}
	if !er.Success {
		if p.log != nil {
			p.log.WithContext(ctx).Errorf("exchange response not successful: %+v", er)
		}
		// detect missing_access_key if present
		if er.Error != nil {
			if t, ok := er.Error["type"].(string); ok && t == "missing_access_key" {
				info := ""
				if v, ok := er.Error["info"].(string); ok {
					info = v
				}
				return 0, MissingAPIKeyError{Info: info}
			}
		}
		return 0, fmt.Errorf("exchange response not successful")
	}
	rate, ok := er.Rates[to]
	if !ok {
		if p.log != nil {
			p.log.WithContext(ctx).Errorf("target currency %s not found in rates", to)
		}
		return 0, fmt.Errorf("currency %s not found in exchange rates", to)
	}
	amountUnits := float64(amount) / 100.0
	resultUnits := amountUnits * rate
	resultCents := int64(math.Round(resultUnits * 100.0))
	if p.log != nil {
		p.log.WithContext(ctx).Debugf("exchange convert computed: units=%v rate=%v result=%v", amountUnits, rate, resultUnits)
	}
	return resultCents, nil
}

// NewProviderFromConfig creates a Provider based on config.
func NewProviderFromConfig(cfg *config.Config, lg *logger.Logger, c Cache) Provider {
	// for now we only support exchangerate.host as default
	switch cfg.Provider {
	case "exchangerate.host":
		return NewExchangerateHost(lg, cfg.ExchangeAPIKey, c)
	case "exchangerate-api", "exchangerate-api.com", "exchange-rate-api":
		return NewExchangeRateAPI(lg, cfg.ExchangeAPIKey, c)
	default:
		return NewExchangerateHost(lg, cfg.ExchangeAPIKey, c)
	}
}
