package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/thiagozs/go-exchange/internal/logger"
)

// BCBProvider queries PTAX endpoints from Central
// Bank of Brazil.
type BCBProvider struct {
	log         *logger.Logger
	timeout     time.Duration
	cache       Cache
	baseURL     string
	maxRetries  int
	maxBackDays int
}

// NewBCBProvider constructs a new BCBProvider. If baseURL is empty a sensible default is used.
func NewBCBProvider(lg *logger.Logger, baseURL string, timeout time.Duration, maxRetries, maxBackDays int, c Cache) *BCBProvider {
	if baseURL == "" {
		baseURL = "https://olinda.bcb.gov.br/olinda/servico/PTAX/versao/v1/odata/"
	}
	return &BCBProvider{baseURL: strings.TrimRight(baseURL, "/") + "/", log: lg, timeout: timeout, maxRetries: maxRetries, maxBackDays: maxBackDays, cache: c}
}

type bcbResponse struct {
	Value []struct {
		CotacaoCompra float64 `json:"cotacaoCompra"`
		CotacaoVenda  float64 `json:"cotacaoVenda"`
		DataHora      string  `json:"dataHoraCotacao"`
	} `json:"value"`
}

func (b *BCBProvider) buildURL(currency string, date time.Time) string {
	cur := strings.ToUpper(currency)
	d := date.Format("01-02-2006")
	if cur == "USD" {
		return fmt.Sprintf(b.baseURL+"CotacaoDolarPeriodo(dataInicial=@dataInicial,dataFinalCotacao=@dataFinalCotacao)?@dataInicial='%s'&@dataFinalCotacao='%s'&$top=100&$format=json&$select=cotacaoCompra,cotacaoVenda,dataHoraCotacao", d, d)
	}
	return fmt.Sprintf(b.baseURL+"CotacaoMoedaAberturaOuIntermediario(codigoMoeda=@codigoMoeda,dataCotacao=@dataCotacao)?@codigoMoeda='%s'&@dataCotacao='%s'&$format=json&$select=cotacaoCompra,cotacaoVenda,dataHoraCotacao,tipoBoletim", cur, d)
}

// Convert converts amount (cents) from 'from' to 'to' using BCB PTAX rates.
// BCB provides BRL per unit of currency (venda). We use BRL as intermediary when needed.
func (b *BCBProvider) Convert(ctx context.Context, from, to string, amount int64) (int64, error) {
	fromU := strings.ToUpper(from)
	toU := strings.ToUpper(to)
	if fromU == toU {
		return amount, nil
	}

	// get venda rate BRL per unit for currency
	getRate := func(currency string) (float64, error) {
		cacheKey := "rates:bcb:" + strings.ToUpper(currency)
		if b.cache != nil {
			if cached, err := b.cache.Get(ctx, cacheKey); err == nil && cached != "" {
				if b.log != nil {
					b.log.WithContext(ctx).Debugf("using cached bcb rates for %s", currency)
				}
				var br bcbResponse
				if err := json.Unmarshal([]byte(cached), &br); err == nil && len(br.Value) > 0 {
					return br.Value[0].CotacaoVenda, nil
				}
			}
		}

		client := &http.Client{Timeout: b.timeout}
		for i := 0; i <= b.maxBackDays; i++ {
			tryDate := time.Now().AddDate(0, 0, -i)
			url := b.buildURL(currency, tryDate)
			if b.log != nil {
				b.log.WithContext(ctx).Debugf("bcb request url=%s", url)
			}

			var resp *http.Response
			var err error
			for attempt := 0; attempt <= b.maxRetries; attempt++ {
				req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
				resp, err = client.Do(req)
				if err != nil {
					return 0, err
				}
				if resp.StatusCode == http.StatusOK {
					break
				}
				if resp.StatusCode >= 500 && attempt < b.maxRetries {
					resp.Body.Close()
					time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
					continue
				}
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return 0, fmt.Errorf("bcb returned status=%d body=%s", resp.StatusCode, string(body))
			}

			if resp == nil {
				continue
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return 0, err
			}

			var br bcbResponse
			if err := json.Unmarshal(bodyBytes, &br); err != nil {
				s := strings.TrimSpace(string(bodyBytes))
				if strings.HasPrefix(s, "/*") && strings.HasSuffix(s, "*/") {
					s = strings.TrimPrefix(s, "/*")
					s = strings.TrimSuffix(s, "*/")
					s = strings.TrimSpace(s)
					if err2 := json.Unmarshal([]byte(s), &br); err2 != nil {
						return 0, err
					}
				} else {
					return 0, err
				}
			}

			if len(br.Value) == 0 {
				if i < b.maxBackDays {
					continue
				}
				return 0, fmt.Errorf("no bcb rate found for currency %s", currency)
			}

			if b.cache != nil {
				_ = b.cache.Set(ctx, cacheKey, string(bodyBytes), 20*time.Minute)
			}
			return br.Value[0].CotacaoVenda, nil
		}
		return 0, fmt.Errorf("no bcb rate found for %s in last %d days", currency, b.maxBackDays)
	}

	// convert using BRL as intermediary
	if fromU == "BRL" {
		toBRL, err := getRate(toU)
		if err != nil {
			return 0, err
		}
		amountUnits := float64(amount) / 100.0
		toUnits := amountUnits / toBRL
		return int64(math.Round(toUnits * 100.0)), nil
	}
	if toU == "BRL" {
		fromBRL, err := getRate(fromU)
		if err != nil {
			return 0, err
		}
		amountUnits := float64(amount) / 100.0
		brlUnits := amountUnits * fromBRL
		return int64(math.Round(brlUnits * 100.0)), nil
	}

	fromBRL, err := getRate(fromU)
	if err != nil {
		return 0, err
	}
	toBRL, err := getRate(toU)
	if err != nil {
		return 0, err
	}

	rate := fromBRL / toBRL
	amountUnits := float64(amount) / 100.0
	resultUnits := amountUnits * rate
	return int64(math.Round(resultUnits * 100.0)), nil
}
