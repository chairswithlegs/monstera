package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

const testFollowingPath = "/following"

type fakeStatusService struct {
	service.StatusService
	statuses map[string]*domain.Status
}

func (f *fakeStatusService) GetByAPID(_ context.Context, apID string) (*domain.Status, error) {
	if s, ok := f.statuses[apID]; ok {
		return s, nil
	}
	return nil, domain.ErrNotFound
}

type fakeRemoteStatusWriteService struct {
	service.RemoteStatusWriteService
	created []service.CreateRemoteStatusInput
}

func (f *fakeRemoteStatusWriteService) CreateRemote(_ context.Context, in service.CreateRemoteStatusInput) (*domain.Status, error) {
	f.created = append(f.created, in)
	return &domain.Status{ID: uid.New()}, nil
}

func newTestBackfillWorker(fs *testutil.FakeStore, remoteStatuses service.RemoteStatusWriteService, statusSvc service.StatusService) *BackfillWorker {
	return &BackfillWorker{
		accounts:       service.NewAccountService(fs, "https://local.example"),
		backfill:       service.NewBackfillService(fs, nil, time.Hour),
		remoteStatuses: remoteStatuses,
		remoteFollows:  service.NewRemoteFollowService(fs),
		statuses:       statusSvc,
		instanceDomain: "local.example",
		maxPages:       2,
		maxItems:       0,
		cooldown:       time.Hour,
	}
}

func makeCreateNoteActivity(noteID, activityID, actorAPID string) (json.RawMessage, error) {
	note := vocab.Note{
		ID:           noteID,
		Type:         vocab.ObjectTypeNote,
		Content:      "<p>Hello from backfill</p>",
		AttributedTo: actorAPID,
		To:           []string{"https://www.w3.org/ns/activitystreams#Public"},
	}
	noteJSON, err := json.Marshal(note)
	if err != nil {
		return nil, err
	}
	activity := vocab.Activity{
		Type:      vocab.ObjectTypeCreate,
		ID:        activityID,
		Actor:     actorAPID,
		ObjectRaw: noteJSON,
	}
	return json.Marshal(activity)
}

func TestBackfillWorker_processItems_CreateNote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	statusSvc := &fakeStatusService{statuses: map[string]*domain.Status{}}
	remoteWrites := &fakeRemoteStatusWriteService{}

	w := newTestBackfillWorker(fs, remoteWrites, statusSvc)

	noteID := "https://remote.example/notes/1"
	actJSON, err := makeCreateNoteActivity(noteID, "https://remote.example/activities/1", account.APID)
	require.NoError(t, err)

	w.processItems(ctx, account, []json.RawMessage{actJSON})

	require.Len(t, remoteWrites.created, 1)
	assert.Equal(t, account.ID, remoteWrites.created[0].AccountID)
	assert.Equal(t, noteID, remoteWrites.created[0].APID)
}

func TestBackfillWorker_processItems_SkipsDuplicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	noteID := "https://remote.example/notes/existing"
	statusSvc := &fakeStatusService{
		statuses: map[string]*domain.Status{
			noteID: {ID: "existing-status-id", APID: noteID},
		},
	}
	remoteWrites := &fakeRemoteStatusWriteService{}

	w := newTestBackfillWorker(fs, remoteWrites, statusSvc)

	actJSON, err := makeCreateNoteActivity(noteID, "https://remote.example/activities/dup", account.APID)
	require.NoError(t, err)

	w.processItems(ctx, account, []json.RawMessage{actJSON})

	assert.Empty(t, remoteWrites.created, "should not create duplicate status")
}

func TestBackfillWorker_processItems_SkipsPrivateVisibility(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	statusSvc := &fakeStatusService{statuses: map[string]*domain.Status{}}
	remoteWrites := &fakeRemoteStatusWriteService{}

	w := newTestBackfillWorker(fs, remoteWrites, statusSvc)

	// Private note: To is followers URL only.
	note := vocab.Note{
		ID:           "https://remote.example/notes/private",
		Type:         vocab.ObjectTypeNote,
		Content:      "<p>Private post</p>",
		AttributedTo: account.APID,
		To:           []string{account.FollowersURL},
	}
	noteJSON, err := json.Marshal(note)
	require.NoError(t, err)

	activity := vocab.Activity{
		Type:      vocab.ObjectTypeCreate,
		ID:        "https://remote.example/activities/priv",
		Actor:     account.APID,
		ObjectRaw: noteJSON,
	}
	actJSON, err := json.Marshal(activity)
	require.NoError(t, err)

	w.processItems(ctx, account, []json.RawMessage{actJSON})

	assert.Empty(t, remoteWrites.created, "should not backfill private statuses")
}

