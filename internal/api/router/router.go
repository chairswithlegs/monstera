package router

import (
	"log/slog"
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
	"github.com/chairswithlegs/monstera-fed/internal/store"
)

// Deps holds dependencies required to build the HTTP router.
type Deps struct {
	Logger        *slog.Logger
	Metrics       *observability.Metrics
	Health        *api.HealthChecker
	OAuthHandler  *oauthhandlers.Handler
	OAuthServer   *oauthpkg.Server
	Store         store.Store
	Accounts      *mastodon.AccountsHandler
	Statuses      *mastodon.StatusesHandler
	Timelines     *mastodon.TimelinesHandler
	Instance      *mastodon.InstanceHandler
	Notifications *mastodon.NotificationsHandler
	Media         *mastodon.MediaHandler
	WebFinger     *activitypub.WebFingerHandler
	NodeInfoPtr   *activitypub.NodeInfoPointerHandler
	NodeInfo      *activitypub.NodeInfoHandler
	Actor         *activitypub.ActorHandler
	Collections   *activitypub.CollectionsHandler
	Outbox        *activitypub.OutboxHandler
	Inbox         *activitypub.InboxHandler
}

// New builds the chi router with global middleware and P1–P2 routes.
func New(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(observability.RequestLogger(deps.Logger))
	r.Use(observability.MetricsMiddleware(deps.Metrics))
	r.Use(middleware.Recoverer(deps.Logger))
	r.Use(middleware.CORS)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Get("/healthz/live", deps.Health.Liveness)
	r.Get("/healthz/ready", deps.Health.Readiness)

	r.Get("/.well-known/webfinger", deps.WebFinger.ServeHTTP)
	r.Get("/.well-known/nodeinfo", deps.NodeInfoPtr.ServeHTTP)
	r.Get("/nodeinfo/2.0", deps.NodeInfo.ServeHTTP)
	r.Get("/users/{username}/outbox", deps.Outbox.ServeHTTP)
	r.Get("/users/{username}/followers", deps.Collections.ServeFollowers)
	r.Get("/users/{username}/following", deps.Collections.ServeFollowing)
	r.Get("/users/{username}/collections/featured", deps.Collections.ServeFeatured)
	r.Get("/users/{username}", deps.Actor.ServeHTTP)
	r.Post("/users/{username}/inbox", deps.Inbox.ServeHTTP)
	r.Post("/inbox", deps.Inbox.ServeHTTP)

	r.Route("/api/v2", func(r chi.Router) {
		r.Get("/instance", deps.Instance.GetInstance)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(deps.OAuthServer, deps.Store))
			r.Method("POST", "/media", middleware.RequiredScopes("write:media")(http.HandlerFunc(deps.Media.Upload)))
		})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/instance", deps.Instance.GetInstance)
		r.Post("/apps", deps.OAuthHandler.RegisterApp)
		r.Get("/custom_emojis", deps.Instance.CustomEmojis)

		r.Group(func(r chi.Router) {
			r.Use(middleware.OptionalAuth(deps.OAuthServer, deps.Store))
			r.Get("/accounts/{id}", deps.Accounts.Get)
			r.Get("/statuses/{id}", deps.Statuses.Get)
			r.Get("/timelines/public", deps.Timelines.Public)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(deps.OAuthServer, deps.Store))
			r.Method("GET", "/accounts/verify_credentials", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Accounts.VerifyCredentials)))
			r.Method("GET", "/accounts/relationships", middleware.RequiredScopes("read:follows")(http.HandlerFunc(deps.Accounts.Relationships)))
			r.Route("/accounts/{id}", func(r chi.Router) {
				r.Method("POST", "/follow", middleware.RequiredScopes("write:follows")(http.HandlerFunc(deps.Accounts.Follow)))
				r.Method("POST", "/unfollow", middleware.RequiredScopes("write:follows")(http.HandlerFunc(deps.Accounts.Unfollow)))
			})
			r.Method("POST", "/statuses", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.Create)))
			r.Method("DELETE", "/statuses/{id}", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.Delete)))
			r.Method("GET", "/timelines/home", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.Timelines.Home)))
			r.Method("GET", "/notifications", middleware.RequiredScopes("read:notifications")(http.HandlerFunc(deps.Notifications.List)))
		})
	})

	r.Get("/oauth/authorize", deps.OAuthHandler.Authorize)
	r.Post("/oauth/authorize", deps.OAuthHandler.AuthorizeSubmit)
	r.Post("/oauth/token", deps.OAuthHandler.Token)
	r.Post("/oauth/revoke", deps.OAuthHandler.Revoke)

	return r
}
