package activitypub

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestRemoteAccountResolver_ResolveRemoteAccount_invalidAcct(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	r := NewRemoteAccountResolver(fake, "example.com", cfg)

	for _, acct := range []string{"", "invalid", "no-at", "@nodomain", "user@"} {
		_, err := r.ResolveRemoteAccount(ctx, acct)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid acct")
	}
}

func TestRemoteAccountResolver_ResolveRemoteAccount_local(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	r := NewRemoteAccountResolver(fake, "example.com", cfg)

	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:           "01alice",
		Username:     "alice",
		Domain:       nil,
		APID:         "https://example.com/users/alice",
		InboxURL:     "https://example.com/users/alice/inbox",
		OutboxURL:    "https://example.com/users/alice/outbox",
		FollowersURL: "https://example.com/users/alice/followers",
		FollowingURL: "https://example.com/users/alice/following",
	})
	require.NoError(t, err)

	acc, err := r.ResolveRemoteAccount(ctx, "alice@example.com")
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.Equal(t, "alice", acc.Username)
	assert.Equal(t, "01alice", acc.ID)
}

func TestRemoteAccountResolver_ResolveRemoteAccount_local_notFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	r := NewRemoteAccountResolver(fake, "example.com", &config.Config{})

	_, err := r.ResolveRemoteAccount(ctx, "nobody@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRemoteAccountResolver_ResolveRemoteAccount_remote_cached(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	domainPtr := testutil.StrPtr("remote.example")
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:           "01bob",
		Username:     "bob",
		Domain:       domainPtr,
		APID:         "https://remote.example/users/bob",
		InboxURL:     "https://remote.example/users/bob/inbox",
		OutboxURL:    "https://remote.example/users/bob/outbox",
		FollowersURL: "https://remote.example/users/bob/followers",
		FollowingURL: "https://remote.example/users/bob/following",
	})
	require.NoError(t, err)

	r := NewRemoteAccountResolver(fake, "example.com", &config.Config{})
	acc, err := r.ResolveRemoteAccount(ctx, "bob@remote.example")
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.Equal(t, "bob", acc.Username)
	assert.Equal(t, "01bob", acc.ID)
}

func TestRemoteAccountResolver_FetchActor_success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	actor := Actor{
		ID:                "https://remote.example/users/carol",
		Type:              "Person",
		PreferredUsername: "carol",
		Name:              "Carol",
		Inbox:             "https://remote.example/users/carol/inbox",
		Outbox:            "https://remote.example/users/carol/outbox",
		Followers:         "https://remote.example/users/carol/followers",
		Following:         "https://remote.example/users/carol/following",
		PublicKey:         PublicKey{ID: "https://remote.example/users/carol#main-key", Owner: "https://remote.example/users/carol", PublicKeyPem: "x"},
	}
	body, _ := json.Marshal(actor)
	transport := &roundTripFunc{fn: func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/activity+json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	}}
	r := &RemoteAccountResolver{
		store:          testutil.NewFakeStore(),
		instanceDomain: "example.com",
		httpClient:     &http.Client{Transport: transport},
	}
	out, err := r.FetchActor(ctx, "https://remote.example/users/carol")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "Person", out.Type)
	assert.Equal(t, "carol", out.PreferredUsername)
	assert.Equal(t, "Carol", out.Name)
}

func TestRemoteAccountResolver_FetchActor_404(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	transport := &roundTripFunc{fn: func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil
	}}
	r := &RemoteAccountResolver{
		store:          testutil.NewFakeStore(),
		instanceDomain: "example.com",
		httpClient:     &http.Client{Transport: transport},
	}
	_, err := r.FetchActor(ctx, "https://remote.example/users/missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
}

type roundTripFunc struct {
	fn func(*http.Request) (*http.Response, error)
}

func (f *roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f.fn(req)
}
