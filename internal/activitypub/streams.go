package activitypub

import (
	"github.com/chairswithlegs/monstera/internal/activitypub/internal"
	"github.com/chairswithlegs/monstera/internal/natsutil"
)

// StreamConfigs returns the JetStream stream configurations for ActivityPub federation.
var StreamConfigs []natsutil.StreamConfig = internal.StreamConfigs

// Stream and consumer names re-exported for wiring.
const (
	StreamBackfill          = internal.StreamBackfill
	ConsumerBackfill        = internal.ConsumerBackfill
	StreamOutboxDeliveryDLQ = internal.StreamOutboxDeliveryDLQ
	StreamOutboxFanoutDLQ   = internal.StreamOutboxFanoutDLQ
)