func TestBackfillWorker_processItems_BareNote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	statusSvc := &fakeStatusService{statuses: map[string]*domain.Status{}}
	remoteWrites := &fakeRemoteStatusWriteService{}

	w := newTestBackfillWorker(fs, remoteWrites, statusSvc)

	// Some implementations return bare Notes in outbox instead of Create{Note}.
	note := vocab.Note{
		ID:           "https://remote.example/notes/bare",
		Type:         vocab.ObjectTypeNote,
		Content:      "<p>Bare note</p>",
		AttributedTo: account.APID,
		To:           []string{"https://www.w3.org/ns/activitystreams#Public"},
	}
	noteJSON, err := json.Marshal(note)
	require.NoError(t, err)

	w.processItems(ctx, account, []json.RawMessage{noteJSON})

	require.Len(t, remoteWrites.created, 1)
	assert.Equal(t, account.ID, remoteWrites.created[0].AccountID)
}

func TestBackfillWorker_processItems_PreservesPublishedTimestamp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	statusSvc := &fakeStatusService{statuses: map[string]*domain.Status{}}
	remoteWrites := &fakeRemoteStatusWriteService{}

	w := newTestBackfillWorker(fs, remoteWrites, statusSvc)

	published := "2023-06-15T12:00:00Z"
	note := vocab.Note{
		ID:           "https://remote.example/notes/timestamped",
		Type:         vocab.ObjectTypeNote,
		Content:      "<p>Old post</p>",
		AttributedTo: account.APID,
		To:           []string{"https://www.w3.org/ns/activitystreams#Public"},
		Published:    published,
	}
	noteJSON, err := json.Marshal(note)
	require.NoError(t, err)
	activity := vocab.Activity{
		Type:      vocab.ObjectTypeCreate,
		ID:        "https://remote.example/activities/ts1",
		Actor:     account.APID,
		ObjectRaw: noteJSON,
	}
	actJSON, err := json.Marshal(activity)
	require.NoError(t, err)

	w.processItems(ctx, account, []json.RawMessage{actJSON})

	require.Len(t, remoteWrites.created, 1)
	in := remoteWrites.created[0]
	require.NotNil(t, in.PublishedAt, "PublishedAt should be set from note.Published")
	assert.Equal(t, "2023-06-15T12:00:00Z", in.PublishedAt.UTC().Format("2006-01-02T15:04:05Z"))
}

func TestBackfillWorker_processBackfill_UpdatesLastBackfilledAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	statusSvc := &fakeStatusService{statuses: map[string]*domain.Status{}}
	remoteWrites := &fakeRemoteStatusWriteService{}

	w := newTestBackfillWorker(fs, remoteWrites, statusSvc)

	// processBackfill will fail to fetch the outbox (no resolver), but should still update last_backfilled_at.
	w.processBackfill(ctx, account.ID)

	updated, err := fs.GetAccountByID(ctx, account.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.LastBackfilledAt, "last_backfilled_at should be set after backfill")
}

func TestBackfillWorker_processBackfill_SkipsIfRecentlyBackfilled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	// Simulate a prior job that finished recently by setting last_backfilled_at.
	recent := time.Now().Add(-5 * time.Minute)
	err := fs.UpdateAccountLastBackfilledAt(ctx, account.ID, recent)
	require.NoError(t, err)

	remoteWrites := &fakeRemoteStatusWriteService{}
	w := newTestBackfillWorker(fs, remoteWrites, &fakeStatusService{statuses: map[string]*domain.Status{}})

	w.processBackfill(ctx, account.ID)

	// The second job must not have updated last_backfilled_at again (it was skipped).
	updated, err := fs.GetAccountByID(ctx, account.ID)
	require.NoError(t, err)
	assert.WithinDuration(t, recent, *updated.LastBackfilledAt, time.Second,
		"last_backfilled_at should not be updated when cooldown has not elapsed")
}

