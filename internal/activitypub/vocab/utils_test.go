package vocab

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDomainFromIRI(t *testing.T) {
	require.Equal(t, "remote.example.com", DomainFromIRI("https://remote.example.com/users/alice"))
	require.Empty(t, DomainFromIRI("not-a-url"))
}
