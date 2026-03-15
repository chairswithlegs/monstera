package activitypub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestRemoteAccountResolver_ResolveRemoteAccount_invalidAcct(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := &mockAccountService{GetLocalByUsernameFunc: func(ctx context.Context, username string) (*domain.Account, error) {
		return nil, domain.ErrNotFound
	}, GetByUsernameFunc: func(ctx context.Context, username string, accountDomain *string) (*domain.Account, error) {
		return nil, domain.ErrNotFound
	}, CreateOrUpdateRemoteAccountFunc: func(ctx context.Context, in service.CreateOrUpdateRemoteInput) (*domain.Account, error) {
		return nil, domain.ErrNotFound
	}}
	cfg := &config.Config{InstanceDomain: "example.com"}
	r := NewRemoteAccountResolver(svc, cfg)

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
	svc := service.NewAccountService(fake, "https://example.com")

	cfg := &config.Config{InstanceDomain: "example.com"}
	r := NewRemoteAccountResolver(svc, cfg)

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
	svc := service.NewAccountService(testutil.NewFakeStore(), "https://example.com")
	r := NewRemoteAccountResolver(svc, &config.Config{InstanceDomain: "example.com"})

	_, err := r.ResolveRemoteAccount(ctx, "nobody@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRemoteAccountResolver_ResolveRemoteAccount_remote_cached(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := service.NewAccountService(fake, "https://example.com")
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

	r := NewRemoteAccountResolver(svc, &config.Config{InstanceDomain: "example.com"})
	acc, err := r.ResolveRemoteAccount(ctx, "bob@remote.example")
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.Equal(t, "bob", acc.Username)
	assert.Equal(t, "01bob", acc.ID)
}

func TestRemoteAccountResolver_ResolveRemoteAccount_stale_cache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := service.NewAccountService(fake, "https://example.com")
	domainPtr := testutil.StrPtr("remote.example")
	fake.SeedAccount(&domain.Account{
		ID:        "01bob",
		Username:  "bob",
		Domain:    domainPtr,
		APID:      "https://remote.example/users/bob",
		UpdatedAt: time.Now().Add(-2 * staleRemoteActorDuration),
	})

	mockWebFingerResponse := JRD{
		Subject: "https://remote.example/users/bob",
		Links: []JRDLink{
			{Rel: "self", Type: "application/activity+json", Href: "https://remote.example/users/bob"},
		},
	}
	mockWebFingerResponseJSON, err := json.Marshal(mockWebFingerResponse)
	require.NoError(t, err)

	mockActorResponse := vocab.Actor{
		ID:                "https://remote.example/users/bob",
		Type:              vocab.ObjectTypePerson,
		PreferredUsername: "bob",
		Name:              "Bobby",
	}
	mockActorResponseJSON, err := json.Marshal(mockActorResponse)
	require.NoError(t, err)

	transport := &roundTripFunc{fn: func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/.well-known/webfinger" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/jrd+json"}},
				Body:       io.NopCloser(bytes.NewReader(mockWebFingerResponseJSON)),
			}, nil
		}
		if req.URL.Path == "/users/bob" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/activity+json"}},
				Body:       io.NopCloser(bytes.NewReader(mockActorResponseJSON)),
			}, nil
		}
		return nil, fmt.Errorf("unexpected request: %s", req.URL.Path)
	}}

	r := &RemoteAccountResolver{
		accounts:       svc,
		instanceDomain: "example.com",
		httpClient:     &http.Client{Transport: transport},
	}

	acc, err := r.ResolveRemoteAccount(ctx, "bob@remote.example")
	require.NoError(t, err)
	assert.Equal(t, "01bob", acc.ID)
	assert.Equal(t, "Bobby", *acc.DisplayName)
}