func createTestRemoteAccount(t *testing.T, ctx context.Context, s *testutil.FakeStore) *domain.Account {
	t.Helper()
	id := uid.New()
	d := "remote.example"
	acc, err := s.CreateAccount(ctx, store.CreateAccountInput{
		ID:           id,
		Username:     "remote-" + id[:8],
		Domain:       &d,
		PublicKey:    "pk",
		OutboxURL:    "https://" + d + "/users/remote/outbox",
		InboxURL:     "https://" + d + "/users/remote/inbox",
		FollowersURL: "https://" + d + "/users/remote/followers",
		APID:         "https://" + d + "/users/remote-" + id[:8],
	})
	require.NoError(t, err)
	return acc
}

func TestBackfillWorker_fetchAndProcessFeatured_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	// knownIRI is in the local DB; unknownIRI is not.
	knownIRI := "https://remote.example/notes/pinned1"
	unknownIRI := "https://remote.example/notes/unknown"
	localStatusID := uid.New()
	statusSvc := &fakeStatusService{
		statuses: map[string]*domain.Status{
			knownIRI: {ID: localStatusID, APID: knownIRI},
		},
	}

	items := makeFeaturedItems(t, knownIRI, unknownIRI)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json")
		coll := vocab.NewOrderedCollectionWithItems("https://remote.example/featured", items)
		_ = json.NewEncoder(w).Encode(coll)
	}))
	defer ts.Close()

	worker := newTestBackfillWorker(fs, &fakeRemoteStatusWriteService{}, statusSvc)
	worker.remoteResolver = &RemoteAccountResolver{httpClient: ts.Client()}

	ids, ok := worker.fetchAndProcessFeatured(ctx, &domain.Account{
		ID:          account.ID,
		FeaturedURL: ts.URL,
	})

	require.True(t, ok, "successful fetch should allow pin update")
	require.Len(t, ids, 1, "only the locally known IRI should be returned")
	assert.Equal(t, localStatusID, ids[0])
}

func TestBackfillWorker_fetchAndProcessFeatured_FetchError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	worker := newTestBackfillWorker(fs, &fakeRemoteStatusWriteService{}, &fakeStatusService{statuses: map[string]*domain.Status{}})
	worker.remoteResolver = &RemoteAccountResolver{httpClient: ts.Client()}

	ids, ok := worker.fetchAndProcessFeatured(ctx, &domain.Account{
		ID:          account.ID,
		FeaturedURL: ts.URL,
	})

	assert.False(t, ok, "fetch error should prevent pin update to avoid wiping existing pins")
	assert.Nil(t, ids)
}

func TestExtractIRI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		raw     string
		wantIRI string
	}{
		{"string IRI", `"https://remote.example/notes/1"`, "https://remote.example/notes/1"},
		{"Note object", `{"id":"https://remote.example/notes/2","type":"Note"}`, "https://remote.example/notes/2"},
		{"empty object", `{}`, ""},
		{"invalid", `not-json`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractIRI(json.RawMessage(tc.raw))
			assert.Equal(t, tc.wantIRI, got)
		})
	}
}

// makeFeaturedItems returns a slice of RawMessage items for a featured collection.
func makeFeaturedItems(t *testing.T, iris ...string) []json.RawMessage {
	t.Helper()
	items := make([]json.RawMessage, 0, len(iris))
	for _, iri := range iris {
		b, err := json.Marshal(iri)
		require.NoError(t, err)
		items = append(items, b)
	}
	return items
}

