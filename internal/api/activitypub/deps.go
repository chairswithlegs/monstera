package activitypub

import (
	"log/slog"

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
