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
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"

	ap "github.com/chairswithlegs/monstera/internal/activitypub"
	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/activitypub"
	"github.com/chairswithlegs/monstera/internal/api/mastodon"
	"github.com/chairswithlegs/monstera/internal/api/monstera"
	oauthhandlers "github.com/chairswithlegs/monstera/internal/api/oauth"
	"github.com/chairswithlegs/monstera/internal/api/router"
	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/email"
	_ "github.com/chairswithlegs/monstera/internal/email/noop"
	_ "github.com/chairswithlegs/monstera/internal/email/smtp"
	"github.com/chairswithlegs/monstera/internal/events/sse"
	sseMastodon "github.com/chairswithlegs/monstera/internal/events/sse/mastodon"
	"github.com/chairswithlegs/monstera/internal/media"
	_ "github.com/chairswithlegs/monstera/internal/media/local"
	_ "github.com/chairswithlegs/monstera/internal/media/s3"
	natsinternal "github.com/chairswithlegs/monstera/internal/nats"
	"github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/observability"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/store/postgres"
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
	observability.SetMetrics(metrics)

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

	natsClient, err := natsinternal.New(cfg)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer natsClient.Close()

	cacheStore, err := cache.New(cache.Config{
		Driver: cfg.CacheDriver,
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

	var mediaFileServer http.Handler
	if h, ok := mediaStore.(http.Handler); ok {
		mediaFileServer = h
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
	emailTemplates, err := email.NewTemplates()
	if err != nil {
		return fmt.Errorf("email templates: %w", err)
	}
	registrationMailer := service.NewRegistrationEmailSender(emailSender, emailTemplates, cfg.EmailFrom, cfg.EmailFromName)

	instanceBaseURL := "https://" + cfg.InstanceDomain
	accountSvc := service.NewAccountService(s, instanceBaseURL)
	blocklistCache := ap.NewBlocklistCache(s)
	if err := blocklistCache.Refresh(ctx); err != nil {
		logger.Warn("blocklist refresh failed", slog.Any("error", err))
	}
	if err := ap.CreateOrUpdateStreams(ctx, natsClient.JS); err != nil {
		return fmt.Errorf("activitypub: create or update streams: %w", err)
	}

	hub := sse.NewHub(&natsConnAdapter{nc: natsClient.Conn}, metrics)
	eventBus := sseMastodon.NewPublisher(natsClient.Conn, s, metrics, logger, cfg.InstanceDomain)
	signatureService := ap.NewHTTPSignatureService(cfg, cacheStore, accountSvc)
	outbox := ap.NewOutbox(s, natsClient.JS, blocklistCache, signatureService, cfg)
	statusSvc := service.NewStatusService(s, outbox, eventBus, instanceBaseURL, cfg.InstanceDomain, cfg.MaxStatusChars, logger)
	timelineSvc := service.NewTimelineService(s, statusSvc)
	instanceSvc := service.NewInstanceService(s)
	followSvc := service.NewFollowService(s, outbox, nil)
	notificationSvc := service.NewNotificationService(s)
	mediaSvc := service.NewMediaService(s, mediaStore, cfg.MediaMaxBytes)
	remoteResolver := ap.NewRemoteAccountResolver(s, cfg.InstanceDomain, cfg)
	searchSvc := service.NewSearchService(s, remoteResolver, logger)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	go func() { _ = outbox.Start(workerCtx) }()
	go func() { _ = hub.Start(workerCtx) }()
	go runScheduledStatusWorker(workerCtx, s, statusSvc, logger)
	defer workerCancel()
	inboxProcessor := ap.NewInbox(
		accountSvc,
		followSvc,
		notificationSvc,
		statusSvc,
		mediaSvc,
		remoteResolver,
		cacheStore,
		blocklistCache,
		outbox,
		eventBus,
		eventBus,
		cfg,
	)

	// Setup handlers
	oauthServer := oauth.NewServer(s, cacheStore, logger)
	oauthHandler := oauthhandlers.NewHandler(oauthServer, s, cfg)
	health := api.NewHealthChecker(pool, natsClient.Conn)
	accountsHandler := mastodon.NewAccountsHandler(accountSvc, followSvc, timelineSvc, cfg.InstanceDomain)
	statusesHandler := mastodon.NewStatusesHandler(accountSvc, statusSvc, cfg.InstanceDomain, cacheStore, nil)
	scheduledStatusesHandler := mastodon.NewScheduledStatusesHandler(statusSvc)
	pollsHandler := mastodon.NewPollsHandler(statusSvc)
	timelinesHandler := mastodon.NewTimelinesHandler(timelineSvc, cfg.InstanceDomain)
	instanceHandler := mastodon.NewInstanceHandler(cfg.InstanceDomain, cfg.InstanceName, cfg.MaxStatusChars, cfg.MediaMaxBytes, nil, instanceSvc)
	notificationsHandler := mastodon.NewNotificationsHandler(notificationSvc, accountSvc, cfg.InstanceDomain)
	mediaHandler := mastodon.NewMediaHandler(mediaSvc)
	searchHandler := mastodon.NewSearchHandler(searchSvc, cfg.InstanceDomain)
	streamingHandler := mastodon.NewStreamingHandler(hub)
	webFingerHandler := activitypub.NewWebFingerHandler(accountSvc, cfg)
	nodeInfoPtrHandler := activitypub.NewNodeInfoPointerHandler(cfg)
	nodeInfoHandler := activitypub.NewNodeInfoHandler(instanceSvc, cfg)
	actorHandler := activitypub.NewActorHandler(accountSvc, cfg)
	collectionsHandler := activitypub.NewCollectionsHandler(accountSvc, statusSvc, cfg)
	outboxHandler := activitypub.NewOutbox(accountSvc, timelineSvc, cfg)
	inboxHandler := activitypub.NewInboxHandler(inboxProcessor, cacheStore, cfg, signatureService)
	userHandler := monstera.NewUserHandler(accountSvc)

	moderationSvc := service.NewModerationService(s)
	reportsHandler := mastodon.NewReportsHandler(moderationSvc, accountSvc, cfg.InstanceDomain)
	followRequestsHandler := mastodon.NewFollowRequestsHandler(followSvc, accountSvc, cfg.InstanceDomain)
	listSvc := service.NewListService(s)
	listsHandler := mastodon.NewListsHandler(listSvc, accountSvc, cfg.InstanceDomain)
	userFilterSvc := service.NewUserFilterService(s)
	filtersHandler := mastodon.NewFiltersHandler(userFilterSvc)
	preferencesHandler := mastodon.NewPreferencesHandler(accountSvc)
	markerSvc := service.NewMarkerService(s)
	markersHandler := mastodon.NewMarkersHandler(markerSvc)
	featuredTagSvc := service.NewFeaturedTagService(s)
	featuredTagsHandler := mastodon.NewFeaturedTagsHandler(featuredTagSvc, accountSvc, cfg.InstanceDomain)
	announcementSvc := service.NewAnnouncementService(s)
	announcementsHandler := mastodon.NewAnnouncementsHandler(announcementSvc)
	registrationSvc := service.NewRegistrationService(s, registrationMailer, registrationMailer, instanceBaseURL, cfg.InstanceName)
	serverFilterSvc := service.NewServerFilterService(s)
	moderatorDashboard := monstera.NewModeratorDashboardHandler(instanceSvc, moderationSvc)
	adminUsers := monstera.NewAdminUsersHandler(accountSvc, moderationSvc)
	moderatorRegistrations := monstera.NewModeratorRegistrationsHandler(registrationSvc)
	moderatorInvites := monstera.NewModeratorInvitesHandler(registrationSvc)
	moderatorReports := monstera.NewModeratorReportsHandler(moderationSvc)
	adminFederation := monstera.NewAdminFederationHandler(instanceSvc, moderationSvc)
	moderatorContent := monstera.NewModeratorContentHandler(serverFilterSvc)
	adminSettings := monstera.NewAdminSettingsHandler(instanceSvc)
	adminAnnouncements := monstera.NewAdminAnnouncementsHandler(announcementSvc)

	handler := router.New(router.Deps{
		AccountsService:        accountSvc,
		Health:                 health,
		MediaFileServer:        mediaFileServer,
		OAuthHandler:           oauthHandler,
		OAuthServer:            oauthServer,
		Accounts:               accountsHandler,
		Statuses:               statusesHandler,
		ScheduledStatuses:      scheduledStatusesHandler,
		Polls:                  pollsHandler,
		Timelines:              timelinesHandler,
		Instance:               instanceHandler,
		Notifications:          notificationsHandler,
		Media:                  mediaHandler,
		Search:                 searchHandler,
		Streaming:              streamingHandler,
		Reports:                reportsHandler,
		FollowRequests:         followRequestsHandler,
		Lists:                  listsHandler,
		Filters:                filtersHandler,
		Preferences:            preferencesHandler,
		Markers:                markersHandler,
		FeaturedTags:           featuredTagsHandler,
		Announcements:          announcementsHandler,
		WebFinger:              webFingerHandler,
		NodeInfoPtr:            nodeInfoPtrHandler,
		NodeInfo:               nodeInfoHandler,
		Actor:                  actorHandler,
		Collections:            collectionsHandler,
		Outbox:                 outboxHandler,
		Inbox:                  inboxHandler,
		User:                   userHandler,
		ModeratorDashboard:     moderatorDashboard,
		AdminUsers:             adminUsers,
		ModeratorRegistrations: moderatorRegistrations,
		ModeratorInvites:       moderatorInvites,
		ModeratorReports:       moderatorReports,
		AdminFederation:        adminFederation,
		ModeratorContent:       moderatorContent,
		AdminSettings:          adminSettings,
		AdminAnnouncements:     adminAnnouncements,
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

// runScheduledStatusWorker runs a ticker that publishes due scheduled statuses.
func runScheduledStatusWorker(ctx context.Context, st store.Store, statusSvc service.StatusService, logger *slog.Logger) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			due, err := st.ListScheduledStatusesDue(ctx, 20)
			if err != nil {
				if logger != nil {
					logger.WarnContext(ctx, "scheduled status worker list failed", slog.Any("error", err))
				}
				continue
			}
			for i := range due {
				if err := statusSvc.PublishScheduled(ctx, due[i].ID); err != nil && logger != nil {
					logger.WarnContext(ctx, "scheduled status publish failed", slog.String("id", due[i].ID), slog.Any("error", err))
				}
			}
		}
	}
}

// natsConnAdapter adapts *nats.Conn to sse's natsConn interface (Subscribe return type).
type natsConnAdapter struct {
	nc *nats.Conn
}

func (a *natsConnAdapter) Subscribe(subject string, handler nats.MsgHandler) (interface{ Unsubscribe() error }, error) {
	sub, err := a.nc.Subscribe(subject, handler)
	if err != nil {
		return nil, fmt.Errorf("nats subscribe %s: %w", subject, err)
	}
	return sub, nil
}
