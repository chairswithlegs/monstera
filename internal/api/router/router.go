package router

import (
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
	oauthpkg "github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/observability"
	"github.com/chairswithlegs/monstera/internal/service"
)

// Deps holds dependencies required to build the HTTP router.
type Deps struct {
	OAuthServer     *oauthpkg.Server
	AccountsService service.AccountService

	// Health check handlers
	Health *api.HealthChecker

	// OAuth handlers
	OAuthHandler *oauthhandlers.Handler

	// Mastodon API handlers
	Accounts      *mastodon.AccountsHandler
	Statuses      *mastodon.StatusesHandler
	Timelines     *mastodon.TimelinesHandler
	Instance      *mastodon.InstanceHandler
	Notifications *mastodon.NotificationsHandler
	Media         *mastodon.MediaHandler
	Search        *mastodon.SearchHandler
	Streaming     *mastodon.StreamingHandler

	// ActivityPub handlers
	WebFinger   *activitypub.WebFingerHandler
	NodeInfoPtr *activitypub.NodeInfoPointerHandler
	NodeInfo    *activitypub.NodeInfoHandler
	Actor       *activitypub.ActorHandler
	Collections *activitypub.CollectionsHandler
	Outbox      *activitypub.OutboxHandler
	Inbox       *activitypub.InboxHandler

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
}

