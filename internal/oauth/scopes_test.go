package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_empty(t *testing.T) {
	ss := Parse("")
	require.NotNil(t, ss)
	assert.Empty(t, ss)
	assert.Empty(t, ss.String())
}

func TestParse_singleTopLevel(t *testing.T) {
	ss := Parse("read")
	require.NotNil(t, ss)
	assert.True(t, ss.HasScope("read"))
	assert.True(t, ss.HasScope("read:accounts"))
	assert.True(t, ss.HasScope("read:statuses"))
	assert.False(t, ss.HasScope("write"))
	assert.True(t, ss.HasAll("read", "read:notifications"))
}

func TestParse_multipleWithExpansion(t *testing.T) {
	ss := Parse("read write")
	assert.True(t, ss.HasScope("read"))
	assert.True(t, ss.HasScope("write"))
	assert.True(t, ss.HasScope("read:statuses"))
	assert.True(t, ss.HasScope("write:statuses"))
	assert.True(t, ss.HasAll("read:accounts", "write:media"))
}

func TestParse_unknownScopesDropped(t *testing.T) {
	ss := Parse("read unknown:scope phase2:feature")
	assert.True(t, ss.HasScope("read"))
	assert.False(t, ss.HasScope("unknown:scope"))
	assert.False(t, ss.HasScope("phase2:feature"))
	assert.Contains(t, ss.String(), "read")
	assert.NotContains(t, ss.String(), "unknown")
}

func TestParse_whitespaceAndTrim(t *testing.T) {
	ss := Parse("  read   write  ")
	assert.True(t, ss.HasScope("read"))
	assert.True(t, ss.HasScope("write"))
}

func TestNormalize(t *testing.T) {
	assert.Equal(t, Parse("read").String(), Normalize("read"))
	assert.Equal(t, Parse("write read").String(), Normalize("read write"))
	assert.NotEmpty(t, Normalize("read"))
	assert.True(t, Parse(Normalize("read")).HasScope("read:accounts"))
}

func TestScopeSet_String_sorted(t *testing.T) {
	ss := Parse("write read")
	out := ss.String()
	assert.Contains(t, out, "read")
	assert.Contains(t, out, "write")
	assert.Equal(t, out, ss.String(), "String() should be deterministic and sorted")
}

func TestScopeSet_Intersect(t *testing.T) {
	a := Parse("read write")
	b := Parse("read admin:read")
	got := a.Intersect(b)
	assert.True(t, got.HasScope("read"))
	assert.False(t, got.HasScope("write"))
	assert.False(t, got.HasScope("admin:read"))
	assert.True(t, got.HasScope("read:statuses"))
}

func TestScopeSet_Intersect_empty(t *testing.T) {
	a := Parse("read")
	b := Parse("write")
	got := a.Intersect(b)
	assert.Empty(t, got)
}

func TestScopeSet_HasAll(t *testing.T) {
	ss := Parse("read write")
	assert.True(t, ss.HasAll("read", "write"))
	assert.True(t, ss.HasAll("read:accounts", "write:statuses"))
	assert.False(t, ss.HasAll("read", "admin:read"))
	assert.True(t, ss.HasAll())
}

func TestFollow_expansion(t *testing.T) {
	ss := Parse("follow")
	assert.True(t, ss.HasScope("follow"))
	assert.True(t, ss.HasScope("read:follows"))
	assert.True(t, ss.HasScope("write:follows"))
	assert.True(t, ss.HasScope("read:blocks"))
	assert.True(t, ss.HasScope("write:mutes"))
}

func TestPush_standalone(t *testing.T) {
	ss := Parse("push")
	assert.True(t, ss.HasScope("push"))
	assert.Equal(t, "push", ss.String())
}
