package activitypub

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/ap"
	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// Deps collects dependencies for ActivityPub HTTP handlers.
// Handlers call service layer only; no direct store access for business flows.
type Deps struct {
	Accounts  *service.AccountService
	Timelines *service.TimelineService
	Instance  *service.InstanceService
	Cache     cache.Store
	Config    *config.Config
	Logger    *slog.Logger
	Inbox     *ap.InboxProcessor
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJRD(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/jrd+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
