package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ap "github.com/chairswithlegs/monstera/internal/activitypub"
	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/activitypub"
	"github.com/chairswithlegs/monstera/internal/api/mastodon"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/sse"
	"github.com/chairswithlegs/monstera/internal/api/monstera"
	oauthhandlers "github.com/chairswithlegs/monstera/internal/api/oauth"
	"github.com/chairswithlegs/monstera/internal/api/router"
	"github.com/chairswithlegs/monstera/internal/blocklist"
	"github.com/chairswithlegs/monstera/internal/cache"
	natscache "github.com/chairswithlegs/monstera/internal/cache/nats"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/email"
	_ "github.com/chairswithlegs/monstera/internal/email/noop"
	_ "github.com/chairswithlegs/monstera/internal/email/smtp"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/media"
	_ "github.com/chairswithlegs/monstera/internal/media/local"
	_ "github.com/chairswithlegs/monstera/internal/media/s3"
	"github.com/chairswithlegs/monstera/internal/natsutil"
	"github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/observability"
	"github.com/chairswithlegs/monstera/internal/ratelimit"
	"github.com/chairswithlegs/monstera/internal/scheduler"
	schedulerjobs "github.com/chairswithlegs/monstera/internal/scheduler/jobs"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/store/postgres"
	"github.com/chairswithlegs/monstera/internal/webpush"
)

// infra bundles infrastructure dependencies created during startup.
type infra struct {
	pool            *pgxpool.Pool
	store           store.Store
	cache           cache.Store
	sharedCache     cache.SharedStore
	blocklist       *blocklist.BlocklistCache
	mediaStore      media.MediaStore
	mediaFileServer http.Handler
	nats            *natsutil.Client
	eventPoller     *events.Poller
	emailSender     email.Sender
	emailTemplates  *email.Templates
}

// svcs bundles all domain and subsystem services.
type svcs struct {
	account           service.AccountService
	statusRead        service.StatusService
	statusWrite       service.StatusWriteService
	statusInteraction service.StatusInteractionService
	remoteStatusWrite service.RemoteStatusWriteService
	scheduled         service.ScheduledStatusService
	timeline          service.TimelineService
	conversation      service.ConversationService
	instance          service.InstanceService
	follow            service.FollowService
	remoteFollow      service.RemoteFollowService
	tagFollow         service.TagFollowService
	notification      service.NotificationService
	media             service.MediaService
	search            service.SearchService
	trends            service.TrendsService
	card              service.CardService
	auth              service.AuthService
	monsteraSettings  service.MonsteraSettingsService
	moderation        service.ModerationService
	list              service.ListService
	userFilter        service.UserFilterService
	marker            service.MarkerService
	featuredTag       service.FeaturedTagService
	registration      service.RegistrationService
	serverFilter      service.ServerFilterService
	announcement      service.AnnouncementService
	pushSubscription  service.PushSubscriptionService
	backfill          service.BackfillService

	oauthServer      *oauth.Server
	remoteResolver   *ap.RemoteAccountResolver
	signatureService ap.HTTPSignatureService
	inboxProcessor   ap.Inbox
}

