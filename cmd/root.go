package cmd

import (
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
		lg := logger.New(logger.Options{Format: cfg.LogFormat, Level: cfg.LogLevel, Name: cfg.AppName})

		// register telemetry hooks / formatter helpers
		if err := lg.SetupTelemetry(cmd.Context(), cfg); err != nil {
			lg.WithContext(cmd.Context()).Errorf("setup telemetry error: %v", err)
		}

		// init OTLP (traces/metrics/logs) if collector configured
		//var shutdown func(context.Context) error

		shutdown, infos, err := lg.SetupOTel(cmd.Context(), cfg)
		if err != nil {
			lg.WithContext(cmd.Context()).Warnf("failed to setup otel: %v", err)
		} else {
			if len(infos) > 0 {
				for _, info := range infos {
					lg.WithContext(cmd.Context()).Infof("OTEL exporter: type=%s endpoint=%s insecure=%t headers=%v", info.Type, info.Endpoint, info.Insecure, info.Headers)
				}
			}
		}
		if shutdown != nil {
			defer func() { _ = shutdown(cmd.Context()) }()
		}

		s := server.New(cfg, lg)
		lg.WithContext(cmd.Context()).Infof("Starting server on %s", cfg.HTTPAddr)

		// if shutdown != nil {
		// 	defer shutdown(cmd.Context())
		// }

		return s.Run()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
