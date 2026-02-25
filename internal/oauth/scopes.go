package oauth

import (
	"slices"
	"strings"
)

// scopeExpansion maps parent scopes to their children. A token with "read"
// is treated as having all "read:*" scopes. This matches Mastodon's behaviour.
var scopeExpansion = map[string][]string{
	"read": {
		"read:accounts",
		"read:statuses",
		"read:notifications",
		"read:blocks",
		"read:filters",
		"read:follows",
		"read:lists",
		"read:mutes",
		"read:search",
		"read:favourites",
		"read:bookmarks",
	},
	"write": {
		"write:accounts",
		"write:statuses",
		"write:media",
		"write:follows",
		"write:notifications",
		"write:blocks",
		"write:filters",
		"write:lists",
		"write:mutes",
		"write:favourites",
		"write:bookmarks",
		"write:conversations",
		"write:reports",
	},
	"admin:read": {
		"admin:read:accounts",
		"admin:read:reports",
		"admin:read:domain_allows",
		"admin:read:domain_blocks",
		"admin:read:ip_blocks",
		"admin:read:email_domain_blocks",
		"admin:read:canonical_email_blocks",
	},
	"admin:write": {
		"admin:write:accounts",
		"admin:write:reports",
		"admin:write:domain_allows",
		"admin:write:domain_blocks",
		"admin:write:ip_blocks",
		"admin:write:email_domain_blocks",
		"admin:write:canonical_email_blocks",
	},
	"follow": {
		"read:follows",
		"write:follows",
		"read:blocks",
		"write:blocks",
		"read:mutes",
		"write:mutes",
	},
}

// allKnownScopes is the set of every valid scope string (top-level + children).
// Built once at package init time.
var allKnownScopes map[string]bool

func init() {
	allKnownScopes = make(map[string]bool)
	for parent, children := range scopeExpansion {
		allKnownScopes[parent] = true
		for _, c := range children {
			allKnownScopes[c] = true
		}
	}
	allKnownScopes["push"] = true
}

// ScopeSet is the resolved set of scopes carried by a token.
// Implemented as a map for O(1) lookup.
type ScopeSet map[string]bool

// Parse splits a space-separated scope string, expands parent scopes, and
// returns the fully resolved ScopeSet.
//
// Unknown scopes are silently dropped. This is intentional: Mastodon clients
// sometimes request scopes that a server doesn't support (e.g. a Phase 2
// scope). Dropping them avoids hard failures while the granted token correctly
// reflects what the server actually supports.
func Parse(raw string) ScopeSet {
	ss := make(ScopeSet)
	for _, s := range strings.Fields(raw) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !allKnownScopes[s] {
			continue
		}
		ss[s] = true
		if children, ok := scopeExpansion[s]; ok {
			for _, c := range children {
				ss[c] = true
			}
		}
	}
	return ss
}

// HasScope returns true if the set contains the required scope, accounting
// for scope expansion. For example, if the set contains "read", then
// HasScope("read:statuses") returns true.
func (s ScopeSet) HasScope(required string) bool {
	return s[required]
}

// HasAll returns true if the set contains every scope in required.
func (s ScopeSet) HasAll(required ...string) bool {
	for _, r := range required {
		if !s[r] {
			return false
		}
	}
	return true
}

// String returns the canonical space-separated, alphabetically sorted
// representation of all scopes in the set.
func (s ScopeSet) String() string {
	out := make([]string, 0, len(s))
	for scope := range s {
		out = append(out, scope)
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

// Normalize expands parent scopes and returns a canonical sorted string.
// Used when storing scopes in the database to ensure consistent comparisons.
func Normalize(raw string) string {
	return Parse(raw).String()
}

// Intersect returns a new ScopeSet containing only scopes present in both
// sets. Used to restrict a token's scopes to the intersection of what the
// application registered and what the user authorized.
func (s ScopeSet) Intersect(other ScopeSet) ScopeSet {
	result := make(ScopeSet)
	for scope := range s {
		if other[scope] {
			result[scope] = true
		}
	}
	return result
}
