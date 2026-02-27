package activitypub

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/store"
)

// addChiURLParam sets chi's "username" URL param on the request for testing.
func addChiURLParam(r *http.Request, username string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("username", username)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func strPtr(s string) *string { return &s }

// testAccountService returns an AccountService for handler tests.
func testAccountService(s store.Store, cfg *config.Config) *service.AccountService {
	return service.NewAccountService(s, "https://"+cfg.InstanceDomain)
}

// testTimelineService returns a TimelineService for handler tests.
func testTimelineService(s store.Store) *service.TimelineService {
	return service.NewTimelineService(s)
}

// testInstanceService returns an InstanceService for handler tests.
func testInstanceService(s store.Store) *service.InstanceService {
	return service.NewInstanceService(s)
}