func setupInfra(ctx context.Context, cfg *config.Config) (*infra, func(), error) {
	pool, err := pgxpool.New(ctx, store.DatabaseConnectionString(cfg, true))
	if err != nil {
		return nil, nil, fmt.Errorf("database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("database ping: %w", err)
	}
	if err := store.RunUp(store.DatabaseConnectionString(cfg, false)); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	s := postgres.New(pool)

	cacheStore, err := cache.New(cache.Config{Driver: cfg.CacheDriver})
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("cache: %w", err)
	}

	bl := blocklist.NewBlocklistCache(s)
	if err := bl.Refresh(ctx); err != nil {
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
		_ = cacheStore.Close()
		pool.Close()
		return nil, nil, fmt.Errorf("media: %w", err)
	}

	var mediaFileServer http.Handler
	if h, ok := mediaStore.(http.Handler); ok {
		mediaFileServer = h
	}

	natsClient, err := natsutil.New(cfg)
	if err != nil {
		_ = cacheStore.Close()
		pool.Close()
		return nil, nil, fmt.Errorf("nats: %w", err)
	}
	allStreams := append(append(ap.StreamConfigs, events.StreamConfigs...), scheduler.StreamConfigs...)
	if err := natsutil.EnsureStreams(ctx, natsClient.JS, allStreams...); err != nil {
		natsClient.Close()
		_ = cacheStore.Close()
		pool.Close()
		return nil, nil, fmt.Errorf("nats: setup streams: %w", err)
	}

	sharedCache, err := natscache.New(ctx, natsClient.JS, 24*time.Hour)
	if err != nil {
		natsClient.Close()
		_ = cacheStore.Close()
		pool.Close()
		return nil, nil, fmt.Errorf("shared cache: %w", err)
	}

	poller := events.NewPoller(s, natsClient.JS, events.PollerConfig{
		PollInterval: 500 * time.Millisecond,
		BatchSize:    100,
	})

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
		natsClient.Close()
		_ = cacheStore.Close()
		pool.Close()
		return nil, nil, fmt.Errorf("email: %w", err)
	}
	emailTemplates, err := email.NewTemplates()
	if err != nil {
		natsClient.Close()
		_ = cacheStore.Close()
		pool.Close()
		return nil, nil, fmt.Errorf("email templates: %w", err)
	}

	i := &infra{
		pool:            pool,
		store:           s,
		cache:           cacheStore,
		sharedCache:     sharedCache,
		blocklist:       bl,
		mediaStore:      mediaStore,
		mediaFileServer: mediaFileServer,
		nats:            natsClient,
		eventPoller:     poller,
		emailSender:     emailSender,
		emailTemplates:  emailTemplates,
	}

	cleanup := func() {
		_ = sharedCache.Close()
		natsClient.Close()
		_ = cacheStore.Close()
		pool.Close()
	}

	return i, cleanup, nil
}

