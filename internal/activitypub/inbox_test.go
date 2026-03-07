package activitypub

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/media"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func TestInboxProcessor_Process_unsupportedType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake, &config.Config{InstanceDomain: "example.com"})
	activity := &Activity{Type: "Unknown", ID: "https://remote.example/activities/1", Actor: "https://remote.example/users/alice"}
	err := proc.Process(ctx, activity)
	assert.NoError(t, err)
}

func TestInboxProcessor_Process_emptyActorDomain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	proc := newInboxProcessorForTest(t, fake, &config.Config{InstanceDomain: "example.com"})
	activity := &Activity{Type: "Follow", Actor: "not-a-url"}
	err := proc.Process(ctx, activity)
	assert.ErrorIs(t, err, ErrFatal)
}

// testMediaStore is a minimal MediaStore for inbox tests (CreateRemote is not used in these tests).
type testMediaStore struct{}

func (testMediaStore) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	return nil
}
func (testMediaStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, media.ErrNotFound
}
func (testMediaStore) Delete(ctx context.Context, key string) error { return nil }
func (testMediaStore) URL(ctx context.Context, key string) (string, error) {
	return "https://example.com/" + key, nil
}

// noopInboxEvents is a test double that implements both InboxEventPublisher and events.EventBus with no-op methods.
type noopInboxEvents struct{}

func (noopInboxEvents) PublishStatusCreatedRaw(context.Context, json.RawMessage, StatusEventOpts)   {}
func (noopInboxEvents) PublishStatusDeletedRaw(context.Context, string, StatusEventOpts)            {}
func (noopInboxEvents) PublishNotificationCreatedRaw(context.Context, string, json.RawMessage)      {}
func (noopInboxEvents) PublishStatusCreated(context.Context, events.StatusCreatedEvent)             {}
func (noopInboxEvents) PublishStatusDeleted(context.Context, events.StatusDeletedEvent)             {}
func (noopInboxEvents) PublishNotificationCreated(context.Context, events.NotificationCreatedEvent) {}

func newInboxProcessorForTest(t *testing.T, fake *testutil.FakeStore, cfg *config.Config) Inbox {
	t.Helper()
	cacheStore, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	bl := NewBlocklistCache(fake)
	_ = bl.Refresh(context.Background())
	instanceBaseURL := "https://example.com"
	if cfg != nil && cfg.InstanceDomain != "" {
		instanceBaseURL = "https://" + cfg.InstanceDomain
	}
	accountSvc := service.NewAccountService(fake, instanceBaseURL)
	followSvc := service.NewFollowService(fake, nil, nil)
	notificationSvc := service.NewNotificationService(fake)
	statusSvc := service.NewStatusService(fake, service.NoopFederationPublisher, events.NoopEventBus, instanceBaseURL, "example.com", 5000, nil)
	mediaSvc := service.NewMediaService(fake, &testMediaStore{}, 1<<20)
	noopEvents := &noopInboxEvents{}
	return NewInbox(accountSvc, followSvc, notificationSvc, statusSvc, mediaSvc, nil, cacheStore, bl, nil, noopEvents, noopEvents, cfg)
}

const (
	testAliceAPID = "https://example.com/users/alice"
	testBobAPID   = "https://bob.com/users/bob"
)

func TestInbox_Process_AcceptFollow_forgedActorRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	proc := newInboxProcessorForTest(t, fake, cfg)

	aliceID := uid.New()
	bobID := uid.New()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: aliceID, Username: "alice", Domain: nil,
		InboxURL: "https://example.com/users/alice/inbox", OutboxURL: "https://example.com/users/alice/outbox",
		FollowersURL: "https://example.com/users/alice/followers", FollowingURL: "https://example.com/users/alice/following",
		APID: testAliceAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: bobID, Username: "bob", Domain: testutil.StrPtr("bob.com"),
		InboxURL: "https://bob.com/users/bob/inbox", OutboxURL: "https://bob.com/users/bob/outbox",
		FollowersURL: "https://bob.com/users/bob/followers", FollowingURL: "https://bob.com/users/bob/following",
		APID: testBobAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: uid.New(), AccountID: bobID, TargetID: aliceID, State: domain.FollowStatePending, APID: nil,
	})
	require.NoError(t, err)

	innerFollow := map[string]string{"type": "Follow", "actor": testBobAPID, "object": testAliceAPID}
	objectRaw, err := json.Marshal(innerFollow)
	require.NoError(t, err)
	activity := &Activity{
		Type:      "Accept",
		ID:        "https://evil.com/accept/1",
		Actor:     "https://evil.com/users/attacker",
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFatal)
}

