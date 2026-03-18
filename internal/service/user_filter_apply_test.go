package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/chairswithlegs/monstera/internal/domain"
)

func TestApplyUserFiltersToEnriched(t *testing.T) {
	t.Parallel()

	sp := func(s string) *string { return &s }

	mkStatus := func(id, content string, cw *string) EnrichedStatus {
		return EnrichedStatus{
			Status: &domain.Status{
				ID:             id,
				Content:        &content,
				ContentWarning: cw,
			},
		}
	}

	tests := []struct {
		name     string
		statuses []EnrichedStatus
		filters  []domain.UserFilter
		wantIDs  []string
	}{
		{
			name:     "no filters",
			statuses: []EnrichedStatus{mkStatus("1", "hello world", nil)},
			filters:  nil,
			wantIDs:  []string{"1"},
		},
		{
			name:     "substring match",
			statuses: []EnrichedStatus{mkStatus("1", "contains badword here", nil), mkStatus("2", "clean text", nil)},
			filters:  []domain.UserFilter{{Phrase: "badword", WholeWord: false}},
			wantIDs:  []string{"2"},
		},
		{
			name:     "whole word match",
			statuses: []EnrichedStatus{mkStatus("1", "this is bad stuff", nil)},
			filters:  []domain.UserFilter{{Phrase: "bad", WholeWord: true}},
			wantIDs:  nil,
		},
		{
			name:     "whole word no match",
			statuses: []EnrichedStatus{mkStatus("1", "badger is an animal", nil)},
			filters:  []domain.UserFilter{{Phrase: "bad", WholeWord: true}},
			wantIDs:  []string{"1"},
		},
		{
			name:     "content warning match",
			statuses: []EnrichedStatus{mkStatus("1", "safe content", sp("spoiler alert"))},
			filters:  []domain.UserFilter{{Phrase: "spoiler", WholeWord: false}},
			wantIDs:  nil,
		},
		{
			name:     "case insensitive",
			statuses: []EnrichedStatus{mkStatus("1", "Hello WORLD", nil)},
			filters:  []domain.UserFilter{{Phrase: "hello world", WholeWord: false}},
			wantIDs:  nil,
		},
		{
			name:     "empty phrase skipped",
			statuses: []EnrichedStatus{mkStatus("1", "anything", nil)},
			filters:  []domain.UserFilter{{Phrase: "", WholeWord: false}},
			wantIDs:  []string{"1"},
		},
		{
			name: "all filtered",
			statuses: []EnrichedStatus{
				mkStatus("1", "spam message", nil),
				mkStatus("2", "more spam", nil),
			},
			filters: []domain.UserFilter{{Phrase: "spam", WholeWord: false}},
			wantIDs: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ApplyUserFiltersToEnriched(tc.statuses, tc.filters)
			var gotIDs []string
			for _, es := range got {
				gotIDs = append(gotIDs, es.Status.ID)
			}
			assert.Equal(t, tc.wantIDs, gotIDs)
		})
	}
}
