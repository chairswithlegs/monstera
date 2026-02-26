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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"

	"github.com/chairswithlegs/monstera-fed/internal/ap"
	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon"
	oauthhandlers "github.com/chairswithlegs/monstera-fed/internal/api/oauth"
	"github.com/chairswithlegs/monstera-fed/internal/api/router"
	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/email"
	_ "github.com/chairswithlegs/monstera-fed/internal/email/noop"
	_ "github.com/chairswithlegs/monstera-fed/internal/email/smtp"
	"github.com/chairswithlegs/monstera-fed/internal/media"
	_ "github.com/chairswithlegs/monstera-fed/internal/media/local"
	_ "github.com/chairswithlegs/monstera-fed/internal/media/s3"
	"github.com/chairswithlegs/monstera-fed/internal/nats"
	"github.com/chairswithlegs/monstera-fed/internal/nats/federation"
	"github.com/chairswithlegs/monstera-fed/internal/oauth"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/store/postgres"
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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := observability.NewLogger(cfg.AppEnv, cfg.LogLevel)
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}

	if err := store.RunUp(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	natsClient, err := nats.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer natsClient.Close()

	cacheStore, err := cache.New(cache.Config{
		Driver:   cfg.CacheDriver,
		RedisURL: cfg.CacheRedisURL,
		Logger:   logger,
	})
	if err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	defer func() { _ = cacheStore.Close() }()

	mediaStore, err := media.New(media.Config{
		Driver:     cfg.MediaDriver,
		LocalPath:  cfg.MediaLocalPath,
		BaseURL:    cfg.MediaBaseURL,
		S3Bucket:   cfg.MediaS3Bucket,
		S3Region:   cfg.MediaS3Region,
		S3Endpoint: cfg.MediaS3Endpoint,
		CDNBase:    cfg.MediaCDNBase,
		MaxBytes:   cfg.MediaMaxBytes,
	})
	if err != nil {
		return fmt.Errorf("media: %w", err)
	}

	emailSender, err := email.New(email.Config{
		Driver:       cfg.EmailDriver,
		From:         cfg.EmailFrom,
		FromName:     cfg.EmailFromName,
		SMTPHost:     cfg.EmailSMTPHost,
		SMTPPort:     cfg.EmailSMTPPort,
		SMTPUsername: cfg.EmailSMTPUsername,
		SMTPPassword: cfg.EmailSMTPPassword,
		Logger:       logger,
	})
	if err != nil {
		return fmt.Errorf("email: %w", err)
	}
	_ = emailSender

	s := postgres.New(pool)
	instanceBaseURL := "https://" + cfg.InstanceDomain
	accountSvc := service.NewAccountService(s, instanceBaseURL)
	fedProducer := federation.NewProducer(natsClient.JS, metrics)
	outboxPublisher := ap.NewOutboxPublisher(s, fedProducer, cfg, logger)
	statusSvc := service.NewStatusService(s, outboxPublisher, instanceBaseURL, cfg.InstanceDomain, cfg.MaxStatusChars, logger)
	timelineSvc := service.NewTimelineService(s)
	instanceSvc := service.NewInstanceService(s)
	followSvc := service.NewFollowService(s, outboxPublisher)
	notificationSvc := service.NewNotificationService(s)
	mediaSvc := service.NewMediaService(s, mediaStore, cfg.MediaMaxBytes)

	oauthServer := oauth.NewServer(s, cacheStore, logger)
	loginTmpl, err := oauthhandlers.ParseLoginTemplate()
	if err != nil {
		return fmt.Errorf("oauth template: %w", err)
	}
	secretKey, err := cfg.SecretKeyBytes()
	if err != nil {
		return fmt.Errorf("secret key: %w", err)
	}
	oauthHandler := oauthhandlers.NewHandler(oauthServer, s, logger, loginTmpl, cfg.InstanceName, secretKey)

	health := api.NewHealthChecker(pool, natsClient.Conn)
	accountsHandler := mastodon.NewAccountsHandler(accountSvc, followSvc, logger, cfg.InstanceDomain)
	statusesHandler := mastodon.NewStatusesHandler(statusSvc, accountSvc, logger, cfg.InstanceDomain)
	timelinesHandler := mastodon.NewTimelinesHandler(timelineSvc, logger, cfg.InstanceDomain)
	instanceHandler := mastodon.NewInstanceHandler(
		cfg.InstanceDomain,
		cfg.InstanceName,
		cfg.MaxStatusChars,
		cfg.MediaMaxBytes,
		nil,
		logger,
	)
	notificationsHandler := mastodon.NewNotificationsHandler(notificationSvc, accountSvc, logger, cfg.InstanceDomain)
	mediaHandler := mastodon.NewMediaHandler(mediaSvc, logger)

	blocklistCache := ap.NewBlocklistCache(s, logger)
	if err := blocklistCache.Refresh(ctx); err != nil {
		logger.Warn("blocklist refresh failed", slog.Any("error", err))
	}
	inboxProcessor := ap.NewInboxProcessor(s, cacheStore, blocklistCache, nil, nil, cfg, logger, ap.DefaultActorFetch)
	apDeps := activitypub.Deps{
		Accounts:  accountSvc,
		Timelines: timelineSvc,
		Instance:  instanceSvc,
		Cache:     cacheStore,
		Config:    cfg,
		Logger:    logger,
		Inbox:     inboxProcessor,
	}
	handler := router.New(router.Deps{
		Logger:        logger,
		Metrics:       metrics,
		Health:        health,
		OAuthHandler:  oauthHandler,
		OAuthServer:   oauthServer,
		Store:         s,
		Accounts:      accountsHandler,
		Statuses:      statusesHandler,
		Timelines:     timelinesHandler,
		Instance:      instanceHandler,
		Notifications: notificationsHandler,
		Media:         mediaHandler,
		WebFinger:     activitypub.NewWebFingerHandler(apDeps),
		NodeInfoPtr:   activitypub.NewNodeInfoPointerHandler(apDeps),
		NodeInfo:      activitypub.NewNodeInfoHandler(apDeps),
		Actor:         activitypub.NewActorHandler(apDeps),
		Collections:   activitypub.NewCollectionsHandler(apDeps),
		Outbox:        activitypub.NewOutboxHandler(apDeps),
		Inbox:         activitypub.NewInboxHandler(apDeps),
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.AppPort),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	logger.Info("server ready", slog.Int("port", cfg.AppPort))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown", slog.Any("error", err))
	}
	natsClient.Close()
	logger.Info("server stopped")
	return nil
}
