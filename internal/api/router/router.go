package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	oauthhandlers "github.com/chairswithlegs/monstera-fed/internal/api/oauth"
	oauthpkg "github.com/chairswithlegs/monstera-fed/internal/oauth"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// Deps holds dependencies required to build the HTTP router.
type Deps struct {
	OAuthServer     *oauthpkg.Server
	AccountsService service.AccountService
	Health          *api.HealthChecker
	OAuthHandler    *oauthhandlers.Handler
	Accounts        *mastodon.AccountsHandler
	Statuses        *mastodon.StatusesHandler
	Timelines       *mastodon.TimelinesHandler
	Instance        *mastodon.InstanceHandler
	Notifications   *mastodon.NotificationsHandler
	Media           *mastodon.MediaHandler
	Search          *mastodon.SearchHandler
	Streaming       *mastodon.StreamingHandler
	WebFinger       *activitypub.WebFingerHandler
	NodeInfoPtr     *activitypub.NodeInfoPointerHandler
	NodeInfo        *activitypub.NodeInfoHandler
	Actor           *activitypub.ActorHandler
	Collections     *activitypub.CollectionsHandler
	Outbox          *activitypub.OutboxHandler
	Inbox           *activitypub.InboxHandler
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

	r.Route("/api/v2", func(r chi.Router) {
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

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/instance", deps.Instance.GETInstance)
		r.Post("/apps", deps.OAuthHandler.POSTRegisterApp)
		r.Get("/custom_emojis", deps.Instance.GETCustomEmojis)
		r.Get("/streaming/health", deps.Streaming.GETHealth)

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

	r.Get("/oauth/authorize", deps.OAuthHandler.GETAuthorize)
	r.Post("/oauth/authorize", deps.OAuthHandler.POSTAuthorizeSubmit)
	r.Post("/oauth/token", deps.OAuthHandler.POSTToken)
	r.Post("/oauth/revoke", deps.OAuthHandler.POSTRevoke)

	return r
}
