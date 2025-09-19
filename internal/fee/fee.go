package fee

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/thiagozs/go-exchange/internal/logger"
)

// Provider returns fee percent as a float (e.g., 0.005 = 0.5%)
type Provider interface {
	FeePercent(from, to string) (float64, error)
}

// EnvFeeProvider reads a default fee percent from env var EXCHANGE_FEE_PERCENT
type EnvFeeProvider struct {
	percent float64
}

func NewEnvFeeProviderWithPercent(pct float64) *EnvFeeProvider {
	return &EnvFeeProvider{percent: pct}
}

func (e *EnvFeeProvider) FeePercent(from, to string) (float64, error) {
	return e.percent, nil
}

// FeeAPIProvider queries an external API to get fee percent for a pair.
type FeeAPIProvider struct {
	baseURL string
	client  *http.Client
	log     *logger.Logger
}

func NewFeeAPIProvider(url string, lg *logger.Logger) *FeeAPIProvider {
	if url == "" {
		return &FeeAPIProvider{baseURL: "", client: http.DefaultClient, log: lg}
	}
	return &FeeAPIProvider{baseURL: url, client: &http.Client{Timeout: 5 * time.Second}, log: lg}
}

type feeAPIResp struct {
	Percent float64 `json:"percent"`
}

func (f *FeeAPIProvider) FeePercent(from, to string) (float64, error) {
	if f.baseURL == "" {
		return 0, nil
	}
	url := f.baseURL + "?from=" + from + "&to=" + to
	req, _ := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	resp, err := f.client.Do(req)
	if err != nil {
		if f.log != nil {
			f.log.Errorf("fee api request error: %v", err)
		}
		return 0, err
	}
	defer resp.Body.Close()
	var r feeAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		if f.log != nil {
			f.log.Errorf("fee api decode error: %v", err)
		}
		return 0, err
	}
	if f.log != nil {
		f.log.Debugf("fee api percent: %v", r.Percent)
	}
	return r.Percent, nil
}