func TestBackfillWorker_fetchAndProcessFollowing_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	// Actor IRIs must point to the test server so the resolver can fetch them.
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json")
		switch r.URL.Path {
		case testFollowingPath:
			items := makeFeaturedItems(t, ts.URL+"/users/target1", ts.URL+"/users/target2")
			coll := vocab.NewOrderedCollectionWithItems(ts.URL+"/following", items)
			_ = json.NewEncoder(w).Encode(coll)
		default:
			// Serve a minimal Actor document for any actor IRI.
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                ts.URL + r.URL.Path,
				"preferredUsername": r.URL.Path[7:], // strip "/users/"
				"inbox":             ts.URL + r.URL.Path + "/inbox",
				"outbox":            ts.URL + r.URL.Path + "/outbox",
				"followers":         ts.URL + r.URL.Path + "/followers",
				"following":         ts.URL + r.URL.Path + "/following",
				"publicKey": map[string]any{
					"id":           ts.URL + r.URL.Path + "#main-key",
					"owner":        ts.URL + r.URL.Path,
					"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAMY\n-----END PUBLIC KEY-----",
				},
			}
			_ = json.NewEncoder(w).Encode(actor)
		}
	}))
	defer ts.Close()

	worker := newTestBackfillWorker(fs, &fakeRemoteStatusWriteService{}, &fakeStatusService{statuses: map[string]*domain.Status{}})
	worker.remoteResolver = &RemoteAccountResolver{httpClient: ts.Client(), accounts: service.NewAccountService(fs, "https://local.example"), instanceDomain: "local.example"}

	worker.fetchAndProcessFollowing(ctx, &domain.Account{
		ID:           account.ID,
		APID:         account.APID,
		FollowingURL: ts.URL + "/following",
	})

	follows, err := fs.GetFollowing(ctx, account.ID, nil, 10)
	require.NoError(t, err)
	assert.Len(t, follows, 2, "both followed accounts should be stored locally")
}

func TestBackfillWorker_fetchAndProcessFollowing_SkipsDuplicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	// Pre-seed a second remote account and a follow relationship.
	// The APID must be set after the test server is created so it points to ts.URL.
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json")
		switch r.URL.Path {
		case testFollowingPath:
			items := makeFeaturedItems(t, ts.URL+"/users/existing")
			coll := vocab.NewOrderedCollectionWithItems(ts.URL+"/following", items)
			_ = json.NewEncoder(w).Encode(coll)
		default:
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                ts.URL + r.URL.Path,
				"preferredUsername": "existing-target",
				"inbox":             ts.URL + r.URL.Path + "/inbox",
				"outbox":            ts.URL + r.URL.Path + "/outbox",
				"followers":         ts.URL + r.URL.Path + "/followers",
				"following":         ts.URL + r.URL.Path + "/following",
				"publicKey": map[string]any{
					"id":           ts.URL + r.URL.Path + "#main-key",
					"owner":        ts.URL + r.URL.Path,
					"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAMY\n-----END PUBLIC KEY-----",
				},
			}
			_ = json.NewEncoder(w).Encode(actor)
		}
	}))
	defer ts.Close()

	// Pre-seed the existing account with an APID matching the test server URL, then
	// create the follow so the worker hits ErrConflict on the second attempt.
	d := "127.0.0.1"
	existing, err := fs.CreateAccount(ctx, store.CreateAccountInput{
		ID:       uid.New(),
		Username: "existing-target",
		Domain:   &d,
		APID:     ts.URL + "/users/existing",
		InboxURL: ts.URL + "/users/existing/inbox",
	})
	require.NoError(t, err)
	_, err = service.NewRemoteFollowService(fs).CreateRemoteFollow(ctx, account.ID, existing.ID, domain.FollowStateAccepted, nil)
	require.NoError(t, err)

	worker := newTestBackfillWorker(fs, &fakeRemoteStatusWriteService{}, &fakeStatusService{statuses: map[string]*domain.Status{}})
	worker.remoteResolver = &RemoteAccountResolver{httpClient: ts.Client(), accounts: service.NewAccountService(fs, "https://local.example"), instanceDomain: "local.example"}

	// Should not panic or return error — ErrConflict is silently discarded.
	worker.fetchAndProcessFollowing(ctx, &domain.Account{
		ID:           account.ID,
		APID:         account.APID,
		FollowingURL: ts.URL + "/following",
	})

	follows, err := fs.GetFollowing(ctx, account.ID, nil, 10)
	require.NoError(t, err)
	assert.Len(t, follows, 1, "duplicate follow should not be stored twice")
}

func TestBackfillWorker_fetchAndProcessFollowing_EmptyURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	worker := newTestBackfillWorker(fs, &fakeRemoteStatusWriteService{}, &fakeStatusService{statuses: map[string]*domain.Status{}})
	worker.remoteResolver = &RemoteAccountResolver{httpClient: ts.Client(), accounts: service.NewAccountService(fs, "https://local.example"), instanceDomain: "local.example"}

	worker.fetchAndProcessFollowing(ctx, &domain.Account{
		ID:           account.ID,
		APID:         account.APID,
		FollowingURL: "", // empty — should be a no-op
	})

	assert.False(t, called, "no HTTP request should be made when FollowingURL is empty")
}

