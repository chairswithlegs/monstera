package activitypub

import (
	"context"
	"encoding/json"
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
		statuses:       statusSvc,
		instanceDomain: "local.example",
		maxPages:       2,
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
