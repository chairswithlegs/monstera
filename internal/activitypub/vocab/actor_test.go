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

func TestActorToRemoteFields(t *testing.T) {
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
		f := ActorToRemoteFields(actor)
		assert.Equal(t, "https://remote.example.com/users/bob", f.APID)
		assert.Equal(t, "bob", f.Username)
		assert.Equal(t, "remote.example.com", f.Domain)
		assert.Equal(t, "Bob", f.DisplayName)
		assert.Equal(t, "<p>Bio</p>", f.Note)
		assert.False(t, f.Bot)
	})

	t.Run("Service actor is Bot", func(t *testing.T) {
		t.Parallel()
		actor := &Actor{
			ID:                "https://remote.example.com/users/bot",
			Type:              ObjectTypeService,
			PreferredUsername: "bot",
		}
		f := ActorToRemoteFields(actor)
		assert.True(t, f.Bot)
	})

	t.Run("maps avatar and header URLs", func(t *testing.T) {
		t.Parallel()
		actor := &Actor{
			ID:                "https://remote.example.com/users/alice",
			Type:              ObjectTypePerson,
			PreferredUsername: "alice",
			Icon:              &Icon{Type: ObjectTypeImage, URL: "https://remote.example.com/avatars/alice.png"},
			Image:             &Icon{Type: ObjectTypeImage, URL: "https://remote.example.com/headers/alice.jpg"},
		}
		f := ActorToRemoteFields(actor)
		assert.Equal(t, "https://remote.example.com/avatars/alice.png", f.AvatarURL)
		assert.Equal(t, "https://remote.example.com/headers/alice.jpg", f.HeaderURL)
	})

	t.Run("no icon or image yields empty URLs", func(t *testing.T) {
		t.Parallel()
		actor := &Actor{
			ID:                "https://remote.example.com/users/bob",
			Type:              ObjectTypePerson,
			PreferredUsername: "bob",
		}
		f := ActorToRemoteFields(actor)
		assert.Empty(t, f.AvatarURL)
		assert.Empty(t, f.HeaderURL)
	})

	t.Run("SharedInboxURL populated from Endpoints.SharedInbox", func(t *testing.T) {
		t.Parallel()
		actor := &Actor{
			ID:                "https://remote.example.com/users/alice",
			Type:              ObjectTypePerson,
			PreferredUsername: "alice",
			Endpoints:         &Endpoints{SharedInbox: "https://remote.example.com/inbox"},
		}
		f := ActorToRemoteFields(actor)
		assert.Equal(t, "https://remote.example.com/inbox", f.SharedInboxURL)
	})

	t.Run("no Endpoints yields empty SharedInboxURL", func(t *testing.T) {
		t.Parallel()
		actor := &Actor{
			ID:                "https://remote.example.com/users/bob",
			Type:              ObjectTypePerson,
			PreferredUsername: "bob",
		}
		f := ActorToRemoteFields(actor)
		assert.Empty(t, f.SharedInboxURL)
	})

	t.Run("Endpoints with empty SharedInbox yields empty SharedInboxURL", func(t *testing.T) {
		t.Parallel()
		actor := &Actor{
			ID:                "https://remote.example.com/users/bob",
			Type:              ObjectTypePerson,
			PreferredUsername: "bob",
			Endpoints:         &Endpoints{},
		}
		f := ActorToRemoteFields(actor)
		assert.Empty(t, f.SharedInboxURL)
	})
}

func TestActorToRemoteFields_attachment_and_url(t *testing.T) {
	t.Parallel()
	actor := &Actor{
		ID:                "https://remote.example/users/alice",
		Type:              ObjectTypePerson,
		PreferredUsername: "alice",
		URL:               "https://remote.example/@alice",
		PublicKey:         PublicKey{ID: "https://remote.example/users/alice#main-key", PublicKeyPem: "pem"},
		Inbox:             "https://remote.example/users/alice/inbox",
		Outbox:            "https://remote.example/users/alice/outbox",
		Followers:         "https://remote.example/users/alice/followers",
		Following:         "https://remote.example/users/alice/following",
		Attachment: []PropertyValue{
			{Type: "PropertyValue", Name: "Website", Value: "<a href=\"https://example.com\">example.com</a>"},
			{Type: "PropertyValue", Name: "Pronouns", Value: "they/them"},
			{Type: "Note", Name: "Ignored", Value: "should be skipped"},
		},
	}
	fields := ActorToRemoteFields(actor)
	assert.Equal(t, "https://remote.example/@alice", fields.URL)
	assert.NotNil(t, fields.Fields)

	var decoded []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	require.NoError(t, json.Unmarshal(fields.Fields, &decoded))
	require.Len(t, decoded, 2)
	assert.Equal(t, "Website", decoded[0].Name)
	assert.Equal(t, "Pronouns", decoded[1].Name)
}
