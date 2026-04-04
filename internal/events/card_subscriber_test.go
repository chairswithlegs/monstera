package events

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
)

type fakeCardProcessor struct {
	calls []string
	err   error
}

func (f *fakeCardProcessor) FetchAndStoreCard(_ context.Context, statusID string) error {
	f.calls = append(f.calls, statusID)
	return f.err
}

func newTestCardSub(svc CardProcessor) *CardSubscriber {
	return &CardSubscriber{svc: svc}
}

func TestCardSubscriber_LocalStatus_TriggersProcessing(t *testing.T) {
	t.Parallel()
	svc := &fakeCardProcessor{}
	sub := newTestCardSub(svc)

	event := makeEvent(t, domain.EventStatusCreated, domain.StatusCreatedPayload{
		Status: &domain.Status{ID: "status-1"},
		Local:  true,
	})
	sub.handleStatusCreated(context.Background(), event)

	require.Len(t, svc.calls, 1)
	assert.Equal(t, "status-1", svc.calls[0])
}

func TestCardSubscriber_RemoteStatus_TriggersProcessing(t *testing.T) {
	t.Parallel()
	svc := &fakeCardProcessor{}
	sub := newTestCardSub(svc)

	event := makeEvent(t, domain.EventStatusCreated, domain.StatusCreatedPayload{
		Status: &domain.Status{ID: "status-1"},
		Local:  false,
	})
	sub.handleStatusCreated(context.Background(), event)

	require.Len(t, svc.calls, 1)
	assert.Equal(t, "status-1", svc.calls[0])
}

func TestCardSubscriber_RemoteStatusCreatedEvent_TriggersProcessing(t *testing.T) {
	t.Parallel()
	svc := &fakeCardProcessor{}
	sub := newTestCardSub(svc)

	event := makeEvent(t, domain.EventStatusCreatedRemote, domain.StatusCreatedPayload{
		Status: &domain.Status{ID: "status-2"},
		Local:  false,
	})
	sub.handleStatusCreated(context.Background(), event)

	require.Len(t, svc.calls, 1)
	assert.Equal(t, "status-2", svc.calls[0])
}

func TestCardSubscriber_NilStatus_Skipped(t *testing.T) {
	t.Parallel()
	svc := &fakeCardProcessor{}
	sub := newTestCardSub(svc)

	event := makeEvent(t, domain.EventStatusCreated, domain.StatusCreatedPayload{
		Status: nil,
		Local:  true,
	})
	sub.handleStatusCreated(context.Background(), event)

	assert.Empty(t, svc.calls)
}

func TestCardSubscriber_InvalidPayload_Skipped(t *testing.T) {
	t.Parallel()
	svc := &fakeCardProcessor{}
	sub := newTestCardSub(svc)

	event := domain.DomainEvent{
		ID:        "evt-1",
		EventType: domain.EventStatusCreated,
		Payload:   []byte("not valid json {{{"),
	}
	sub.handleStatusCreated(context.Background(), event)

	assert.Empty(t, svc.calls)
}

func TestCardSubscriber_ServiceError_Warns(t *testing.T) {
	t.Parallel()
	svc := &fakeCardProcessor{err: errors.New("store unavailable")}
	sub := newTestCardSub(svc)

	event := makeEvent(t, domain.EventStatusCreated, domain.StatusCreatedPayload{
		Status: &domain.Status{ID: "status-1"},
		Local:  true,
	})

	// Should not panic; error is only logged.
	require.NotPanics(t, func() {
		sub.handleStatusCreated(context.Background(), event)
	})
	require.Len(t, svc.calls, 1)
}
