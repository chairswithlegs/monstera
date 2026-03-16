package vocab

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
)

func TestDomainFromIRI(t *testing.T) {
	require.Equal(t, "remote.example.com", DomainFromIRI("https://remote.example.com/users/alice"))
	require.Empty(t, DomainFromIRI("not-a-url"))
}

func TestStatusNoteID(t *testing.T) {
	t.Parallel()
	base := "https://example.com"

	t.Run("APID present", func(t *testing.T) {
		s := &domain.Status{ID: "01S", APID: "https://example.com/statuses/01S", URI: "https://example.com/statuses/01S"}
		require.Equal(t, "https://example.com/statuses/01S", StatusNoteID(s, base))
	})

	t.Run("URI fallback", func(t *testing.T) {
		s := &domain.Status{ID: "01S", URI: "https://example.com/statuses/01S"}
		require.Equal(t, "https://example.com/statuses/01S", StatusNoteID(s, base))
	})

	t.Run("constructed fallback", func(t *testing.T) {
		s := &domain.Status{ID: "01S"}
		require.Equal(t, "https://example.com/statuses/01S", StatusNoteID(s, base))
	})
}

func TestAccountActorID(t *testing.T) {
	t.Parallel()
	base := "https://example.com"

	t.Run("APID present", func(t *testing.T) {
		a := &domain.Account{Username: "alice", APID: "https://example.com/users/alice"}
		require.Equal(t, "https://example.com/users/alice", AccountActorID(a, base))
	})

	t.Run("constructed fallback", func(t *testing.T) {
		a := &domain.Account{Username: "alice"}
		require.Equal(t, "https://example.com/users/alice", AccountActorID(a, base))
	})
}
