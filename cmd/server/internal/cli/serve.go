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
	"github.com/chairswithlegs/monstera/internal/activitypub/blocklist"
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
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/events/sse"
	"github.com/chairswithlegs/monstera/internal/media"
	_ "github.com/chairswithlegs/monstera/internal/media/local"
	_ "github.com/chairswithlegs/monstera/internal/media/s3"
	natsinternal "github.com/chairswithlegs/monstera/internal/nats"
	"github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/observability"
	"github.com/chairswithlegs/monstera/internal/scheduler"
	schedulerjobs "github.com/chairswithlegs/monstera/internal/scheduler/jobs"
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

	monsteraUIHost := ""
	if cfg.MonsteraUiUrl != nil {
		monsteraUIHost = cfg.MonsteraUiUrl.Host
	}

	// Initialize logger
	logger := observability.NewLogger(cfg.AppEnv, cfg.LogLevel)
	slog.SetDefault(logger)

	// Initialize metrics
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	observability.SetMetrics(metrics)

	// Setup database and run migrations
	pool, err := pgxpool.New(ctx, store.DatabaseConnectionString(cfg, true))
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}

	// Run migrations (TODO: determine if this should be removed)
	if err := store.RunUp(store.DatabaseConnectionString(cfg, false)); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Setup storage layer
	s := postgres.New(pool)

	cacheStore, err := cache.New(cache.Config{
		Driver: cfg.CacheDriver,
	})
	if err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	defer func() { _ = cacheStore.Close() }()

	blocklistCache := blocklist.NewBlocklistCache(s)
	if err := blocklistCache.Refresh(ctx); err != nil {
		slog.WarnContext(ctx, "blocklist refresh failed", slog.Any("error", err))
	}

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

	// Setup NATS
	natsClient, err := natsinternal.New(cfg)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer natsClient.Close()
	if err := ap.CreateOrUpdateStreams(ctx, natsClient.JS); err != nil {
		return fmt.Errorf("FederationSubscriber: create or update streams: %w", err)
	}
	if err := scheduler.CreateOrUpdateStreams(ctx, natsClient.JS); err != nil {
		return fmt.Errorf("scheduler: setup streams: %w", err)
	}
	if err := events.CreateOrUpdateStreams(ctx, natsClient.JS); err != nil {
		return fmt.Errorf("outbox: setup streams: %w", err)
	}
	eventPoller := events.NewPoller(s, natsClient.JS, events.PollerConfig{
		PollInterval: 500 * time.Millisecond,
		BatchSize:    100,
	})

	// Setup email
	emailSender, err := email.New(email.Config{
		Driver:       cfg.EmailDriver,
		From:         cfg.EmailFrom,
		FromName:     cfg.EmailFromName,
		SMTPHost:     cfg.EmailSMTPHost,
		SMTPPort:     cfg.EmailSMTPPort,
		SMTPUsername: cfg.EmailSMTPUsername,
		SMTPPassword: cfg.EmailSMTPPassword,
	})
	if err != nil {
		return fmt.Errorf("email: %w", err)
	}
	emailTemplates, err := email.NewTemplates()
	if err != nil {
		return fmt.Errorf("email templates: %w", err)
	}
	registrationMailer := service.NewRegistrationEmailSender(emailSender, emailTemplates, cfg.EmailFrom, cfg.EmailFromName)

	// Setup domain services
	instanceBaseURL := "https://" + cfg.InstanceDomain
	accountSvc := service.NewAccountService(s, instanceBaseURL)
	statusSvc := service.NewStatusService(s, instanceBaseURL, cfg.InstanceDomain, cfg.MaxStatusChars)
	timelineSvc := service.NewTimelineService(s, accountSvc, statusSvc)
	conversationSvc := service.NewConversationService(s, statusSvc)
	statusWriteSvc := service.NewStatusWriteService(s, statusSvc, conversationSvc, instanceBaseURL, cfg.InstanceDomain, cfg.MaxStatusChars)
	instanceSvc := service.NewInstanceService(s)
	followSvc := service.NewFollowService(s, accountSvc)
	notificationSvc := service.NewNotificationService(s)
	mediaSvc := service.NewMediaService(s, mediaStore, cfg.MediaMaxBytes)
	remoteResolver := ap.NewRemoteAccountResolver(accountSvc, cfg)
	searchSvc := service.NewSearchService(s, remoteResolver, logger)
	trendsSvc := service.NewTrendsService(s)
	cardSvc := service.NewCardService(s)
	authSvc := service.NewAuthService(s, monsteraUIHost, oauth.MONSTERA_UI_APPLICATION_ID)
	monsteraSettingsSvc := service.NewMonsteraSettingsService(s)
	moderationSvc := service.NewModerationService(s)
	listSvc := service.NewListService(s)
	userFilterSvc := service.NewUserFilterService(s)
	markerSvc := service.NewMarkerService(s)
	featuredTagSvc := service.NewFeaturedTagService(s)
	registrationSvc := service.NewRegistrationService(s, registrationMailer, registrationMailer, instanceBaseURL, cfg.InstanceName)
	serverFilterSvc := service.NewServerFilterService(s)
	announcementSvc := service.NewAnnouncementService(s)

	// Setup scheduled jobs
	sched := scheduler.New(natsClient.JS)
	sched.Register(scheduler.Job{
		Name:     "scheduled-statuses",
		Interval: time.Minute,
		Handler:  schedulerjobs.ScheduledStatuses(statusWriteSvc),
	})
	sched.Register(scheduler.Job{
		Name:     "update-trending",
		Interval: 15 * time.Minute,
		Handler:  schedulerjobs.UpdateTrendingIndexes(trendsSvc),
	})
	sched.Register(scheduler.Job{
		Name:     "fetch-status-cards",
		Interval: time.Minute,
		Handler:  schedulerjobs.ProcessPendingCards(cardSvc),
	})
	sched.Register(scheduler.Job{
		Name:     "cleanup-outbox-events",
		Interval: time.Hour,
		Handler:  schedulerjobs.CleanupOutboxEvents(s, 24*time.Hour),
	})

	// Setup SSE
	sseHub := sse.NewHub(&natsConnAdapter{nc: natsClient.Conn}, metrics)
	sseSub := sse.NewSubscriber(natsClient.JS, natsClient.Conn, s, metrics, cfg.InstanceDomain)

	// Setup ActivityPub
	signatureService := ap.NewHTTPSignatureService(cfg, cacheStore, accountSvc)
	fedSub := ap.NewFederationSubscriber(natsClient.JS, followSvc, blocklistCache, signatureService, cfg)
	inboxProcessor := ap.NewInbox(
		accountSvc,
		followSvc,
		notificationSvc,
		statusSvc,
		statusWriteSvc,
		mediaSvc,
		remoteResolver,
		cacheStore,
		blocklistCache,
		cfg,
	)

	// Start the background workers
	workerCtx, workerCancel := context.WithCancel(context.Background())
	go func() { _ = eventPoller.Start(workerCtx) }()
	go func() { _ = fedSub.Start(workerCtx) }()
	go func() { _ = sseSub.Start(workerCtx) }()
	go func() { _ = sseHub.Start(workerCtx) }()
	go func() {
		if err := sched.Start(workerCtx); err != nil {
			slog.Error("scheduler: fatal", slog.Any("error", err))
		}
	}()
	defer workerCancel()

	// Setup handlers
	oauthServer := oauth.NewServer(s, cacheStore, logger)
	oauthHandler := oauthhandlers.NewHandler(oauthServer, authSvc, cfg)
	health := api.NewHealthChecker(pool, natsClient.Conn)
	accountsHandler := mastodon.NewAccountsHandler(accountSvc, followSvc, timelineSvc, monsteraSettingsSvc, mediaSvc, cfg.MediaMaxBytes, cfg.InstanceDomain)
	statusesHandler := mastodon.NewStatusesHandler(accountSvc, statusSvc, statusWriteSvc, cfg.InstanceDomain, cacheStore, nil)
	scheduledStatusesHandler := mastodon.NewScheduledStatusesHandler(statusSvc, statusWriteSvc)
	pollsHandler := mastodon.NewPollsHandler(statusSvc, statusWriteSvc)
	timelinesHandler := mastodon.NewTimelinesHandler(timelineSvc, cfg.InstanceDomain)
	instanceHandler := mastodon.NewInstanceHandler(cfg.InstanceDomain, cfg.InstanceName, cfg.MaxStatusChars, cfg.MediaMaxBytes, nil, instanceSvc)
	trendsHandler := mastodon.NewTrendsHandler(trendsSvc, cfg.InstanceDomain)
	conversationsHandler := mastodon.NewConversationsHandler(conversationSvc, cfg.InstanceDomain)
	suggestionsHandler := mastodon.NewSuggestionsHandler()
	notificationsHandler := mastodon.NewNotificationsHandler(notificationSvc, accountSvc, statusSvc, cfg.InstanceDomain)
	mediaHandler := mastodon.NewMediaHandler(mediaSvc)
	searchHandler := mastodon.NewSearchHandler(searchSvc, cfg.InstanceDomain)
	streamingHandler := mastodon.NewStreamingHandler(sseHub)
	webFingerHandler := activitypub.NewWebFingerHandler(accountSvc, cfg)
	nodeInfoPtrHandler := activitypub.NewNodeInfoPointerHandler(cfg)
	nodeInfoHandler := activitypub.NewNodeInfoHandler(instanceSvc, cfg)
	actorHandler := activitypub.NewActorHandler(accountSvc, cfg)
	collectionsHandler := activitypub.NewCollectionsHandler(accountSvc, statusSvc, cfg)
	outboxHandler := activitypub.NewOutbox(accountSvc, timelineSvc, cfg)
	inboxHandler := activitypub.NewInboxHandler(inboxProcessor, cacheStore, cfg, signatureService)
	userHandler := monstera.NewUserHandler(accountSvc)
	reportsHandler := mastodon.NewReportsHandler(moderationSvc, accountSvc, cfg.InstanceDomain)
	followRequestsHandler := mastodon.NewFollowRequestsHandler(followSvc, accountSvc, cfg.InstanceDomain)
	listsHandler := mastodon.NewListsHandler(listSvc, accountSvc, cfg.InstanceDomain)
	filtersHandler := mastodon.NewFiltersHandler(userFilterSvc)
	preferencesHandler := mastodon.NewPreferencesHandler(accountSvc)
	markersHandler := mastodon.NewMarkersHandler(markerSvc)
	featuredTagsHandler := mastodon.NewFeaturedTagsHandler(featuredTagSvc, accountSvc, cfg.InstanceDomain)
	announcementsHandler := mastodon.NewAnnouncementsHandler(announcementSvc)
	moderatorDashboard := monstera.NewModeratorDashboardHandler(instanceSvc, moderationSvc)
	adminUsers := monstera.NewAdminUsersHandler(accountSvc, moderationSvc)
	moderatorRegistrations := monstera.NewModeratorRegistrationsHandler(registrationSvc)
	moderatorInvites := monstera.NewModeratorInvitesHandler(registrationSvc, monsteraSettingsSvc)
	moderatorReports := monstera.NewModeratorReportsHandler(moderationSvc)
	adminFederation := monstera.NewAdminFederationHandler(instanceSvc, moderationSvc)
	moderatorContent := monstera.NewModeratorContentHandler(serverFilterSvc)
	adminSettings := monstera.NewAdminSettingsHandler(monsteraSettingsSvc)
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
		Trends:                 trendsHandler,
		Conversations:          conversationsHandler,
		Suggestions:            suggestionsHandler,
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

	slog.Info("server ready", slog.Int("port", cfg.AppPort))

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	workerCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown", slog.Any("error", err))
	}
	natsClient.Close()
	slog.Info("server stopped")
	return nil
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
