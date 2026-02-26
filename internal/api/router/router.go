package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	oauthhandlers "github.com/chairswithlegs/monstera-fed/internal/api/oauth"
	oauthpkg "github.com/chairswithlegs/monstera-fed/internal/oauth"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
	"github.com/chairswithlegs/monstera-fed/internal/store"
)

// Deps holds dependencies required to build the HTTP router.
type Deps struct {
	Logger       *slog.Logger
	Metrics      *observability.Metrics
	Health       *api.HealthChecker
	OAuthHandler *oauthhandlers.Handler
	OAuthServer  *oauthpkg.Server
	Store        store.Store
	Accounts     *mastodon.AccountsHandler
	Statuses     *mastodon.StatusesHandler
	Timelines    *mastodon.TimelinesHandler
	Instance     *mastodon.InstanceHandler
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

	r.Route("/api/v2", func(r chi.Router) {
		r.Get("/instance", deps.Instance.GetInstance)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/instance", deps.Instance.GetInstance)
		r.Post("/apps", deps.OAuthHandler.RegisterApp)
		r.Get("/custom_emojis", deps.Instance.CustomEmojis)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(deps.OAuthServer, deps.Store))
			r.Method("GET", "/accounts/verify_credentials", middleware.RequiredScopes("read:accounts")(http.HandlerFunc(deps.Accounts.VerifyCredentials)))
			r.Method("POST", "/statuses", middleware.RequiredScopes("write:statuses")(http.HandlerFunc(deps.Statuses.Create)))
			r.Method("GET", "/timelines/home", middleware.RequiredScopes("read:statuses")(http.HandlerFunc(deps.Timelines.Home)))
		})
	})

	r.Get("/oauth/authorize", deps.OAuthHandler.Authorize)
	r.Post("/oauth/authorize", deps.OAuthHandler.AuthorizeSubmit)
	r.Post("/oauth/token", deps.OAuthHandler.Token)
	r.Post("/oauth/revoke", deps.OAuthHandler.Revoke)

	return r
}
