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
	CacheTTL         time.Duration `env:"CACHE_TTL" envDefault:"5m"`
	FeeAPIURL        string        `env:"FEE_API_URL" envDefault:""`
	FeePercent       float64       `env:"EXCHANGE_FEE_PERCENT" envDefault:"0"`
	// Exchangerate.host or others - specific settings
	Provider       string `env:"EXCHANGE_PROVIDER" envDefault:"exchangerate.host"`
	ExchangeAPIKey string `env:"EXCHANGE_API_KEY" envDefault:""`
	// BCB / PTAX provider specific settings
	BCBAPIBaseURL  string        `env:"BCB_API_BASE_URL" envDefault:"https://olinda.bcb.gov.br/olinda/servico/PTAX/versao/v1/odata/"`
	BCBTimeout     time.Duration `env:"BCB_TIMEOUT_SECONDS" envDefault:"10s"`
	BCBMaxRetries  int           `env:"BCB_MAX_RETRIES" envDefault:"3"`
	BCBMaxBackDays int           `env:"BCB_MAX_BACK_DAYS" envDefault:"0"`
	// Logger configuration
	LogFormat     string `env:"LOG_FORMAT" envDefault:"text"` // text or json
	LogLevel      string `env:"LOG_LEVEL" envDefault:"info"`
	OTelCollector string `env:"OTEL_COLLECTOR_URL" envDefault:""` // optional OTEL collector endpoint
	// Advanced OTLP options
	OTLPEndpoint string `env:"OTLP_ENDPOINT" envDefault:""` // explicit OTLP endpoint (overrides OTEL_COLLECTOR_URL)
	OTLPHeaders  string `env:"OTLP_HEADERS" envDefault:""`  // comma-separated headers KEY=VALUE
	OTLPUseTLS   bool   `env:"OTLP_USE_TLS" envDefault:"false"`
	// TLS customization for OTLP exporters (optional)
	OTLPTLSCAPath          string `env:"OTLP_TLS_CA_PATH" envDefault:""`
	OTLPTLSCertPath        string `env:"OTLP_TLS_CERT_PATH" envDefault:""`
	OTLPTLSKeyPath         string `env:"OTLP_TLS_KEY_PATH" envDefault:""`
	OTLPInsecureSkipVerify bool   `env:"OTLP_INSECURE_SKIP_VERIFY" envDefault:"false"`
	// Application environment (development, staging, production, etc)
	Environment string `env:"ENVIRONMENT" envDefault:"development"`
	// Service identification
	AppName    string `env:"APP_NAME" envDefault:"go-exchange"`
	AppVersion string `env:"APP_VERSION" envDefault:"1.0.0"`
	AppEnv     string `env:"APP_ENV" envDefault:"development"`
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