func createServices(cfg *config.Config, i *infra) *svcs {
	instanceBaseURL := cfg.InstanceBaseURL()
	monsteraUIHost := ""
	if cfg.MonsteraUIURL != nil {
		monsteraUIHost = cfg.MonsteraUIURL.Host
	}

	accountSvc := service.NewAccountService(i.store, instanceBaseURL)
	statusSvc := service.NewStatusService(i.store, instanceBaseURL, cfg.MonsteraInstanceDomain, cfg.MaxStatusChars)
	conversationSvc := service.NewConversationService(i.store, statusSvc)
	remoteFollowSvc := service.NewRemoteFollowService(i.store)
	followSvc := service.NewFollowService(i.store, accountSvc, remoteFollowSvc)
	tagFollowSvc := service.NewTagFollowService(i.store)
	mediaSvc := service.NewMediaService(i.store, i.mediaStore, cfg.MediaMaxBytes)

	mailer := service.NewRegistrationEmailSender(i.emailSender, i.emailTemplates, cfg.EmailFrom, cfg.EmailFromName)
	backfillSvc := service.NewBackfillService(i.store, i.nats.JS, cfg.BackfillCooldown)
	remoteResolver := ap.NewRemoteAccountResolver(accountSvc, cfg.AppEnv, cfg.FederationInsecureSkipTLS, cfg.MonsteraInstanceDomain)
	signatureService := ap.NewHTTPSignatureService(cfg.FederationInsecureSkipTLS, instanceBaseURL, i.sharedCache, i.cache, accountSvc)

	statusWriteSvc := service.NewStatusWriteService(i.store, statusSvc, conversationSvc, instanceBaseURL, cfg.MonsteraInstanceDomain, cfg.MaxStatusChars)
	interactionSvc := service.NewStatusInteractionService(i.store, statusSvc, instanceBaseURL)
	remoteStatusWriteSvc := service.NewRemoteStatusWriteService(i.store, conversationSvc, mediaSvc, instanceBaseURL)
	scheduledSvc := service.NewScheduledStatusService(i.store, statusWriteSvc)

	return &svcs{
		account:           accountSvc,
		statusRead:        statusSvc,
		statusWrite:       statusWriteSvc,
		statusInteraction: interactionSvc,
		remoteStatusWrite: remoteStatusWriteSvc,
		scheduled:         scheduledSvc,
		timeline:          service.NewTimelineService(i.store, accountSvc, statusSvc),
		conversation:      conversationSvc,
		instance:          service.NewInstanceService(i.store),
		follow:            followSvc,
		remoteFollow:      remoteFollowSvc,
		tagFollow:         tagFollowSvc,
		notification:      service.NewNotificationService(i.store),
		media:             mediaSvc,
		search:            service.NewSearchService(i.store, remoteResolver, backfillSvc),
		backfill:          backfillSvc,
		trends:            service.NewTrendsService(i.store, statusSvc),
		card:              service.NewCardService(i.store),
		auth:              service.NewAuthService(i.store, monsteraUIHost, oauth.MONSTERA_UI_APPLICATION_ID),
		monsteraSettings:  service.NewMonsteraSettingsService(i.store),
		moderation:        service.NewModerationService(i.store),
		list:              service.NewListService(i.store),
		userFilter:        service.NewUserFilterService(i.store),
		marker:            service.NewMarkerService(i.store),
		featuredTag:       service.NewFeaturedTagService(i.store),
		registration:      service.NewRegistrationService(i.store, mailer, mailer, instanceBaseURL, cfg.InstanceName),
		serverFilter:      service.NewServerFilterService(i.store),
		announcement:      service.NewAnnouncementService(i.store),
		pushSubscription:  service.NewPushSubscriptionService(i.store),

		oauthServer:      oauth.NewServer(i.store, i.sharedCache, cfg.VAPIDPublicKey),
		remoteResolver:   remoteResolver,
		signatureService: signatureService,
		inboxProcessor:   ap.NewInbox(accountSvc, followSvc, remoteFollowSvc, statusSvc, remoteStatusWriteSvc, mediaSvc, remoteResolver, i.blocklist, cfg.MonsteraInstanceDomain),
	}
}

func registerSchedulerJobs(s *svcs, i *infra) scheduler.Scheduler {
	sched := scheduler.New(i.nats.JS)
	sched.Register(scheduler.Job{
		Name:     "scheduled-statuses",
		Interval: time.Minute,
		Handler:  schedulerjobs.ScheduledStatuses(s.scheduled),
	})
	sched.Register(scheduler.Job{
		Name:     "update-trending",
		Interval: 15 * time.Minute,
		Handler:  schedulerjobs.UpdateTrendingIndexes(s.trends),
	})
	sched.Register(scheduler.Job{
		Name:     "fetch-status-cards",
		Interval: time.Minute,
		Handler:  schedulerjobs.ProcessPendingCards(s.card),
	})
	sched.Register(scheduler.Job{
		Name:     "cleanup-outbox-events",
		Interval: time.Hour,
		Handler:  schedulerjobs.CleanupOutboxEvents(i.store, 24*time.Hour),
	})
	return sched
}

type namedWorker struct {
	name    string
	starter interface{ Start(context.Context) error }
}

// backgroundWorkers holds named workers and any shared objects the router also needs.
type backgroundWorkers struct {
	list   []namedWorker
	sseHub *sse.Hub
}

