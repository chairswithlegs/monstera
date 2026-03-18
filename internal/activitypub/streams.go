package activitypub

import (
	"github.com/chairswithlegs/monstera/internal/activitypub/internal"
	"github.com/chairswithlegs/monstera/internal/natsutil"
)

// StreamConfigs returns the JetStream stream configurations for ActivityPub federation.
var StreamConfigs []natsutil.StreamConfig = internal.StreamConfigs
