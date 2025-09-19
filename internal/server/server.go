package server

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thiagozs/go-exchange/internal/cache"
	"github.com/thiagozs/go-exchange/internal/config"
	"github.com/thiagozs/go-exchange/internal/fee"
	"github.com/thiagozs/go-exchange/internal/logger"
	"github.com/thiagozs/go-exchange/internal/provider"
)

type Server struct {
	cfg   *config.Config
	cache provider.Cache
	prov  provider.Provider
	fee   fee.Provider
	log   *logger.Logger
}

// respWriter captures HTTP status and size
type respWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *respWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *respWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

func New(cfg *config.Config, lg *logger.Logger) *Server {
	c := cache.New(cfg.RedisAddr, cfg.RedisDB,
		cfg.RedisUsername, cfg.RedisPassword, lg)

	prov := provider.NewProviderFromConfig(cfg, lg, c)

	var fprov fee.Provider

	if cfg.FeeAPIURL != "" {
		fprov = fee.NewFeeAPIProvider(cfg.FeeAPIURL, lg)
	} else if cfg.FeePercent > 0 {
		fprov = fee.NewEnvFeeProviderWithPercent(cfg.FeePercent)
	}

	return &Server{cfg: cfg, cache: c,
		prov: prov, fee: fprov, log: lg,
	}
}

func (s *Server) Run() error {
	http.HandleFunc("/convert", s.instrumentHandler(s.handleConvert))
	http.HandleFunc("/health", s.instrumentHandler(s.handleHealth))
	s.log.WithContext(context.Background()).Infof("listening on %s", s.cfg.HTTPAddr)
	return http.ListenAndServe(s.cfg.HTTPAddr, nil)
}

func (s *Server) instrumentHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx, end := s.log.StartSpan(r.Context(), r.URL.Path)
		defer end()
		// pass context with span to request handlers
		r = r.WithContext(ctx)
		rw := &respWriter{ResponseWriter: w,
			status: http.StatusOK,
		}

		next(rw, r)

		duration := time.Since(start)

		// structured access log
		entry := s.log.WithContext(ctx).WithFields(map[string]any{
			"method":   r.Method,
			"path":     r.URL.Path,
			"status":   rw.status,
			"duration": duration.Seconds(),
			"size":     rw.size,
		})

		// add trace_id/span_id if present
		span := s.log.WithContext(ctx)
		if v, ok := span.Data["trace_id"]; ok {
			entry = entry.WithField("trace_id", v)
		}
		if v, ok := span.Data["span_id"]; ok {
			entry = entry.WithField("span_id", v)
		}

		entry.Info("access")
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	amountStr := r.URL.Query().Get("amount")
	if from == "" || to == "" || amountStr == "" {
		http.Error(w, "missing parameters", http.StatusBadRequest)
		return
	}
	// amount can be integer cents (1000 => 10.00) or decimal units (10.00)
	var amountInt int64
	if strings.Contains(amountStr, ".") {
		f, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			http.Error(w, "invalid amount", http.StatusBadRequest)
			return
		}

		amountInt = int64(math.Round(f * 100.0))
	} else {
		ai, err := strconv.ParseInt(amountStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid amount", http.StatusBadRequest)
			return
		}

		amountInt = ai
	}

	// normalize cache key to use integer cents to avoid duplicates
	key := "convert:" + from + ":" + to + ":" + strconv.FormatInt(amountInt, 10)
	if val, err := s.cache.Get(ctx, key); err == nil && val != "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(val))
		return
	}
	resCents, err := s.prov.Convert(ctx, from, to, amountInt)
	if err != nil {
		// if upstream complains about missing API key, return a clearer status
		if mae, ok := err.(interface{ Error() string }); ok {
			// check error message content for the specific MissingAPIKeyError
			if _, isMissing := err.(provider.MissingAPIKeyError); isMissing {
				s.log.Errorf("provider missing API key: %v", err)
				http.Error(w, "exchange provider requires an API key. Set EXCHANGE_API_KEY.", http.StatusBadGateway)
				return
			}
			_ = mae
		}
		s.log.Errorf("provider error: %v", err)
		http.Error(w, "provider error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// apply fee (if configured)
	var feePct float64
	if s.fee != nil {
		feePct, _ = s.fee.FeePercent(from, to)
	}

	// feeAmt in cents
	feeAmt := int64(math.Round(float64(resCents) * feePct))

	netCents := resCents - feeAmt

	out := map[string]any{"from": from,
		"to": to, "amount_cents": amountInt,
		"result_cents":     resCents,
		"result":           float64(resCents) / 100.0,
		"fee_percent":      feePct,
		"fee_amount_cents": feeAmt,
		"net_result_cents": netCents,
		"net_result":       float64(netCents) / 100.0,
	}

	b, _ := json.Marshal(out)

	// avoid caching zero results which are likely from a failed provider call
	if resCents == 0 {
		s.log.Errorf("not caching zero conversion result for %s->%s amount=%d", from, to, amountInt)
	} else {
		s.cache.Set(ctx, key, string(b), s.cfg.CacheTTL)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
