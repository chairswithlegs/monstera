package mastodon

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// PageParams holds cursor-based pagination parameters.
type PageParams struct {
	MaxID   string
	MinID   string
	SinceID string
	Limit   int
}

// AbsoluteRequestURL returns the full request URL for the instance (scheme + instance domain + path + query).
// Link headers must use the instance's canonical URL so clients get valid absolute URLs (e.g. https://monstera.local/...).
func AbsoluteRequestURL(r *http.Request, instanceDomain string) string {
	base := "https://" + strings.TrimSuffix(instanceDomain, "/")
	return base + r.URL.RequestURI()
}

// PageParamsFromRequest parses max_id, min_id, since_id, and limit from the query string.
func PageParamsFromRequest(r *http.Request) PageParams {
	q := r.URL.Query()
	limit := DefaultPageLimit
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n >= 1 {
			limit = n
			if limit > MaxPageLimit {
				limit = MaxPageLimit
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

// LinkHeader builds an RFC 5988 Link header for cursor-based pagination.
// Clients use the Link response header to discover next/prev page URLs: rel="next"
// (max_id=lastID for older items) and rel="prev" (min_id=firstID for newer items).
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

// linkHeaderFavourites builds a Link header for GET /api/v1/favourites.
// nextCursor is the favourite ID for the next page (max_id); firstID is the first status ID for prev (min_id).
func linkHeaderFavourites(requestURL, firstID string, nextCursor *string) string {
	base, err := url.Parse(requestURL)
	if err != nil {
		return ""
	}
	var parts []string
	if nextCursor != nil && *nextCursor != "" {
		u := *base
		q := u.Query()
		q.Set("max_id", *nextCursor)
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

// linkHeaderWithNext builds a Link header with only rel="next" and max_id=cursor.
func linkHeaderWithNext(requestURL, cursor string) string {
	if cursor == "" {
		return ""
	}
	base, err := url.Parse(requestURL)
	if err != nil {
		return ""
	}
	q := base.Query()
	q.Set("max_id", cursor)
	base.RawQuery = q.Encode()
	return fmt.Sprintf("<%s>; rel=%q", base.String(), "next")
}

// optionalString returns a pointer to s if non-empty, otherwise nil.
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
