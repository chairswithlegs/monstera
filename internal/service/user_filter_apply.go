package service

import (
	"regexp"
	"strings"

	"github.com/chairswithlegs/monstera/internal/domain"
)

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
	out := make([]EnrichedStatus, 0, len(enriched))
	for i := range enriched {
		if !statusMatchesAnyFilter(enriched[i].Status, filters) {
			out = append(out, enriched[i])
		}
	}
	return out
}

func statusMatchesAnyFilter(s *domain.Status, filters []domain.UserFilter) bool {
	content := ""
	if s.Content != nil {
		content = *s.Content
	}
	cw := ""
	if s.ContentWarning != nil {
		cw = *s.ContentWarning
	}
	text := cw + " " + content
	for _, f := range filters {
		if phraseMatchesText(f.Phrase, text, f.WholeWord) {
			return true
		}
	}
	return false
}

func phraseMatchesText(phrase, text string, wholeWord bool) bool {
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return false
	}
	textLower := strings.ToLower(text)
	phraseLower := strings.ToLower(phrase)
	if wholeWord {
		// Word boundary match: phrase must appear as a whole word.
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(phrase) + `\b`)
		if err != nil {
			return strings.Contains(textLower, phraseLower)
		}
		return re.MatchString(text)
	}
	return strings.Contains(textLower, phraseLower)
}
