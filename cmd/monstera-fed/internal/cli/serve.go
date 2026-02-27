package cli

import (
	"context"
	"crypto/tls"
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

	ap "github.com/chairswithlegs/monstera-fed/internal/activitypub"
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
	ctx := context.Background()
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// Initialize logger
	logger := observability.NewLogger(cfg.AppEnv, cfg.LogLevel)
	slog.SetDefault(logger)

	// Initialize metrics
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)

	// Setup database and run migrations
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

	// Setup services and other dependencies
	s := postgres.New(pool)

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
	remoteResolver, actorFetchForInbox := setupFederationResolver(s, cfg)
	searchSvc := service.NewSearchService(s, remoteResolver, logger)

	oauthServer := oauth.NewServer(s, cacheStore, logger)
	loginTmpl, err := oauthhandlers.ParseLoginTemplate()
	if err != nil {
		return fmt.Errorf("oauth template: %w", err)
	}
	secretKey, err := cfg.SecretKeyBytes()
	if err != nil {
		return fmt.Errorf("secret key: %w", err)
	}

	blocklistCache := ap.NewBlocklistCache(s, logger)
	if err := blocklistCache.Refresh(ctx); err != nil {
		logger.Warn("blocklist refresh failed", slog.Any("error", err))
	}
	if err := nats.EnsureStreams(ctx, natsClient.JS); err != nil {
		return fmt.Errorf("nats: ensure streams: %w", err)
	}
	fedWorker := federation.NewFederationWorker(natsClient.JS, fedProducer, s, blocklistCache, cfg, logger, metrics)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	go func() {
		_ = fedWorker.Start(workerCtx)
	}()
	defer workerCancel()
	inboxProcessor := ap.NewInboxProcessor(s, cacheStore, blocklistCache, nil, outboxPublisher, cfg, logger, actorFetchForInbox)

	// Setup handlers and middleware
	oauthHandler := oauthhandlers.NewHandler(oauthServer, s, logger, loginTmpl, cfg.InstanceName, secretKey)
	health := api.NewHealthChecker(pool, natsClient.Conn)
	accountsHandler := mastodon.NewAccountsHandler(accountSvc, followSvc, cfg.InstanceDomain)
	statusesHandler := mastodon.NewStatusesHandler(accountSvc, statusSvc, cfg.InstanceDomain)
	timelinesHandler := mastodon.NewTimelinesHandler(timelineSvc, cfg.InstanceDomain)
	instanceHandler := mastodon.NewInstanceHandler(cfg.InstanceDomain, cfg.InstanceName, cfg.MaxStatusChars, cfg.MediaMaxBytes, nil)
	notificationsHandler := mastodon.NewNotificationsHandler(notificationSvc, accountSvc, cfg.InstanceDomain)
	mediaHandler := mastodon.NewMediaHandler(mediaSvc)
	searchHandler := mastodon.NewSearchHandler(searchSvc, cfg.InstanceDomain)
	webFingerHandler := activitypub.NewWebFingerHandler(accountSvc, cfg)
	nodeInfoPtrHandler := activitypub.NewNodeInfoPointerHandler(cfg)
	nodeInfoHandler := activitypub.NewNodeInfoHandler(instanceSvc, cfg)
	actorHandler := activitypub.NewActorHandler(accountSvc, cfg)
	collectionsHandler := activitypub.NewCollectionsHandler(accountSvc, cfg)
	outboxHandler := activitypub.NewOutbox(accountSvc, timelineSvc, cfg)
	inboxHandler := activitypub.NewInboxHandler(inboxProcessor, cacheStore)

	handler := router.New(router.Deps{
		AccountsService: accountSvc,
		Metrics:         metrics,
		Health:          health,
		OAuthHandler:    oauthHandler,
		OAuthServer:     oauthServer,
		Accounts:        accountsHandler,
		Statuses:        statusesHandler,
		Timelines:       timelinesHandler,
		Instance:        instanceHandler,
		Notifications:   notificationsHandler,
		Media:           mediaHandler,
		Search:          searchHandler,
		WebFinger:       webFingerHandler,
		NodeInfoPtr:     nodeInfoPtrHandler,
		NodeInfo:        nodeInfoHandler,
		Actor:           actorHandler,
		Collections:     collectionsHandler,
		Outbox:          outboxHandler,
		Inbox:           inboxHandler,
	})

	// Start HTTP server
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

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	workerCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown", slog.Any("error", err))
	}
	natsClient.Close()
	logger.Info("server stopped")
	return nil
}

func setupFederationResolver(s store.Store, cfg *config.Config) (*ap.RemoteAccountResolver, func(context.Context, string) (*ap.Actor, error)) {
	if !cfg.FederationInsecureSkipTLS {
		actorFetch := ap.ActorFetch
		return ap.NewRemoteAccountResolver(s, actorFetch, cfg.InstanceDomain, nil), actorFetch
	}
	fedClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			//nolint:gosec // G402: intentional for development federation with self-signed certs
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	actorFetchForInbox := ap.ActorFetchWithClient(fedClient)
	remoteResolver := ap.NewRemoteAccountResolver(s, actorFetchForInbox, cfg.InstanceDomain, fedClient)
	return remoteResolver, actorFetchForInbox
}
