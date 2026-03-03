package mastodon

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/events"
	"github.com/chairswithlegs/monstera-fed/internal/events/sse"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

func TestNoopEventBus_ImplementsEventBus(t *testing.T) {
	t.Parallel()
	var _ events.EventBus = events.NoopEventBus //nolint:staticcheck // interface check
}

func TestPublisher_PublishStatusCreated_NilData_NoPanic(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	mockNC := newMockNatsPublisher()
	fake := testutil.NewFakeStore()
	pub := NewPublisher(mockNC, fake, metrics, nil, "example.com")
	ctx := context.Background()

	assert.NotPanics(t, func() {
		pub.PublishStatusCreated(ctx, events.StatusCreatedEvent{})
	})
	assert.NotPanics(t, func() {
		pub.PublishStatusCreated(ctx, events.StatusCreatedEvent{Status: &domain.Status{}, Author: nil})
	})
	assert.NotPanics(t, func() {
		pub.PublishStatusDeleted(ctx, events.StatusDeletedEvent{})
	})
	assert.NotPanics(t, func() {
		pub.PublishNotificationCreated(ctx, events.NotificationCreatedEvent{})
	})
}

func TestPublisher_PublishStatusCreatedRaw_PublishesToNATS(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mockNC := newMockNatsPublisher()
	fake := testutil.NewFakeStore()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	pub := NewPublisher(mockNC, fake, metrics, nil, "example.com")

	statusJSON := json.RawMessage(`{"id":"01status","content":"hello"`)
	opts := activitypub.StatusEventOpts{
		AccountID:    "01account",
		Visibility:   domain.VisibilityPublic,
		Local:        true,
		HashtagNames: []string{"test"},
	}
	pub.PublishStatusCreatedRaw(ctx, statusJSON, opts)

	assert.True(t, mockNC.publishedTo(sse.SubjectPrefixPublic))
	assert.True(t, mockNC.publishedTo(sse.SubjectPrefixPublicLocal))
	assert.True(t, mockNC.publishedTo(sse.SubjectPrefixHashtag+"test"))
}

func TestPublisher_PublishNotificationCreatedRaw_PublishesToUserSubject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mockNC := newMockNatsPublisher()
	fake := testutil.NewFakeStore()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	pub := NewPublisher(mockNC, fake, metrics, nil, "example.com")

	notifJSON := json.RawMessage(`{"id":"01notif","type":"mention"`)
	pub.PublishNotificationCreatedRaw(ctx, "01recipient", notifJSON)

	require.True(t, mockNC.publishedTo(sse.StreamKeyToSubject(sse.StreamUserPrefix+"01recipient")))
}

type mockNatsPublisher struct {
	mu       sync.Mutex
	subjects []string
}

func newMockNatsPublisher() *mockNatsPublisher {
	return &mockNatsPublisher{subjects: nil}
}

func (m *mockNatsPublisher) Publish(subject string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subjects = append(m.subjects, subject)
	return nil
}

func (m *mockNatsPublisher) publishedTo(subject string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.subjects {
		if s == subject {
			return true
		}
	}
	return false
}