func TestRemoteAccountResolver_ResolveRemoteAccount_success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	svc := service.NewAccountService(fake, "https://example.com")

	mockWebFingerResponse := JRD{
		Subject: "https://remote.example/users/bob",
		Links: []JRDLink{
			{Rel: "self", Type: "application/activity+json", Href: "https://remote.example/users/bob"},
		},
	}
	mockWebFingerResponseJSON, err := json.Marshal(mockWebFingerResponse)
	require.NoError(t, err)

	mockActorResponse := vocab.Actor{
		ID:                "https://remote.example/users/bob",
		Type:              vocab.ObjectTypePerson,
		PreferredUsername: "bob",
	}
	mockActorResponseJSON, err := json.Marshal(mockActorResponse)
	require.NoError(t, err)

	transport := &roundTripFunc{fn: func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/.well-known/webfinger" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/jrd+json"}},
				Body:       io.NopCloser(bytes.NewReader(mockWebFingerResponseJSON)),
			}, nil
		}
		if req.URL.Path == "/users/bob" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/activity+json"}},
				Body:       io.NopCloser(bytes.NewReader(mockActorResponseJSON)),
			}, nil
		}
		return nil, fmt.Errorf("unexpected request: %s", req.URL.Path)
	}}

	r := &RemoteAccountResolver{
		accounts:       svc,
		instanceDomain: "example.com",
		httpClient:     &http.Client{Transport: transport},
	}

	acc, err := r.ResolveRemoteAccount(ctx, "bob@remote.example")
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.Equal(t, "bob", acc.Username)
}

func TestRemoteAccountResolver_FetchActor_404(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	transport := &roundTripFunc{fn: func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil
	}}
	r := &RemoteAccountResolver{
		accounts:       &mockAccountService{},
		instanceDomain: "example.com",
		httpClient:     &http.Client{Transport: transport},
	}
	_, err := r.fetchActor(ctx, "https://remote.example/users/missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
}

func TestRemoteAccountResolver_FetchActor_success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	transport := &roundTripFunc{fn: func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/activity+json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(mockActorResponseBody))),
		}, nil
	}}
	r := &RemoteAccountResolver{
		accounts:       &mockAccountService{},
		instanceDomain: "example.com",
		httpClient:     &http.Client{Transport: transport},
	}
	out, err := r.fetchActor(ctx, "https://example.com/users/alice")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, vocab.ObjectTypePerson, out.Type)
	assert.Equal(t, "https://example.com/ap/users/115871897412638518", out.ID)
	assert.Equal(t, vocab.ObjectTypePerson, out.Type)
	assert.Equal(t, "alice", out.PreferredUsername)
	assert.Equal(t, "Alice", out.Name)
	assert.Equal(t, "https://cdn.masto.host/examplecom/accounts/avatars/115/871/897/412/638/518/original/82ac2397bf2dcde1.png", out.Icon.URL)
	assert.Equal(t, "https://example.com/ap/users/115871897412638518/inbox", out.Inbox)
	assert.Equal(t, "https://example.com/ap/users/115871897412638518/outbox", out.Outbox)
	assert.Equal(t, "https://example.com/ap/users/115871897412638518/followers", out.Followers)
	assert.Equal(t, "https://example.com/ap/users/115871897412638518/following", out.Following)
	assert.Equal(t, "https://example.com/ap/users/115871897412638518#main-key", out.PublicKey.ID)
	assert.Equal(t, "https://example.com/ap/users/115871897412638518", out.PublicKey.Owner)
	assert.Equal(t, "-----BEGIN PUBLIC KEY-----nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnqsKxoOwP/8Ic01DuvIVnglLrxMHWswc7hBSnoYuyzCLt+Iok/6kDGzKKalONU03uxcMDeXX7QwdSWH6mWQzwn6HtjIBCKUSk32H3MPgEyyZR0HC6jP2YbrJ2QZ8S+8oUNQ9yt8gJKzUKdE2QjRVP5ndvLCFbqcILE/64g9468F0gogccTjSYcTBYMzLkgM5bEAvlH5XvOgov+Ck0PkNnuBn4hx9Oc9zuTRTRKz9Ps81ZcmDNNOU33FEp0UKDC//NxmDBQtu8OcPlNG7u6ZxWpFTnxc8JH5gyL6CeursnqGgR+tmXJQ7Y0emOgkltdjT/3sOglJCezXAyCZ2h5NtJjhwwn3QIDAQABn-----END PUBLIC KEY-----n", out.PublicKey.PublicKeyPem)
	assert.False(t, out.ManuallyApprovesFollowers)
	assert.Equal(t, "2026-01-10T00:00:00Z", out.Published)
}

