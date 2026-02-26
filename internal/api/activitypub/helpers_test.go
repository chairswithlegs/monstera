package activitypub

import (
	"context"
	"net/http"

	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/store"
)

// TODO: determine if these belong here

// testDeps builds Deps from a store and config for handler tests.
func testDeps(s store.Store, cfg *config.Config) Deps {
	return Deps{
		Accounts:  service.NewAccountService(s, "https://"+cfg.InstanceDomain),
		Timelines: service.NewTimelineService(s),
		Instance:  service.NewInstanceService(s),
		Config:    cfg,
		Logger:    slog.Default(),
	}
}

// addChiURLParam sets chi's "username" URL param on the request for testing.
func addChiURLParam(r *http.Request, username string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("username", username)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func strPtr(s string) *string { return &s }
