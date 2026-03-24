package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/activitypub"
	"github.com/chairswithlegs/monstera/internal/api/mastodon"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/api/monstera"
	oauthhandlers "github.com/chairswithlegs/monstera/internal/api/oauth"
	"github.com/chairswithlegs/monstera/internal/media/local"
	oauthpkg "github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/observability"
	"github.com/chairswithlegs/monstera/internal/ratelimit"
	"github.com/chairswithlegs/monstera/internal/service"
)

// RateLimitConfig holds rate limit parameters for the router.
type RateLimitConfig struct {
	Limiter      ratelimit.Limiter
	AuthLimit    int
	AuthWindow   time.Duration
	PublicLimit  int
	PublicWindow time.Duration
}

// Deps holds dependencies required to build the HTTP router.
type Deps struct {
	OAuthServer     *oauthpkg.Server
	AccountsService service.AccountService
	RateLimit       *RateLimitConfig

	// Health check handlers
	Health *api.HealthChecker

	// OAuth handlers
	OAuthHandler *oauthhandlers.Handler

	// Mastodon API handlers
	Accounts          *mastodon.AccountsHandler
	Statuses          *mastodon.StatusesHandler
	ScheduledStatuses *mastodon.ScheduledStatusesHandler
	Polls             *mastodon.PollsHandler
	Timelines         *mastodon.TimelinesHandler
	Instance          *mastodon.InstanceHandler
	Trends            *mastodon.TrendsHandler
	Conversations     *mastodon.ConversationsHandler
	Suggestions       *mastodon.SuggestionsHandler
	Notifications     *mastodon.NotificationsHandler
	Media             *mastodon.MediaHandler
	Search            *mastodon.SearchHandler
	Streaming         *mastodon.StreamingHandler
	Reports           *mastodon.ReportsHandler
	FollowRequests    *mastodon.FollowRequestsHandler
	Lists             *mastodon.ListsHandler
	Filters           *mastodon.FiltersHandler
	Preferences       *mastodon.PreferencesHandler
	Markers           *mastodon.MarkersHandler
	FeaturedTags      *mastodon.FeaturedTagsHandler
	Announcements     *mastodon.AnnouncementsHandler
	Push              *mastodon.PushHandler

	// ActivityPub handlers
	WebFinger   *activitypub.WebFingerHandler
	NodeInfoPtr *activitypub.NodeInfoPointerHandler
	NodeInfo    *activitypub.NodeInfoHandler
	Actor       *activitypub.ActorHandler
	Collections *activitypub.CollectionsHandler
	Outbox      *activitypub.OutboxHandler
	Inbox       *activitypub.InboxHandler

	// Body size limits.
	MaxRequestBodyBytes int64
	MediaMaxBytes       int64

	// MediaFileServer serves locally-stored media files (local driver only).
	// nil when media is served externally (e.g. S3/CDN).
	MediaFileServer http.Handler

	// Monstera API handlers
	User                   *monstera.UserHandler
	ModeratorDashboard     *monstera.ModeratorDashboardHandler
	AdminUsers             *monstera.AdminUsersHandler
	ModeratorRegistrations *monstera.ModeratorRegistrationsHandler
	ModeratorInvites       *monstera.ModeratorInvitesHandler
	ModeratorReports       *monstera.ModeratorReportsHandler
	AdminFederation        *monstera.AdminFederationHandler
	ModeratorContent       *monstera.ModeratorContentHandler
	AdminSettings          *monstera.AdminSettingsHandler
	AdminAnnouncements     *monstera.AdminAnnouncementsHandler
}

