package service

// Pagination limits used by services. The API layer enforces smaller,
// Mastodon-compatible caps (e.g. 80 for directory/blocks/mutes). These
// constants are defensive: they prevent unbounded queries if a caller
// passes a very large limit.

// DefaultServiceListLimit is used when a list endpoint receives limit <= 0.
const DefaultServiceListLimit = 80

// MaxServicePageLimit is the maximum limit any list/pagination method will
// honor. Callers (handlers) should enforce API-specific caps; this is a
// safety ceiling to avoid unbounded DB queries.
const MaxServicePageLimit = 1000

// ClampLimit normalises a caller-supplied limit: zero/negative values become
// defaultLimit, values above maxLimit are capped to maxLimit.
func ClampLimit(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
