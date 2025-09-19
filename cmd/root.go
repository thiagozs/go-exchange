package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/thiagozs/go-exchange/internal/config"
	"github.com/thiagozs/go-exchange/internal/logger"
	"github.com/thiagozs/go-exchange/internal/server"
)

var rootCmd = &cobra.Command{
	Use:   "go-exchange",
	Short: "Motor de consulta de cotações",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		// initialize logger
		lg := logger.New(logger.Options{Format: cfg.LogFormat, Level: cfg.LogLevel})
		// register telemetry hooks / formatter helpers
		if err := lg.SetupTelemetry(cmd.Context()); err != nil {
			lg.WithContext(cmd.Context()).Errorf("setup telemetry error: %v", err)
		}

		// init tracer (OTLP exporter) if collector configured
		var shutdown func(context.Context) error
		if cfg.OTelCollector != "" {
			sd, err := lg.InitTracer(cmd.Context(), cfg.OTelCollector)
			if err != nil {
				lg.WithContext(cmd.Context()).Errorf("otel init error: %v", err)
			} else {
				shutdown = sd
			}
		}
		s := server.New(cfg, lg)
		lg.WithContext(cmd.Context()).Infof("Starting server on %s", cfg.HTTPAddr)
		// ensure tracer shutdown when command/context ends
		if shutdown != nil {
			defer shutdown(cmd.Context())
		}
		return s.Run()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
