package mastodon

import (
	"log/slog"

	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// Deps collects dependencies for Mastodon HTTP handlers.
// Handlers call service layer only; no direct store access for business flows.
type Deps struct {
	Accounts           *service.AccountService
	Follows            *service.FollowService // nil to disable follow endpoints
	Statuses           *service.StatusService
	Timeline           *service.TimelineService
	Notifications      *service.NotificationService
	Media              *service.MediaService
	Logger             *slog.Logger
	InstanceDomain     string
	InstanceName       string
	MaxStatusChars     int
	MediaMaxBytes      int64
	SupportedMimeTypes []string
}
