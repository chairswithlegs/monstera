package federation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubjectToActivityType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "create", subjectToActivityType("federation.deliver.create"))
	assert.Equal(t, "accept", subjectToActivityType("federation.deliver.accept"))
	assert.Equal(t, "unknown", subjectToActivityType("other.subject"))
}

func TestDomainFromURL(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "remote.example.com", domainFromURL("https://remote.example.com/inbox"))
	assert.Empty(t, domainFromURL("not-a-url"))
}