func TestBackfillWorker_fetchAndProcessFollowing_FetchError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	worker := newTestBackfillWorker(fs, &fakeRemoteStatusWriteService{}, &fakeStatusService{statuses: map[string]*domain.Status{}})
	worker.remoteResolver = &RemoteAccountResolver{httpClient: ts.Client(), accounts: service.NewAccountService(fs, "https://local.example"), instanceDomain: "local.example"}

	// Should not panic or propagate the error.
	worker.fetchAndProcessFollowing(ctx, &domain.Account{
		ID:           account.ID,
		APID:         account.APID,
		FollowingURL: ts.URL + "/following",
	})

	follows, err := fs.GetFollowing(ctx, account.ID, nil, 10)
	require.NoError(t, err)
	assert.Empty(t, follows, "no follows should be stored after a fetch error")
}

func TestBackfillWorker_fetchAndProcessFollowing_RespectsMaxItems(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	const collectionSize = 5
	const cap = 2

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json")
		switch r.URL.Path {
		case testFollowingPath:
			iris := make([]string, 0, collectionSize)
			for i := range collectionSize {
				iris = append(iris, fmt.Sprintf("%s/users/target%d", ts.URL, i))
			}
			items := makeFeaturedItems(t, iris...)
			coll := vocab.NewOrderedCollectionWithItems(ts.URL+"/following", items)
			_ = json.NewEncoder(w).Encode(coll)
		default:
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                ts.URL + r.URL.Path,
				"preferredUsername": r.URL.Path[7:],
				"inbox":             ts.URL + r.URL.Path + "/inbox",
				"outbox":            ts.URL + r.URL.Path + "/outbox",
				"followers":         ts.URL + r.URL.Path + "/followers",
				"following":         ts.URL + r.URL.Path + "/following",
				"publicKey": map[string]any{
					"id":           ts.URL + r.URL.Path + "#main-key",
					"owner":        ts.URL + r.URL.Path,
					"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAMY\n-----END PUBLIC KEY-----",
				},
			}
			_ = json.NewEncoder(w).Encode(actor)
		}
	}))
	defer ts.Close()

	worker := newTestBackfillWorker(fs, &fakeRemoteStatusWriteService{}, &fakeStatusService{statuses: map[string]*domain.Status{}})
	worker.maxItems = cap
	worker.remoteResolver = &RemoteAccountResolver{httpClient: ts.Client(), accounts: service.NewAccountService(fs, "https://local.example"), instanceDomain: "local.example"}

	worker.fetchAndProcessFollowing(ctx, &domain.Account{
		ID:           account.ID,
		APID:         account.APID,
		FollowingURL: ts.URL + "/following",
	})

	follows, err := fs.GetFollowing(ctx, account.ID, nil, 100)
	require.NoError(t, err)
	assert.Len(t, follows, cap, "only maxItems follows should be stored; remaining items must be skipped")
}

func TestBackfillWorker_fetchAndProcessFollowing_SkipsSelf(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fs := testutil.NewFakeStore()
	account := createTestRemoteAccount(t, ctx, fs)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json")
		// Collection contains only the account's own APID.
		items := makeFeaturedItems(t, account.APID)
		coll := vocab.NewOrderedCollectionWithItems("https://remote.example/following", items)
		_ = json.NewEncoder(w).Encode(coll)
	}))
	defer ts.Close()

	worker := newTestBackfillWorker(fs, &fakeRemoteStatusWriteService{}, &fakeStatusService{statuses: map[string]*domain.Status{}})
	worker.remoteResolver = &RemoteAccountResolver{httpClient: ts.Client(), accounts: service.NewAccountService(fs, "https://local.example"), instanceDomain: "local.example"}

	worker.fetchAndProcessFollowing(ctx, &domain.Account{
		ID:           account.ID,
		APID:         account.APID,
		FollowingURL: ts.URL + "/following",
	})

	follows, err := fs.GetFollowing(ctx, account.ID, nil, 10)
	require.NoError(t, err)
	assert.Empty(t, follows, "self-follow should be skipped")
}
