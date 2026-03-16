package vocab

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	actor := Actor{
		Context:           DefaultContext,
		ID:                "https://example.com/users/alice",
		Type:              ObjectTypePerson,
		PreferredUsername: "alice",
		Name:              "Alice",
		URL:               "https://example.com/@alice",
		Inbox:             "https://example.com/users/alice/inbox",
		Outbox:            "https://example.com/users/alice/outbox",
		Followers:         "https://example.com/users/alice/followers",
		Following:         "https://example.com/users/alice/following",
		PublicKey: PublicKey{
			ID:           "https://example.com/users/alice#main-key",
			Owner:        "https://example.com/users/alice",
			PublicKeyPem: "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----",
		},
		ManuallyApprovesFollowers: false,
	}
	data, err := json.Marshal(actor)
	require.NoError(t, err)
	var decoded Actor
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, actor.ID, decoded.ID)
	assert.Equal(t, actor.PreferredUsername, decoded.PreferredUsername)
	assert.Equal(t, actor.PublicKey.ID, decoded.PublicKey.ID)
}

func TestActorToServiceInput(t *testing.T) {
	t.Parallel()

	t.Run("Person actor", func(t *testing.T) {
		t.Parallel()
		actor := &Actor{
			ID:                "https://remote.example.com/users/bob",
			Type:              ObjectTypePerson,
			PreferredUsername: "bob",
			Name:              "Bob",
			Summary:           "<p>Bio</p>",
			Inbox:             "https://remote.example.com/users/bob/inbox",
			Outbox:            "https://remote.example.com/users/bob/outbox",
			Followers:         "https://remote.example.com/users/bob/followers",
			Following:         "https://remote.example.com/users/bob/following",
			PublicKey:         PublicKey{PublicKeyPem: "-----BEGIN PUBLIC KEY-----"},
		}
		in := ActorToServiceInput(actor, []byte(`{}`))
		assert.Equal(t, "https://remote.example.com/users/bob", in.APID)
		assert.Equal(t, "bob", in.Username)
		assert.Equal(t, "remote.example.com", in.Domain)
		assert.Equal(t, "Bob", *in.DisplayName)
		assert.Equal(t, "<p>Bio</p>", *in.Note)
		assert.False(t, in.Bot)
		assert.Equal(t, []byte(`{}`), in.ApRaw)
	})

	t.Run("Service actor is Bot", func(t *testing.T) {
		t.Parallel()
		actor := &Actor{
			ID:                "https://remote.example.com/users/bot",
			Type:              ObjectTypeService,
			PreferredUsername: "bot",
		}
		in := ActorToServiceInput(actor, nil)
		assert.True(t, in.Bot)
	})
}