// New builds the chi router with global middleware and P1–P2 routes.
func New(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(observability.RequestLogger())
	r.Use(observability.MetricsMiddleware())
	r.Use(middleware.Recoverer())
	r.Use(middleware.CORS)

	r.Group(func(r chi.Router) {
		r.Use(chimw.Timeout(30 * time.Second))
		r.Get("/healthz/live", deps.Health.GETLiveness)
		r.Get("/healthz/ready", deps.Health.GETReadiness)
		r.Get("/.well-known/webfinger", deps.WebFinger.GETWebFinger)
		r.Get("/.well-known/nodeinfo", deps.NodeInfoPtr.GETNodeInfoPointer)
		r.Get("/nodeinfo/2.0", deps.NodeInfo.GETNodeInfo)
		r.Get("/users/{username}/outbox", deps.Outbox.GETOutbox)
		r.Get("/users/{username}/followers", deps.Collections.GETFollowers)
		r.Get("/users/{username}/following", deps.Collections.GETFollowing)
		r.Get("/users/{username}/collections/featured", deps.Collections.GETFeatured)
		r.Get("/users/{username}", deps.Actor.GETActor)
		r.Post("/users/{username}/inbox", deps.Inbox.POSTInbox)
		r.Post("/inbox", deps.Inbox.POSTInbox)
	})

	// Mastodon API routes (v2)
	r.Route("/api/v2", func(r chi.Router) {
		r.Use(chimw.Timeout(30 * time.Second))
		r.Get("/instance", deps.Instance.GETInstance)
		r.Group(func(r chi.Router) {
			r.Use(middleware.OptionalAuth(deps.OAuthServer, deps.AccountsService))
			r.Get("/search", deps.Search.GETSearch)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(deps.OAuthServer, deps.AccountsService))
			r.Method("POST", "/media", middleware.RequiredScopes("write:media")(http.HandlerFunc(deps.Media.POSTMedia)))
		})
	})

	// Mastodon API routes (v1) — streaming routes excluded from timeout
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/streaming/health", deps.Streaming.GETHealth)
		r.Group(func(r chi.Router) {
			r.Use(middleware.StreamingTokenFromQuery)
			r.Use(middleware.RequireAuth(deps.OAuthServer, deps.AccountsService))
			r.Get("/streaming/user", deps.Streaming.GETUser)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.StreamingTokenFromQuery)
			r.Use(middleware.OptionalAuth(deps.OAuthServer, deps.AccountsService))
			r.Get("/streaming/public", deps.Streaming.GETPublic)
			r.Get("/streaming/public/local", deps.Streaming.GETPublicLocal)
			r.Get("/streaming/hashtag", deps.Streaming.GETHashtag)
		})

		r.Group(func(r chi.Router) {
			r.Use(chimw.Timeout(30 * time.Second))
			r.Get("/instance", deps.Instance.GETInstance)
			r.Post("/apps", deps.OAuthHandler.POSTRegisterApp)
			r.Get("/custom_emojis", deps.Instance.GETCustomEmojis)

			r.Group(func(r chi.Router) {
				r.Use(middleware.OptionalAuth(deps.OAuthServer, deps.AccountsService))
				r.Get("/accounts/lookup", deps.Accounts.GETAccountsLookup)
				r.Get("/accounts/{id}", deps.Accounts.GETAccounts)
				r.Get("/statuses/{id}", deps.Statuses.GETStatuses)
				r.Get("/statuses/{id}/context", deps.Statuses.GETContext)
				r.Get("/statuses/{id}/favourited_by", deps.Statuses.GETFavouritedBy)
				r.Get("/statuses/{id}/reblogged_by", deps.Statuses.GETRebloggedBy)
				r.Get("/timelines/public", deps.Timelines.GETPublic)
				r.Get("/timelines/tag/{hashtag}", deps.Timelines.GETTag)
			})

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth(deps.OAuthServer, deps.AccountsService))
				r.Method("GET", "/accounts/verify_credentials", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Accounts.GETVerifyCredentials)))
				r.Method("PATCH", "/accounts/update_credentials", middleware.RequiredScopes("write:accounts")(http.HandlerFunc(deps.Accounts.PATCHUpdateCredentials)))
				r.Method("GET", "/accounts/relationships", middleware.RequiredScopes("read:follows")(http.HandlerFunc(deps.Accounts.GETRelationships)))
				r.Method("GET", "/accounts/{id}/statuses", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Accounts.GETAccountStatuses)))
				r.Method("GET", "/accounts/{id}/followers", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Accounts.GETFollowers)))
				r.Method("GET", "/accounts/{id}/following", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Accounts.GETFollowing)))
				r.Method("POST", "/accounts/{id}/follow", middleware.RequiredScopes("write:follows")(http.HandlerFunc(deps.Accounts.POSTFollow)))
				r.Method("POST", "/accounts/{id}/unfollow", middleware.RequiredScopes("write:follows")(http.HandlerFunc(deps.Accounts.POSTUnfollow)))
				r.Method("POST", "/accounts/{id}/block", middleware.RequiredScopes("write:blocks")(http.HandlerFunc(deps.Accounts.POSTBlock)))
				r.Method("POST", "/accounts/{id}/unblock", middleware.RequiredScopes("write:blocks")(http.HandlerFunc(deps.Accounts.POSTUnblock)))
				r.Method("POST", "/accounts/{id}/mute", middleware.RequiredScopes("write:mutes")(http.HandlerFunc(deps.Accounts.POSTMute)))
				r.Method("POST", "/accounts/{id}/unmute", middleware.RequiredScopes("write:mutes")(http.HandlerFunc(deps.Accounts.POSTUnmute)))
				r.Method("POST", "/statuses", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.POSTStatuses)))
				r.Route("/statuses/{id}", func(r chi.Router) {
					r.Method("DELETE", "/", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.DELETEStatuses)))
					r.Method("POST", "/reblog", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.POSTReblog)))
					r.Method("POST", "/unreblog", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.POSTUnreblog)))
					r.Method("POST", "/favourite", middleware.RequiredScopes("write:favourites")(http.HandlerFunc(deps.Statuses.POSTFavourite)))
					r.Method("POST", "/unfavourite", middleware.RequiredScopes("write:favourites")(http.HandlerFunc(deps.Statuses.POSTUnfavourite)))
				})
				r.Method("GET", "/timelines/home", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.Timelines.GETHome)))
				r.Method("GET", "/notifications", middleware.RequiredScopes("read:notifications")(http.HandlerFunc(deps.Notifications.GETNotifications)))
				r.Method("GET", "/notifications/{id}", middleware.RequiredScopes("read:notifications")(http.HandlerFunc(deps.Notifications.GETNotification)))
				r.Method("POST", "/notifications/clear", middleware.RequiredScopes("write:notifications")(http.HandlerFunc(deps.Notifications.POSTClear)))
				r.Method("POST", "/notifications/{id}/dismiss", middleware.RequiredScopes("write:notifications")(http.HandlerFunc(deps.Notifications.POSTDismiss)))
			})
		})
	})

	// Monstera API routes
	r.Route("/monstera/api/v1", func(r chi.Router) {
		r.Use(chimw.Timeout(30 * time.Second))
		r.Use(middleware.RequireAuth(deps.OAuthServer, deps.AccountsService))
		r.Method("GET", "/user", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.User.GETUser)))

		// Admin API (requires admin or moderator role)
		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.RequireModerator())
			r.Get("/dashboard", deps.ModeratorDashboard.GETDashboard)
			r.Get("/users", deps.AdminUsers.GETUsers)
			r.Get("/users/{id}", deps.AdminUsers.GETUser)
			r.Post("/users/{id}/suspend", deps.AdminUsers.POSTSuspend)
			r.Post("/users/{id}/unsuspend", deps.AdminUsers.POSTUnsuspend)
			r.Post("/users/{id}/silence", deps.AdminUsers.POSTSilence)
			r.Post("/users/{id}/unsilence", deps.AdminUsers.POSTUnsilence)
			r.With(middleware.RequireAdmin()).Put("/users/{id}/role", deps.AdminUsers.PUTRole)
			r.With(middleware.RequireAdmin()).Delete("/users/{id}", deps.AdminUsers.DELETEUser)
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
			r.With(middleware.RequireAdmin()).Post("/federation/domain-blocks", deps.AdminFederation.POSTDomainBlocks)
			r.With(middleware.RequireAdmin()).Delete("/federation/domain-blocks/{domain}", deps.AdminFederation.DELETEDomainBlock)
			r.Get("/content/filters", deps.ModeratorContent.GETFilters)
			r.Post("/content/filters", deps.ModeratorContent.POSTFilters)
			r.Put("/content/filters/{id}", deps.ModeratorContent.PUTFilter)
			r.Delete("/content/filters/{id}", deps.ModeratorContent.DELETEFilter)
			r.Route("/settings", func(r chi.Router) {
				r.Use(middleware.RequireAdmin())
				r.Get("/", deps.AdminSettings.GETSettings)
				r.Put("/", deps.AdminSettings.PUTSettings)
			})
		})
	})

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