func TestInbox_Process_RejectFollow_forgedActorRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	proc := newInboxProcessorForTest(t, fake, cfg)

	aliceID := uid.New()
	bobID := uid.New()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: aliceID, Username: "alice", Domain: nil,
		InboxURL: "https://example.com/users/alice/inbox", OutboxURL: "https://example.com/users/alice/outbox",
		FollowersURL: "https://example.com/users/alice/followers", FollowingURL: "https://example.com/users/alice/following",
		APID: testAliceAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: bobID, Username: "bob", Domain: testutil.StrPtr("bob.com"),
		InboxURL: "https://bob.com/users/bob/inbox", OutboxURL: "https://bob.com/users/bob/outbox",
		FollowersURL: "https://bob.com/users/bob/followers", FollowingURL: "https://bob.com/users/bob/following",
		APID: testBobAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID: uid.New(), AccountID: bobID, TargetID: aliceID, State: domain.FollowStatePending, APID: nil,
	})
	require.NoError(t, err)

	innerFollow := map[string]string{"type": "Follow", "actor": testBobAPID, "object": testAliceAPID}
	objectRaw, err := json.Marshal(innerFollow)
	require.NoError(t, err)
	activity := &Activity{
		Type:      "Reject",
		ID:        "https://evil.com/reject/1",
		Actor:     "https://evil.com/users/attacker",
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFatal)
}

func TestInbox_Process_UpdatePerson_forgedActorRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	proc := newInboxProcessorForTest(t, fake, cfg)

	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: uid.New(), Username: "bob", Domain: testutil.StrPtr("bob.com"),
		InboxURL: "https://bob.com/users/bob/inbox", OutboxURL: "https://bob.com/users/bob/outbox",
		FollowersURL: "https://bob.com/users/bob/followers", FollowingURL: "https://bob.com/users/bob/following",
		APID: testBobAPID,
	})
	require.NoError(t, err)

	objectActor := map[string]string{"id": testBobAPID, "type": "Person", "preferredUsername": "bob"}
	objectRaw, err := json.Marshal(objectActor)
	require.NoError(t, err)
	activity := &Activity{
		Type:      "Update",
		ID:        "https://evil.com/update/1",
		Actor:     "https://evil.com/users/attacker",
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFatal)
}

func TestInbox_Process_Delete_statusWrongActorRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	proc := newInboxProcessorForTest(t, fake, cfg)

	aliceID := uid.New()
	bobID := uid.New()
	_, err := fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: aliceID, Username: "alice", Domain: nil,
		InboxURL: "https://example.com/users/alice/inbox", OutboxURL: "https://example.com/users/alice/outbox",
		FollowersURL: "https://example.com/users/alice/followers", FollowingURL: "https://example.com/users/alice/following",
		APID: testAliceAPID,
	})
	require.NoError(t, err)
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID: bobID, Username: "bob", Domain: testutil.StrPtr("bob.com"),
		InboxURL: "https://bob.com/users/bob/inbox", OutboxURL: "https://bob.com/users/bob/outbox",
		FollowersURL: "https://bob.com/users/bob/followers", FollowingURL: "https://bob.com/users/bob/following",
		APID: testBobAPID,
	})
	require.NoError(t, err)
	statusAPID := "https://example.com/statuses/1"
	st, err := fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: uid.New(), URI: statusAPID, AccountID: aliceID, APID: statusAPID,
		Visibility: domain.VisibilityPublic, Local: true,
	})
	require.NoError(t, err)
	require.NotNil(t, st)

	objectRaw, err := json.Marshal(map[string]string{"id": statusAPID, "type": "Note"})
	require.NoError(t, err)
	activity := &Activity{
		Type:      "Delete",
		Actor:     testBobAPID,
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFatal)
}

func TestInbox_Process_Delete_authorLookupFailsReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	cfg := &config.Config{InstanceDomain: "example.com"}
	proc := newInboxProcessorForTest(t, fake, cfg)

	statusAPID := "https://example.com/statuses/orphan"
	orphanAccountID := "01orphan"
	st, err := fake.CreateStatus(ctx, store.CreateStatusInput{
		ID: uid.New(), URI: statusAPID, AccountID: orphanAccountID, APID: statusAPID,
		Visibility: domain.VisibilityPublic, Local: false,
	})
	require.NoError(t, err)
	require.NotNil(t, st)

	objectRaw, err := json.Marshal(map[string]string{"id": statusAPID, "type": "Note"})
	require.NoError(t, err)
	activity := &Activity{
		Type:      "Delete",
		Actor:     "https://example.com/users/someone",
		ObjectRaw: objectRaw,
	}
	err = proc.Process(ctx, activity)
	require.Error(t, err)
}
