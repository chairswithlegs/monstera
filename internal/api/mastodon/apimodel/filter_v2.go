package apimodel

import (
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// FilterKeyword is the Mastodon API v2 FilterKeyword entity.
type FilterKeyword struct {
	ID        string `json:"id"`
	Keyword   string `json:"keyword"`
	WholeWord bool   `json:"whole_word"`
}

// FilterStatus is the Mastodon API v2 FilterStatus entity.
type FilterStatus struct {
	ID       string `json:"id"`
	StatusID string `json:"status_id"`
}

// FilterV2 is the Mastodon API v2 Filter entity.
type FilterV2 struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Context      []string        `json:"context"`
	ExpiresAt    *string         `json:"expires_at"`
	FilterAction string          `json:"filter_action"`
	Keywords     []FilterKeyword `json:"keywords"`
	Statuses     []FilterStatus  `json:"statuses"`
}

// FilterResult is the Mastodon API FilterResult entity, embedded in Status.filtered.
type FilterResult struct {
	Filter         FilterV2 `json:"filter"`
	KeywordMatches []string `json:"keyword_matches"`
	StatusMatches  []string `json:"status_matches"`
}

// ToFilterV2 converts a domain UserFilterV2 to the Mastodon API FilterV2 shape.
func ToFilterV2(f *domain.UserFilterV2) FilterV2 {
	out := FilterV2{
		ID:           f.ID,
		Title:        f.Title,
		Context:      f.Context,
		FilterAction: f.FilterAction,
		Keywords:     make([]FilterKeyword, 0, len(f.Keywords)),
		Statuses:     make([]FilterStatus, 0, len(f.Statuses)),
	}
	if f.ExpiresAt != nil {
		s := f.ExpiresAt.UTC().Format(time.RFC3339)
		out.ExpiresAt = &s
	}
	for _, kw := range f.Keywords {
		out.Keywords = append(out.Keywords, FilterKeyword{
			ID:        kw.ID,
			Keyword:   kw.Keyword,
			WholeWord: kw.WholeWord,
		})
	}
	for _, fs := range f.Statuses {
		out.Statuses = append(out.Statuses, FilterStatus{
			ID:       fs.ID,
			StatusID: fs.StatusID,
		})
	}
	return out
}

// ToFilterKeyword converts a domain FilterKeyword to the API shape.
func ToFilterKeyword(k *domain.FilterKeyword) FilterKeyword {
	return FilterKeyword{
		ID:        k.ID,
		Keyword:   k.Keyword,
		WholeWord: k.WholeWord,
	}
}

// ToFilterStatus converts a domain FilterStatus to the API shape.
func ToFilterStatus(fs *domain.FilterStatus) FilterStatus {
	return FilterStatus{
		ID:       fs.ID,
		StatusID: fs.StatusID,
	}
}

// ToFilterResults converts domain FilterResult slice to API FilterResult slice.
func ToFilterResults(results []domain.FilterResult) []FilterResult {
	out := make([]FilterResult, 0, len(results))
	for _, r := range results {
		fr := FilterResult{
			Filter:         ToFilterV2(&r.Filter),
			KeywordMatches: r.KeywordMatches,
			StatusMatches:  r.StatusMatches,
		}
		if fr.KeywordMatches == nil {
			fr.KeywordMatches = []string{}
		}
		if fr.StatusMatches == nil {
			fr.StatusMatches = []string{}
		}
		out = append(out, fr)
	}
	return out
}