type roundTripFunc struct {
	fn func(*http.Request) (*http.Response, error)
}

func (f *roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f.fn(req)
}

var mockActorResponseBody = `{
    "@context": [
        "https://www.w3.org/ns/activitystreams",
        "https://w3id.org/security/v1",
        {
            "manuallyApprovesFollowers": "as:manuallyApprovesFollowers",
            "toot": "http://joinmastodon.org/ns#",
            "featured": {
                "@id": "toot:featured",
                "@type": "@id"
            },
            "featuredTags": {
                "@id": "toot:featuredTags",
                "@type": "@id"
            },
            "alsoKnownAs": {
                "@id": "as:alsoKnownAs",
                "@type": "@id"
            },
            "movedTo": {
                "@id": "as:movedTo",
                "@type": "@id"
            },
            "schema": "http://schema.org#",
            "PropertyValue": "schema:PropertyValue",
            "value": "schema:value",
            "discoverable": "toot:discoverable",
            "suspended": "toot:suspended",
            "memorial": "toot:memorial",
            "indexable": "toot:indexable",
            "attributionDomains": {
                "@id": "toot:attributionDomains",
                "@type": "@id"
            },
            "focalPoint": {
                "@container": "@list",
                "@id": "toot:focalPoint"
            }
        }
    ],
    "id": "https://example.com/ap/users/115871897412638518",
    "type": "Person",
    "following": "https://example.com/ap/users/115871897412638518/following",
    "followers": "https://example.com/ap/users/115871897412638518/followers",
    "inbox": "https://example.com/ap/users/115871897412638518/inbox",
    "outbox": "https://example.com/ap/users/115871897412638518/outbox",
    "featured": "https://example.com/ap/users/115871897412638518/collections/featured",
    "featuredTags": "https://example.com/ap/users/115871897412638518/collections/tags",
    "preferredUsername": "alice",
    "name": "Alice",
    "summary": "",
    "url": "https://example.com/@alice",
    "manuallyApprovesFollowers": false,
    "discoverable": true,
    "indexable": true,
    "published": "2026-01-10T00:00:00Z",
    "memorial": false,
    "publicKey": {
        "id": "https://example.com/ap/users/115871897412638518#main-key",
        "owner": "https://example.com/ap/users/115871897412638518",
        "publicKeyPem": "-----BEGIN PUBLIC KEY-----nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnqsKxoOwP/8Ic01DuvIVnglLrxMHWswc7hBSnoYuyzCLt+Iok/6kDGzKKalONU03uxcMDeXX7QwdSWH6mWQzwn6HtjIBCKUSk32H3MPgEyyZR0HC6jP2YbrJ2QZ8S+8oUNQ9yt8gJKzUKdE2QjRVP5ndvLCFbqcILE/64g9468F0gogccTjSYcTBYMzLkgM5bEAvlH5XvOgov+Ck0PkNnuBn4hx9Oc9zuTRTRKz9Ps81ZcmDNNOU33FEp0UKDC//NxmDBQtu8OcPlNG7u6ZxWpFTnxc8JH5gyL6CeursnqGgR+tmXJQ7Y0emOgkltdjT/3sOglJCezXAyCZ2h5NtJjhwwn3QIDAQABn-----END PUBLIC KEY-----n"
    },
    "tag": [],
    "attachment": [],
    "endpoints": {
        "sharedInbox": "https://example.com/inbox"
    },
    "icon": {
        "type": "Image",
        "mediaType": "image/png",
        "url": "https://cdn.masto.host/examplecom/accounts/avatars/115/871/897/412/638/518/original/82ac2397bf2dcde1.png"
    }
}`