func buildWorkers(cfg *config.Config, s *svcs, i *infra, metrics *observability.Metrics, sched scheduler.Scheduler) backgroundWorkers {
	instanceBaseURL := cfg.InstanceBaseURL()

	hub := sse.NewHub(natsutil.NewConnSubscriber(i.nats.Conn), metrics)
	sseSub := sse.NewSubscriber(i.nats.JS, i.nats.Conn, i.store, s.statusRead, cfg.MonsteraInstanceDomain)
	fedSub := ap.NewFederationSubscriber(i.nats.JS, s.remoteFollow, i.blocklist, s.signatureService,
		instanceBaseURL, cfg.MonsteraUIURL.String(), cfg.AppEnv, cfg.FederationInsecureSkipTLS, cfg.FederationWorkerConcurrency)
	notifSub := events.NewNotificationSubscriber(i.nats.JS, events.NotificationDeps{
		Notifications: s.notification,
		Accounts:      s.account,
		Conversations: s.statusRead,
	})

	backfillWorker := ap.NewBackfillWorker(i.nats.JS, s.account, s.backfill, s.remoteResolver,
		s.remoteStatusWrite, s.statusRead, cfg.MonsteraInstanceDomain, cfg.BackfillMaxPages, cfg.BackfillCooldown)

	workers := []namedWorker{
		{"event-poller", i.eventPoller},
		{"federation-subscriber", fedSub},
		{"sse-subscriber", sseSub},
		{"sse-hub", hub},
		{"notification-subscriber", notifSub},
		{"scheduler", sched},
		{"backfill-worker", backfillWorker},
	}

	if cfg.VAPIDPrivateKey != "" && cfg.VAPIDPublicKey != "" {
		pushSender := webpush.NewSender(cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, "mailto:admin@"+cfg.MonsteraInstanceDomain)
		pushSub := events.NewPushDeliverySubscriber(i.nats.JS, events.PushDeliveryDeps{
			PushSubs: s.pushSubscription,
			Deleter:  s.pushSubscription,
			Sender:   pushSender,
		})
		workers = append(workers, namedWorker{"push-delivery", pushSub})
	}

	return backgroundWorkers{
		sseHub: hub,
		list:   workers,
	}
}

func startWorkers(ctx context.Context, workers []namedWorker) {
	for _, w := range workers {
		go func(name string, s interface{ Start(context.Context) error }) {
			if err := s.Start(ctx); err != nil {
				slog.Error("worker failed", slog.String("name", name), slog.Any("error", err))
			}
		}(w.name, w.starter)
	}
}

