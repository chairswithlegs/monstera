package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"

	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/observability"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	slog.SetDefault(observability.NewLogger(cfg.AppEnv, cfg.LogLevel))
	metrics := observability.NewMetrics(prometheus.NewRegistry())
	observability.SetMetrics(metrics)

	infra, cleanup, err := setupInfra(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	svcs := createServices(cfg, infra)
	sched := registerSchedulerJobs(svcs, infra)

	workers := buildWorkers(cfg, svcs, infra, metrics, sched)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	startWorkers(workerCtx, workers.list)
	defer workerCancel()

	handler := createRouter(cfg, svcs, infra, workers.sseHub)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.AppPort),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	slog.Info("server ready", slog.Int("port", cfg.AppPort))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	workerCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown", slog.Any("error", err))
	}
	slog.Info("server stopped")
	return nil
}
