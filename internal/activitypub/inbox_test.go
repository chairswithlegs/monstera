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
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/media"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
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