func createRouter(cfg *config.Config, s *svcs, i *infra, sseHub *sse.Hub) http.Handler {
	instanceBaseURL := cfg.InstanceBaseURL()

	var rlCfg *router.RateLimitConfig
	if cfg.RateLimitAuthPerWindow > 0 || cfg.RateLimitPublicPerWindow > 0 {
		rlCfg = &router.RateLimitConfig{
			Limiter:      ratelimit.New(i.sharedCache),
			AuthLimit:    cfg.RateLimitAuthPerWindow,
			AuthWindow:   cfg.RateLimitAuthWindow,
			PublicLimit:  cfg.RateLimitPublicPerWindow,
			PublicWindow: cfg.RateLimitPublicWindow,
		}
	}

	return router.New(router.Deps{
		MaxRequestBodyBytes:    cfg.MaxRequestBodyBytes,
		MediaMaxBytes:          cfg.MediaMaxBytes,
		RateLimit:              rlCfg,
		AccountsService:        s.account,
		Health:                 api.NewHealthChecker(i.pool, i.nats.Conn),
		MediaFileServer:        i.mediaFileServer,
		OAuthHandler:           oauthhandlers.NewHandler(s.oauthServer, s.auth, cfg.MonsteraUIURL),
		OAuthServer:            s.oauthServer,
		Accounts:               mastodon.NewAccountsHandler(s.account, s.follow, s.tagFollow, s.timeline, s.statusRead, s.monsteraSettings, s.media, s.backfill, cfg.MediaMaxBytes, cfg.MonsteraInstanceDomain),
		Statuses:               mastodon.NewStatusesHandler(s.account, s.statusRead, s.statusWrite, s.statusInteraction, s.scheduled, s.conversation, cfg.MonsteraInstanceDomain, i.sharedCache, nil),
		ScheduledStatuses:      mastodon.NewScheduledStatusesHandler(s.statusRead, s.scheduled, cfg.MonsteraInstanceDomain),
		Polls:                  mastodon.NewPollsHandler(s.statusRead, s.statusInteraction),
		Timelines:              mastodon.NewTimelinesHandler(s.timeline, cfg.MonsteraInstanceDomain),
		Instance:               mastodon.NewInstanceHandler(cfg.MonsteraInstanceDomain, cfg.InstanceName, cfg.MaxStatusChars, cfg.MediaMaxBytes, nil, s.instance),
		Trends:                 mastodon.NewTrendsHandler(s.trends, cfg.MonsteraInstanceDomain),
		Conversations:          mastodon.NewConversationsHandler(s.conversation, cfg.MonsteraInstanceDomain),
		Suggestions:            mastodon.NewSuggestionsHandler(),
		Notifications:          mastodon.NewNotificationsHandler(s.notification, s.account, s.statusRead, cfg.MonsteraInstanceDomain),
		Media:                  mastodon.NewMediaHandler(s.media),
		Search:                 mastodon.NewSearchHandler(s.search, cfg.MonsteraInstanceDomain),
		Streaming:              mastodon.NewStreamingHandler(sseHub, s.list),
		Reports:                mastodon.NewReportsHandler(s.moderation, s.account, cfg.MonsteraInstanceDomain),
		FollowRequests:         mastodon.NewFollowRequestsHandler(s.follow, s.account, cfg.MonsteraInstanceDomain),
		Lists:                  mastodon.NewListsHandler(s.list, s.account, cfg.MonsteraInstanceDomain),
		Filters:                mastodon.NewFiltersHandler(s.userFilter),
		Preferences:            mastodon.NewPreferencesHandler(s.account),
		Markers:                mastodon.NewMarkersHandler(s.marker),
		FeaturedTags:           mastodon.NewFeaturedTagsHandler(s.featuredTag, s.account, cfg.MonsteraInstanceDomain),
		Announcements:          mastodon.NewAnnouncementsHandler(s.announcement),
		Push:                   mastodon.NewPushHandler(s.pushSubscription, cfg.VAPIDPublicKey),
		WebFinger:              activitypub.NewWebFingerHandler(s.account, cfg.MonsteraInstanceDomain, instanceBaseURL),
		NodeInfoPtr:            activitypub.NewNodeInfoPointerHandler(instanceBaseURL),
		NodeInfo:               activitypub.NewNodeInfoHandler(s.instance, cfg.Version),
		Actor:                  activitypub.NewActorHandler(s.account, instanceBaseURL, cfg.MonsteraUIURL.String()),
		Collections:            activitypub.NewCollectionsHandler(s.account, s.statusRead, instanceBaseURL),
		Outbox:                 activitypub.NewOutbox(s.account, s.timeline, instanceBaseURL),
		Inbox:                  activitypub.NewInboxHandler(s.inboxProcessor, s.signatureService, cfg.MonsteraInstanceDomain),
		User:                   monstera.NewUserHandler(s.account),
		ModeratorDashboard:     monstera.NewModeratorDashboardHandler(s.instance, s.moderation),
		AdminUsers:             monstera.NewAdminUsersHandler(s.account, s.moderation),
		ModeratorRegistrations: monstera.NewModeratorRegistrationsHandler(s.registration),
		ModeratorInvites:       monstera.NewModeratorInvitesHandler(s.registration, s.monsteraSettings),
		ModeratorReports:       monstera.NewModeratorReportsHandler(s.moderation),
		AdminFederation:        monstera.NewAdminFederationHandler(s.instance, s.moderation),
		ModeratorContent:       monstera.NewModeratorContentHandler(s.serverFilter),
		AdminSettings:          monstera.NewAdminSettingsHandler(s.monsteraSettings),
		AdminAnnouncements:     monstera.NewAdminAnnouncementsHandler(s.announcement),
	})
}
