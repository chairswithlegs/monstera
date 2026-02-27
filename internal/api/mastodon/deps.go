package mastodon

import (
	"errors"

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
	InstanceDomain     string
	InstanceName       string
	MaxStatusChars     int
	MediaMaxBytes      int64
	SupportedMimeTypes []string
}

func (d *Deps) Validate() error {
	if d.Accounts == nil {
		return errors.New("account service not configured")
	}
	if d.Follows == nil {
		return errors.New("follow service not configured")
	}
	if d.Statuses == nil {
		return errors.New("status service not configured")
	}
	if d.Timeline == nil {
		return errors.New("timeline service not configured")
	}
	if d.Notifications == nil {
		return errors.New("notification service not configured")
	}
	if d.Media == nil {
		return errors.New("media service not configured")
	}
	if d.InstanceDomain == "" {
		return errors.New("instance domain not configured")
	}
	if d.InstanceName == "" {
		return errors.New("instance name not configured")
	}
	if d.MaxStatusChars == 0 {
		return errors.New("max status characters not configured")
	}
	if len(d.SupportedMimeTypes) == 0 {
		return errors.New("supported mime types not configured")
	}
	return nil
}
