package mastodon

// Pagination defaults and limits for the Mastodon API. Handlers use these for
// defaults and to enforce per-endpoint caps; the service layer applies a
// separate defensive cap to avoid unbounded queries.

// Cursor-based pagination (timelines, follow requests, etc.)
const (
	DefaultPageLimit = 20 // default limit when client omits or sends invalid limit
	MaxPageLimit     = 40 // max limit for cursor-paginated list endpoints
)

// List endpoints that use offset or cursor with higher caps (directory, blocks, mutes)
const (
	DefaultListLimit = 40 // default limit for directory/blocks/mutes
	MaxListLimit     = 80 // max limit for directory/blocks/mutes
)

const contentTypeOctetStream = "application/octet-stream"
