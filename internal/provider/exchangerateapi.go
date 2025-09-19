package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/thiagozs/go-exchange/internal/logger"
)

type ExchangeRateAPI struct {
	log     *logger.Logger
	cache   Cache
	baseURL string
	apiKey  string
}

func NewExchangeRateAPI(lg *logger.Logger, apiKey string, c Cache) *ExchangeRateAPI {
	return &ExchangeRateAPI{baseURL: "https://v6.exchangerate-api.com/v6", log: lg, apiKey: apiKey, cache: c}
}

type eraResponse struct {
	Result          string             `json:"result"`
	Documentation   string             `json:"documentation"`
	TermsOfUse      string             `json:"terms_of_use"`
	TimeLastUpdate  int64              `json:"time_last_update_unix"`
	BaseCode        string             `json:"base_code"`
	ConversionRates map[string]float64 `json:"conversion_rates"`
}

func (p *ExchangeRateAPI) Convert(ctx context.Context, from, to string, amount int64) (int64, error) {
	if p.apiKey == "" {
		return 0, MissingAPIKeyError{Info: "api key not provided for exchangerate-api"}
	}

	// Try cache of rates per base currency to avoid repeated upstream calls.
	cacheKey := "rates:exchangerate-api:" + from
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
		url := fmt.Sprintf("%s/%s/latest/%s", p.baseURL, p.apiKey, from)
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
			// cache raw rates for 20 minutes
			_ = p.cache.Set(ctx, cacheKey, string(raw), 20*time.Minute)
		}

		if p.log != nil {
			p.log.WithContext(ctx).Debugf("exchange raw response for url=%s body=%s", url, string(raw))
		}
	}

	var er eraResponse
	if err := json.Unmarshal(raw, &er); err != nil {
		if p.log != nil {
			p.log.WithContext(ctx).Errorf("decode exchange response error: %v", err)
		}
		return 0, err
	}

	if er.Result != "success" {
		if p.log != nil {
			p.log.WithContext(ctx).Errorf("exchange response not successful: result=%s", er.Result)
		}
		// exchange-rate-api returns result != "success" for invalid/missing API key
		return 0, MissingAPIKeyError{Info: "upstream returned non-success result"}
	}

	// find the target rate
	rate, ok := er.ConversionRates[to]
	if !ok {
		if p.log != nil {
			p.log.WithContext(ctx).Errorf("target currency %s not found in conversion rates", to)
		}
		return 0, fmt.Errorf("currency %s not found in exchange rates", to)
	}

	// amount units = amount cents / 100; multiply by rate to get target units
	amountUnits := float64(amount) / 100.0
	resultUnits := amountUnits * rate
	if p.log != nil {
		p.log.WithContext(ctx).Debugf("exchange convert computed: units=%v rate=%v result=%v", amountUnits, rate, resultUnits)
	}
	resultCents := int64(math.Round(resultUnits * 100.0))
	return resultCents, nil
}
