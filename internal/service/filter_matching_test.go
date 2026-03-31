package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
)

func TestComputeFilterResults(t *testing.T) {
	t.Parallel()

	makeFilter := func(id, keyword string, wholeWord bool) domain.UserFilter {
		return domain.UserFilter{
			ID:           id,
			FilterAction: "warn",
			Keywords: []domain.FilterKeyword{
				{ID: "kw1", Keyword: keyword, WholeWord: wholeWord},
			},
		}
	}

	tests := []struct {
		name          string
		filters       []domain.UserFilter
		statusID      string
		content       string
		cw            string
		wantFilterIDs []string
		wantKWMatches [][]string
	}{
		{
			name:    "no filters",
			filters: nil,
			content: "hello world",
		},
		{
			name:          "substring match",
			filters:       []domain.UserFilter{makeFilter("f1", "hello", false)},
			content:       "say hello there",
			wantFilterIDs: []string{"f1"},
			wantKWMatches: [][]string{{"hello"}},
		},
		{
			name:    "substring no match",
			filters: []domain.UserFilter{makeFilter("f1", "hello", false)},
			content: "goodbye world",
		},
		{
			name:          "substring case-insensitive",
			filters:       []domain.UserFilter{makeFilter("f1", "Hello", false)},
			content:       "say HELLO there",
			wantFilterIDs: []string{"f1"},
			wantKWMatches: [][]string{{"Hello"}},
		},
		{
			name:          "whole-word match",
			filters:       []domain.UserFilter{makeFilter("f1", "cat", true)},
			content:       "my cat sat on the mat",
			wantFilterIDs: []string{"f1"},
			wantKWMatches: [][]string{{"cat"}},
		},
		{
			name:    "whole-word no match on substring",
			filters: []domain.UserFilter{makeFilter("f1", "cat", true)},
			content: "concatenate all the things",
		},
		{
			name:          "whole-word match with punctuation boundary",
			filters:       []domain.UserFilter{makeFilter("f1", "cat", true)},
			content:       "look: cat!",
			wantFilterIDs: []string{"f1"},
			wantKWMatches: [][]string{{"cat"}},
		},
		{
			name:          "match in content warning",
			filters:       []domain.UserFilter{makeFilter("f1", "spoiler", false)},
			content:       "some content",
			cw:            "spoiler ahead",
			wantFilterIDs: []string{"f1"},
			wantKWMatches: [][]string{{"spoiler"}},
		},
		{
			name: "multiple filters, only one matches",
			filters: []domain.UserFilter{
				makeFilter("f1", "cats", false),
				makeFilter("f2", "dogs", false),
			},
			content:       "I love cats",
			wantFilterIDs: []string{"f1"},
			wantKWMatches: [][]string{{"cats"}},
		},
		{
			name: "both filters match",
			filters: []domain.UserFilter{
				makeFilter("f1", "cats", false),
				makeFilter("f2", "dogs", false),
			},
			content:       "cats and dogs",
			wantFilterIDs: []string{"f1", "f2"},
			wantKWMatches: [][]string{{"cats"}, {"dogs"}},
		},
		{
			name: "status ID match",
			filters: []domain.UserFilter{
				{
					ID:           "f1",
					FilterAction: "warn",
					Statuses:     []domain.FilterStatus{{ID: "fs1", FilterID: "f1", StatusID: "st999"}},
				},
			},
			statusID:      "st999",
			wantFilterIDs: []string{"f1"},
		},
		{
			name: "status ID no match",
			filters: []domain.UserFilter{
				{
					ID:           "f1",
					FilterAction: "warn",
					Statuses:     []domain.FilterStatus{{ID: "fs1", FilterID: "f1", StatusID: "st999"}},
				},
			},
			statusID: "st000",
		},
		{
			name:    "empty content and cw",
			filters: []domain.UserFilter{makeFilter("f1", "hello", false)},
			content: "",
			cw:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			results := computeFilterResults(tc.filters, tc.statusID, tc.content, tc.cw)

			if len(tc.wantFilterIDs) == 0 {
				assert.Empty(t, results)
				return
			}

			require.Len(t, results, len(tc.wantFilterIDs))
			for i, want := range tc.wantFilterIDs {
				assert.Equal(t, want, results[i].Filter.ID)
				if tc.wantKWMatches != nil {
					assert.Equal(t, tc.wantKWMatches[i], results[i].KeywordMatches)
				}
			}
		})
	}
}

func TestCompileFilters_PrecompiledReuse(t *testing.T) {
	t.Parallel()
	// Verifies that compileFilters produces matchers reusable across multiple statuses.
	filters := []domain.UserFilter{
		{
			ID:           "f1",
			FilterAction: "warn",
			Keywords:     []domain.FilterKeyword{{ID: "kw1", Keyword: "badword", WholeWord: true}},
		},
	}
	compiled := compileFilters(filters)

	r1 := matchCompiledFilters(compiled, "s1", "this is a badword example", "")
	assert.Len(t, r1, 1)

	r2 := matchCompiledFilters(compiled, "s2", "nothing relevant here", "")
	assert.Empty(t, r2)

	r3 := matchCompiledFilters(compiled, "s3", "badwords is not a whole-word match", "")
	assert.Empty(t, r3)
}