// New builds the chi router with global middleware and P1–P2 routes.
func New(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(observability.RequestIDMiddleware())
	r.Use(observability.RequestLoggerMiddleware())
	r.Use(observability.MetricsMiddleware())
	r.Use(middleware.Recoverer())
	r.Use(middleware.CORS)

	r.Group(func(r chi.Router) {
		r.Use(chimw.Timeout(30 * time.Second))
		if deps.MaxRequestBodyBytes > 0 {
			r.Use(middleware.MaxBodySize(deps.MaxRequestBodyBytes))
		}

		// Health check routes
		r.Get("/healthz/live", deps.Health.GETLiveness)
		r.Get("/healthz/ready", deps.Health.GETReadiness)

		// ActivityPub routes — paths are protocol-mandated and must not be prefixed.
		// Co-hosting with the UI is handled via reverse proxy + content negotiation.
		r.Get("/.well-known/webfinger", deps.WebFinger.GETWebFinger)
		r.Get("/.well-known/nodeinfo", deps.NodeInfoPtr.GETNodeInfoPointer)
		r.Get("/nodeinfo/2.0", deps.NodeInfo.GETNodeInfo)
		r.Post("/inbox", deps.Inbox.POSTInbox)

		// Note: these routes map to the IRIs generated for local accounts
		r.Post("/users/{username}/inbox", deps.Inbox.POSTInbox)
		r.Get("/users/{username}/outbox", deps.Outbox.GETOutbox)
		r.Get("/users/{username}/followers", deps.Collections.GETFollowers)
		r.Get("/users/{username}/following", deps.Collections.GETFollowing)
		r.Get("/users/{username}/collections/featured", deps.Collections.GETFeatured)
		r.Get("/users/{username}", deps.Actor.GETActor)
	})

	// Mastodon API routes (v2)
	r.Route("/api/v2", func(r chi.Router) {
		r.Use(chimw.Timeout(30 * time.Second))
		r.Get("/instance", deps.Instance.GETInstance)
		r.Group(func(r chi.Router) {
			r.Use(middleware.OptionalAuth(deps.OAuthServer, deps.AccountsService))
			if rl := deps.RateLimit; rl != nil {
				r.Use(middleware.RateLimitByIP(rl.Limiter, rl.PublicLimit, rl.PublicWindow))
			}
			r.Get("/search", deps.Search.GETSearch)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(deps.OAuthServer, deps.AccountsService))
			if rl := deps.RateLimit; rl != nil {
				r.Use(middleware.RateLimitByAccount(rl.Limiter, rl.AuthLimit, rl.AuthWindow))
			}
			r.Group(func(r chi.Router) {
				if deps.MediaMaxBytes > 0 {
					r.Use(middleware.MaxBodySize(deps.MediaMaxBytes))
				}
				r.Method("POST", "/media", middleware.RequiredScopes("write:media")(http.HandlerFunc(deps.Media.POSTMedia)))
			})
			r.Method("GET", "/suggestions", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Suggestions.GETSuggestions)))
		})
	})

	// Mastodon API routes (v1) — streaming routes excluded from timeout
	r.Route("/api/v1", func(r chi.Router) {
		// Streaming routes
		r.Get("/streaming/health", deps.Streaming.GETHealth)
		r.Group(func(r chi.Router) {
			r.Use(middleware.StreamingTokenFromQuery)
			r.Use(middleware.RequireAuth(deps.OAuthServer, deps.AccountsService))
			r.Get("/streaming/user", deps.Streaming.GETUser)
			r.Get("/streaming/user/notification", deps.Streaming.GETUserNotification)
			r.Get("/streaming/list", deps.Streaming.GETList)
			r.Get("/streaming/direct", deps.Streaming.GETDirect)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.StreamingTokenFromQuery)
			r.Use(middleware.OptionalAuth(deps.OAuthServer, deps.AccountsService))
			r.Get("/streaming/public", deps.Streaming.GETPublic)
			r.Get("/streaming/public/local", deps.Streaming.GETPublicLocal)
			r.Get("/streaming/hashtag", deps.Streaming.GETHashtag)
			// Unified WebSocket streaming endpoint (multiplexes all stream types).
			r.Get("/streaming", deps.Streaming.GETStreamingWS)
		})

		r.Group(func(r chi.Router) {
			// Public routes
			r.Use(chimw.Timeout(30 * time.Second))
			r.Get("/instance", deps.Instance.GETInstanceV1)
			r.Post("/apps", deps.OAuthHandler.POSTRegisterApp)
			r.Post("/accounts", deps.Accounts.POSTAccounts)
			r.Get("/custom_emojis", deps.Instance.GETCustomEmojis)

			// Auth optional routes
			r.Group(func(r chi.Router) {
				r.Use(middleware.OptionalAuth(deps.OAuthServer, deps.AccountsService))
				if rl := deps.RateLimit; rl != nil {
					r.Use(middleware.RateLimitByIP(rl.Limiter, rl.PublicLimit, rl.PublicWindow))
				}
				r.Get("/accounts/lookup", deps.Accounts.GETAccountsLookup)
				r.Get("/accounts/{id}", deps.Accounts.GETAccounts)
				r.Get("/directory", deps.Accounts.GETDirectory)
				r.Get("/search", deps.Search.GETSearch)
				r.Get("/statuses/{id}", deps.Statuses.GETStatuses)
				r.Get("/statuses/{id}/context", deps.Statuses.GETContext)
				r.Get("/statuses/{id}/favourited_by", deps.Statuses.GETFavouritedBy)
				r.Get("/statuses/{id}/reblogged_by", deps.Statuses.GETRebloggedBy)
				r.Get("/polls/{id}", deps.Polls.GETPoll)
				r.Get("/timelines/public", deps.Timelines.GETPublic)
				r.Get("/timelines/tag/{hashtag}", deps.Timelines.GETTag)
				r.Get("/tags/{name}", deps.Accounts.GETTag)
				r.Get("/trends/statuses", deps.Trends.GETTrendsStatuses)
				r.Get("/trends/tags", deps.Trends.GETTrendsTags)
				r.Get("/trends/links", deps.Trends.GETTrendsLinks)
			})

			// Auth required routes
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth(deps.OAuthServer, deps.AccountsService))
				if rl := deps.RateLimit; rl != nil {
					r.Use(middleware.RateLimitByAccount(rl.Limiter, rl.AuthLimit, rl.AuthWindow))
				}
				r.Method("GET", "/accounts/verify_credentials", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Accounts.GETVerifyCredentials)))
				r.Method("GET", "/preferences", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Preferences.GETPreferences)))
				r.Method("GET", "/statuses/{id}/quotes", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.Statuses.GETQuotes)))
				r.Method("GET", "/markers", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.Markers.GETMarkers)))
				r.Method("POST", "/markers", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Markers.POSTMarkers)))
				r.Method("PATCH", "/accounts/update_credentials", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.Accounts.PATCHUpdateCredentials)))
				r.Method("GET", "/accounts/relationships", middleware.RequiredScopes("read:follows")(http.HandlerFunc(deps.Accounts.GETRelationships)))
				r.Method("GET", "/accounts/{id}/statuses", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Accounts.GETAccountStatuses)))
				r.Method("GET", "/accounts/{id}/followers", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Accounts.GETFollowers)))
				r.Method("GET", "/accounts/{id}/following", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Accounts.GETFollowing)))
				r.Method("POST", "/accounts/{id}/follow", middleware.RequiredScopes("write:follows")(http.HandlerFunc(deps.Accounts.POSTFollow)))
				r.Method("POST", "/accounts/{id}/unfollow", middleware.RequiredScopes("write:follows")(http.HandlerFunc(deps.Accounts.POSTUnfollow)))
				r.Method("GET", "/blocks", middleware.RequiredScopes("read:blocks")(http.HandlerFunc(deps.Accounts.GETBlocks)))
				r.Method("POST", "/accounts/{id}/block", middleware.RequiredScopes("write:blocks")(http.HandlerFunc(deps.Accounts.POSTBlock)))
				r.Method("POST", "/accounts/{id}/unblock", middleware.RequiredScopes("write:blocks")(http.HandlerFunc(deps.Accounts.POSTUnblock)))
				r.Method("GET", "/mutes", middleware.RequiredScopes("read:mutes")(http.HandlerFunc(deps.Accounts.GETMutes)))
				r.Method("POST", "/accounts/{id}/mute", middleware.RequiredScopes("write:mutes")(http.HandlerFunc(deps.Accounts.POSTMute)))
				r.Method("POST", "/accounts/{id}/unmute", middleware.RequiredScopes("write:mutes")(http.HandlerFunc(deps.Accounts.POSTUnmute)))
				r.Method("GET", "/followed_tags", middleware.RequiredScopes("read:follows")(http.HandlerFunc(deps.Accounts.GETFollowedTags)))
				r.Method("POST", "/tags/{name}/follow", middleware.RequiredScopes("write:follows")(http.HandlerFunc(deps.Accounts.POSTTagFollow)))
				r.Method("POST", "/tags/{name}/unfollow", middleware.RequiredScopes("write:follows")(http.HandlerFunc(deps.Accounts.POSTTagUnfollow)))
				r.Method("GET", "/featured_tags", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.FeaturedTags.GETFeaturedTags)))
				r.Method("POST", "/featured_tags", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.FeaturedTags.POSTFeaturedTags)))
				r.Method("DELETE", "/featured_tags/{id}", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.FeaturedTags.DELETEFeaturedTag)))
				r.Method("GET", "/featured_tags/suggestions", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.FeaturedTags.GETFeaturedTagSuggestions)))
				r.Method("GET", "/announcements", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Announcements.GETAnnouncements)))
				r.Route("/announcements/{id}", func(r chi.Router) {
					r.Method("POST", "/dismiss", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.Announcements.POSTDismissAnnouncement)))
					r.Route("/reactions/{name}", func(r chi.Router) {
						r.Method("PUT", "/", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.Announcements.PUTAnnouncementReaction)))
						r.Method("DELETE", "/", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.Announcements.DELETEAnnouncementReaction)))
					})
				})
				r.Group(func(r chi.Router) {
					if deps.MediaMaxBytes > 0 {
						r.Use(middleware.MaxBodySize(deps.MediaMaxBytes))
					}
					r.Method("POST", "/media", middleware.RequiredScopes("write:media")(http.HandlerFunc(deps.Media.POSTMedia)))
					r.Method("PUT", "/media/{id}", middleware.RequiredScopes("write:media")(http.HandlerFunc(deps.Media.PUTMedia)))
				})
				r.Method("POST", "/statuses", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.POSTStatuses)))
				r.Method("DELETE", "/statuses/{id}", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.DELETEStatuses)))
				r.Method("POST", "/statuses/{id}/reblog", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.POSTReblog)))
				r.Method("POST", "/statuses/{id}/unreblog", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.POSTUnreblog)))
				r.Method("POST", "/statuses/{id}/favourite", middleware.RequiredScopes("write:favourites")(http.HandlerFunc(deps.Statuses.POSTFavourite)))
				r.Method("POST", "/statuses/{id}/unfavourite", middleware.RequiredScopes("write:favourites")(http.HandlerFunc(deps.Statuses.POSTUnfavourite)))
				r.Method("PUT", "/statuses/{id}", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.PUTStatuses)))
				r.Method("PUT", "/statuses/{id}/interaction_policy", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.PUTInteractionPolicy)))
				r.Method("POST", "/statuses/{id}/quotes/{quoting_status_id}/revoke", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.POSTRevokeQuote)))
				r.Method("GET", "/statuses/{id}/history", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.Statuses.GETStatusHistory)))
				r.Method("GET", "/statuses/{id}/source", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.Statuses.GETStatusSource)))
				r.Method("POST", "/statuses/{id}/pin", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.POSTPin)))
				r.Method("POST", "/statuses/{id}/unpin", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.POSTUnpin)))
				r.Method("POST", "/statuses/{id}/mute", middleware.RequiredScopes("write:mutes")(http.HandlerFunc(deps.Statuses.POSTMuteConversation)))
				r.Method("POST", "/statuses/{id}/unmute", middleware.RequiredScopes("write:mutes")(http.HandlerFunc(deps.Statuses.POSTUnmuteConversation)))
				r.Method("GET", "/scheduled_statuses", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.ScheduledStatuses.GETScheduledStatuses)))
				r.Method("GET", "/scheduled_statuses/{id}", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.ScheduledStatuses.GETScheduledStatus)))
				r.Method("POST", "/polls/{id}/votes", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Polls.POSTVotes)))
				r.Method("PUT", "/scheduled_statuses/{id}", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.ScheduledStatuses.PUTScheduledStatus)))
				r.Method("DELETE", "/scheduled_statuses/{id}", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.ScheduledStatuses.DELETEScheduledStatus)))
				r.Method("POST", "/statuses/{id}/bookmark", middleware.RequiredScopes("write:bookmarks")(http.HandlerFunc(deps.Statuses.POSTBookmark)))
				r.Method("POST", "/statuses/{id}/unbookmark", middleware.RequiredScopes("write:bookmarks")(http.HandlerFunc(deps.Statuses.POSTUnbookmark)))
				r.Method("GET", "/timelines/home", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.Timelines.GETHome)))
				r.Method("GET", "/favourites", middleware.RequiredScopes("read:favourites")(http.HandlerFunc(deps.Timelines.GETFavourites)))
				r.Method("GET", "/bookmarks", middleware.RequiredScopes("read:bookmarks")(http.HandlerFunc(deps.Timelines.GETBookmarks)))
				r.Method("GET", "/timelines/list/{id}", middleware.RequiredScopes("read:lists")(http.HandlerFunc(deps.Timelines.GETListTimeline)))
				r.Method("GET", "/lists", middleware.RequiredScopes("read:lists")(http.HandlerFunc(deps.Lists.GETLists)))
				r.Method("POST", "/lists", middleware.RequiredScopes("write:lists")(http.HandlerFunc(deps.Lists.POSTLists)))
				r.Route("/lists/{id}", func(r chi.Router) {
					r.Method("GET", "/", middleware.RequiredScopes("read:lists")(http.HandlerFunc(deps.Lists.GETList)))
					r.Method("PUT", "/", middleware.RequiredScopes("write:lists")(http.HandlerFunc(deps.Lists.PUTList)))
					r.Method("DELETE", "/", middleware.RequiredScopes("write:lists")(http.HandlerFunc(deps.Lists.DELETEList)))
					r.Method("GET", "/accounts", middleware.RequiredScopes("read:lists")(http.HandlerFunc(deps.Lists.GETListAccounts)))
					r.Method("POST", "/accounts", middleware.RequiredScopes("write:lists")(http.HandlerFunc(deps.Lists.POSTListAccounts)))
					r.Method("DELETE", "/accounts", middleware.RequiredScopes("write:lists")(http.HandlerFunc(deps.Lists.DELETEListAccounts)))
				})
				r.Method("GET", "/conversations", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.Conversations.GETConversations)))
				r.Route("/conversations/{id}", func(r chi.Router) {
					r.Method("DELETE", "/", middleware.RequiredScopes("write:conversations")(http.HandlerFunc(deps.Conversations.DELETEConversation)))
					r.Method("POST", "/read", middleware.RequiredScopes("write:conversations")(http.HandlerFunc(deps.Conversations.POSTConversationRead)))
				})
				r.Method("GET", "/suggestions", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Suggestions.GETSuggestions)))
				r.Method("GET", "/notifications", middleware.RequiredScopes("read:notifications")(http.HandlerFunc(deps.Notifications.GETNotifications)))
				r.Method("GET", "/notifications/{id}", middleware.RequiredScopes("read:notifications")(http.HandlerFunc(deps.Notifications.GETNotification)))
				r.Method("POST", "/notifications/clear", middleware.RequiredScopes("write:notifications")(http.HandlerFunc(deps.Notifications.POSTClear)))
				r.Method("POST", "/notifications/{id}/dismiss", middleware.RequiredScopes("write:notifications")(http.HandlerFunc(deps.Notifications.POSTDismiss)))
				r.Method("POST", "/reports", middleware.RequiredScopes("write:reports")(http.HandlerFunc(deps.Reports.POSTReports)))
				r.Method("POST", "/push/subscription", middleware.RequiredScopes("push")(http.HandlerFunc(deps.Push.POSTSubscription)))
				r.Method("GET", "/push/subscription", middleware.RequiredScopes("push")(http.HandlerFunc(deps.Push.GETSubscription)))
				r.Method("PUT", "/push/subscription", middleware.RequiredScopes("push")(http.HandlerFunc(deps.Push.PUTSubscription)))
				r.Method("DELETE", "/push/subscription", middleware.RequiredScopes("push")(http.HandlerFunc(deps.Push.DELETESubscription)))
				r.Method("GET", "/follow_requests", middleware.RequiredScopes("read:follows")(http.HandlerFunc(deps.FollowRequests.GETFollowRequests)))
				r.Route("/follow_requests/{id}", func(r chi.Router) {
					r.Method("POST", "/authorize", middleware.RequiredScopes("write:follows")(http.HandlerFunc(deps.FollowRequests.POSTAuthorize)))
					r.Method("POST", "/reject", middleware.RequiredScopes("write:follows")(http.HandlerFunc(deps.FollowRequests.POSTReject)))
				})
				r.Method("GET", "/filters", middleware.RequiredScopes("read:filters")(http.HandlerFunc(deps.Filters.GETFilters)))
				r.Method("POST", "/filters", middleware.RequiredScopes("write:filters")(http.HandlerFunc(deps.Filters.POSTFilters)))
				r.Route("/filters/{id}", func(r chi.Router) {
					r.Method("GET", "/", middleware.RequiredScopes("read:filters")(http.HandlerFunc(deps.Filters.GETFilter)))
					r.Method("PUT", "/", middleware.RequiredScopes("write:filters")(http.HandlerFunc(deps.Filters.PUTFilter)))
					r.Method("DELETE", "/", middleware.RequiredScopes("write:filters")(http.HandlerFunc(deps.Filters.DELETEFilter)))
				})
			})
		})
	})

	// Monstera API routes
	r.Route("/monstera/api/v1", func(r chi.Router) {
		r.Use(chimw.Timeout(30 * time.Second))
		r.Use(middleware.RequireAuth(deps.OAuthServer, deps.AccountsService))
		r.Method("GET", "/user", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.User.GETUser)))
		r.Method("PATCH", "/account/profile", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.User.PATCHProfile)))
		r.Method("PATCH", "/account/preferences", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.User.PATCHPreferences)))
		r.Method("PATCH", "/account/security/email", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.User.PATCHEmail)))
		r.Method("PATCH", "/account/security/password", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.User.PATCHPassword)))

		// Moderator API (requires moderator or admin role)
		r.Route("/moderator", func(r chi.Router) {
			r.Use(middleware.RequireModerator())
			r.Get("/dashboard", deps.ModeratorDashboard.GETDashboard)
			r.Get("/users", deps.AdminUsers.GETUsers)
			r.Get("/users/{id}", deps.AdminUsers.GETUser)
			r.Post("/users/{id}/suspend", deps.AdminUsers.POSTSuspend)
			r.Post("/users/{id}/unsuspend", deps.AdminUsers.POSTUnsuspend)
			r.Post("/users/{id}/silence", deps.AdminUsers.POSTSilence)
			r.Post("/users/{id}/unsilence", deps.AdminUsers.POSTUnsilence)
			r.Get("/registrations", deps.ModeratorRegistrations.GETRegistrations)
			r.Post("/registrations/{id}/approve", deps.ModeratorRegistrations.POSTApprove)
			r.Post("/registrations/{id}/reject", deps.ModeratorRegistrations.POSTReject)
			r.Get("/invites", deps.ModeratorInvites.GETInvites)
			r.Post("/invites", deps.ModeratorInvites.POSTInvites)
			r.Delete("/invites/{id}", deps.ModeratorInvites.DELETEInvite)
			r.Get("/reports", deps.ModeratorReports.GETReports)
			r.Get("/reports/{id}", deps.ModeratorReports.GETReport)
			r.Post("/reports/{id}/assign", deps.ModeratorReports.POSTAssign)
			r.Post("/reports/{id}/resolve", deps.ModeratorReports.POSTResolve)
			r.Get("/federation/instances", deps.AdminFederation.GETInstances)
			r.Get("/federation/domain-blocks", deps.AdminFederation.GETDomainBlocks)
			r.Get("/content/filters", deps.ModeratorContent.GETFilters)
			r.Post("/content/filters", deps.ModeratorContent.POSTFilters)
			r.Put("/content/filters/{id}", deps.ModeratorContent.PUTFilter)
			r.Delete("/content/filters/{id}", deps.ModeratorContent.DELETEFilter)
		})

		// Admin API (requires admin role)
		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.RequireAdmin())
			r.Put("/users/{id}/role", deps.AdminUsers.PUTRole)
			r.Delete("/users/{id}", deps.AdminUsers.DELETEUser)
			r.Post("/federation/domain-blocks", deps.AdminFederation.POSTDomainBlocks)
			r.Delete("/federation/domain-blocks/{domain}", deps.AdminFederation.DELETEDomainBlock)
			r.Get("/settings", deps.AdminSettings.GETSettings)
			r.Put("/settings", deps.AdminSettings.PUTSettings)
			r.Get("/announcements", deps.AdminAnnouncements.GETAnnouncements)
			r.Post("/announcements", deps.AdminAnnouncements.POSTAnnouncements)
			r.Put("/announcements/{id}", deps.AdminAnnouncements.PUTAnnouncement)
		})
	})

	// Serve locally-stored media files (local driver only).
	if deps.MediaFileServer != nil {
		mediaFileServerPath := fmt.Sprintf("/%s/*", local.LOCAL_MEDIA_URL_PATH_PREFIX)
		r.Get(mediaFileServerPath, deps.MediaFileServer.ServeHTTP)
	}

	// OAuth routes
	r.Group(func(r chi.Router) {
		r.Use(chimw.Timeout(30 * time.Second))
		r.Get("/oauth/authorize", deps.OAuthHandler.GETAuthorize)
		r.Post("/oauth/login", deps.OAuthHandler.POSTLogin)
		r.Post("/oauth/token", deps.OAuthHandler.POSTToken)
		r.Post("/oauth/revoke", deps.OAuthHandler.POSTRevoke)
	})

	return r
}
