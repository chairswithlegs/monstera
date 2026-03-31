package service

import (
	"regexp"
	"strings"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// compiledPhraseFilter holds a pre-compiled matcher for one v1 phrase filter (regex for whole-word, or phrase for substring).
type compiledPhraseFilter struct {
	re          *regexp.Regexp // non-nil when whole-word
	phraseLower string         // for substring match fallback
}

func compilePhraseFilters(filters []domain.UserFilter) []compiledPhraseFilter {
	out := make([]compiledPhraseFilter, 0, len(filters))
	for _, f := range filters {
		phrase := strings.TrimSpace(f.Phrase)
		if phrase == "" {
			continue
		}
		cf := compiledPhraseFilter{phraseLower: strings.ToLower(phrase)}
		if f.WholeWord {
			cf.re, _ = regexp.Compile(`(?i)\b` + regexp.QuoteMeta(phrase) + `\b`)
		}
		out = append(out, cf)
	}
	return out
}

// ApplyUserFiltersToEnriched filters out statuses whose content or content_warning
// matches any of the viewer's active filters (phrase, whole_word). Returns the
// slice of enriched statuses that pass the filters.
//
// We use post-filter (apply in the service layer after fetching) rather than
// query-time filtering because: phrase matching (e.g. content ILIKE '%phrase%')
// does not benefit from SQL indexes (leading wildcards prevent index use), so
// pushing it into the query does not improve performance. For small page sizes
// (20–40 statuses), filtering in Go after fetch is simpler and the cost is
// negligible.
func ApplyUserFiltersToEnriched(enriched []EnrichedStatus, filters []domain.UserFilter) []EnrichedStatus {
	if len(filters) == 0 {
		return enriched
	}
	compiled := compilePhraseFilters(filters)
	if len(compiled) == 0 {
		return enriched
	}
	out := make([]EnrichedStatus, 0, len(enriched))
	for i := range enriched {
		if !statusMatchesAnyFilter(enriched[i].Status, compiled) {
			out = append(out, enriched[i])
		}
	}
	return out
}

func statusMatchesAnyFilter(s *domain.Status, compiled []compiledPhraseFilter) bool {
	content := ""
	if s.Content != nil {
		content = *s.Content
	}
	cw := ""
	if s.ContentWarning != nil {
		cw = *s.ContentWarning
	}
	text := cw + " " + content
	textLower := strings.ToLower(text)
	for _, cf := range compiled {
		if cf.re != nil {
			if cf.re.MatchString(text) {
				return true
			}
		} else {
			if strings.Contains(textLower, cf.phraseLower) {
				return true
			}
		}
	}
	return false
}
