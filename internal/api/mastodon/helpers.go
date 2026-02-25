package mastodon

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	defaultPageLimit = 20
	maxPageLimit     = 40
)

// PageParams holds cursor-based pagination parameters.
type PageParams struct {
	MaxID   string
	MinID   string
	SinceID string
	Limit   int
}

// PageParamsFromRequest parses max_id, min_id, since_id, and limit from the query string.
func PageParamsFromRequest(r *http.Request) PageParams {
	q := r.URL.Query()
	limit := defaultPageLimit
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n >= 1 {
			limit = n
			if limit > maxPageLimit {
				limit = maxPageLimit
			}
		}
	}
	return PageParams{
		MaxID:   q.Get("max_id"),
		MinID:   q.Get("min_id"),
		SinceID: q.Get("since_id"),
		Limit:   limit,
	}
}

// LinkHeader builds an RFC 5988 Link header for pagination.
func LinkHeader(requestURL string, firstID, lastID string) string {
	if firstID == "" && lastID == "" {
		return ""
	}
	base, err := url.Parse(requestURL)
	if err != nil {
		return ""
	}
	var parts []string
	if lastID != "" {
		u := *base
		q := u.Query()
		q.Set("max_id", lastID)
		u.RawQuery = q.Encode()
		parts = append(parts, fmt.Sprintf("<%s>; rel=%q", u.String(), "next"))
	}
	if firstID != "" {
		u := *base
		q := u.Query()
		q.Del("max_id")
		q.Set("min_id", firstID)
		u.RawQuery = q.Encode()
		parts = append(parts, fmt.Sprintf("<%s>; rel=%q", u.String(), "prev"))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}
