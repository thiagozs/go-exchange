package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	HTTPAddr         string        `env:"HTTP_ADDR" envDefault:":8080"`
	RedisAddr        string        `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisDB          int           `env:"REDIS_DB" envDefault:"0"`
	RedisUsername    string        `env:"REDIS_USERNAME" envDefault:""`
	RedisPassword    string        `env:"REDIS_PASSWORD" envDefault:""`
	RedisRequireAuth bool          `env:"REDIS_REQUIRE_AUTH" envDefault:"false"`
	Provider         string        `env:"EXCHANGE_PROVIDER" envDefault:"exchangerate.host"`
	CacheTTL         time.Duration `env:"CACHE_TTL" envDefault:"5m"`
	FeeAPIURL        string        `env:"FEE_API_URL" envDefault:""`
	FeePercent       float64       `env:"EXCHANGE_FEE_PERCENT" envDefault:"0"`
	ExchangeAPIKey   string        `env:"EXCHANGE_API_KEY" envDefault:""`
	// BCB / PTAX provider specific settings
	BCBAPIBaseURL  string        `env:"BCB_API_BASE_URL" envDefault:"https://olinda.bcb.gov.br/olinda/servico/PTAX/versao/v1/odata/"`
	BCBTimeout     time.Duration `env:"BCB_TIMEOUT_SECONDS" envDefault:"10s"`
	BCBMaxRetries  int           `env:"BCB_MAX_RETRIES" envDefault:"3"`
	BCBMaxBackDays int           `env:"BCB_MAX_BACK_DAYS" envDefault:"0"`
	// Logger configuration
	LogFormat     string `env:"LOG_FORMAT" envDefault:"text"` // text or json
	LogLevel      string `env:"LOG_LEVEL" envDefault:"info"`
	OTelCollector string `env:"OTEL_COLLECTOR_URL" envDefault:""` // optional OTEL collector endpoint
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	// basic validation
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	// Warn if Redis address is configured but no password is set. Many Redis
	// deployments require authentication; this helps catch that misconfiguration.
	if cfg.RedisAddr != "" && cfg.RedisPassword == "" {
		// Print a simple warning; avoid creating a logger here to keep
		// config loading simple and free of side-effects.
		fmt.Printf("WARNING: REDIS_ADDR is set (%s) but REDIS_PASSWORD is empty. If your Redis requires auth, set REDIS_PASSWORD.\n", cfg.RedisAddr)
	}
	// If the deployment requires Redis authentication, fail fast when password
	// is not provided. This prevents the runtime NOAUTH errors seen earlier.
	if cfg.RedisAddr != "" && cfg.RedisRequireAuth && cfg.RedisPassword == "" {
		return nil, fmt.Errorf("redis requires authentication (REDIS_REQUIRE_AUTH=true) but REDIS_PASSWORD is empty")
	}
	return cfg, nil
}
