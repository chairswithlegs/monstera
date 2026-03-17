package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

// mockJetStream captures Publish calls for test assertions.
type mockJetStream struct {
	jetstream.JetStream
	published  []publishedMsg
	publishErr error
}

type publishedMsg struct {
	subject string
	data    []byte
	numOpts int
}

func (m *mockJetStream) Publish(_ context.Context, subject string, data []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	if m.publishErr != nil {
		return nil, m.publishErr
	}
	m.published = append(m.published, publishedMsg{subject: subject, data: data, numOpts: len(opts)})
	return &jetstream.PubAck{}, nil
}

func TestPoll_publishes_events_and_marks_published(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := testutil.NewFakeStore()
	js := &mockJetStream{}

	fs.OutboxEvents = []domain.DomainEvent{
		{ID: "ev1", EventType: domain.EventStatusCreated, AggregateType: "status", AggregateID: "s1", Payload: json.RawMessage(`{"status_id":"s1"}`)},
		{ID: "ev2", EventType: domain.EventFollowCreated, AggregateType: "follow", AggregateID: "f1", Payload: json.RawMessage(`{"follow_id":"f1"}`)},
	}

	p := NewPoller(fs, js, PollerConfig{BatchSize: 10})
	err := p.poll(ctx)
	require.NoError(t, err)

	require.Len(t, js.published, 2)
	assert.Equal(t, SubjectPrefix+domain.EventStatusCreated, js.published[0].subject)
	assert.Equal(t, SubjectPrefix+domain.EventFollowCreated, js.published[1].subject)

	assert.Empty(t, fs.OutboxEvents)
}

func TestPoll_no_events_does_nothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := testutil.NewFakeStore()
	js := &mockJetStream{}

	p := NewPoller(fs, js, PollerConfig{BatchSize: 10})
	err := p.poll(ctx)
	require.NoError(t, err)

	assert.Empty(t, js.published)
}

func TestPoll_returns_error_when_publish_fails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := testutil.NewFakeStore()

	publishErr := errors.New("nats connection lost")
	js := &mockJetStream{publishErr: publishErr}

	fs.OutboxEvents = []domain.DomainEvent{
		{ID: "ev1", EventType: domain.EventStatusCreated, AggregateType: "status", AggregateID: "s1", Payload: json.RawMessage(`{}`)},
	}

	p := NewPoller(fs, js, PollerConfig{BatchSize: 10})
	err := p.poll(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, publishErr)

	assert.Len(t, fs.OutboxEvents, 1, "events should not be marked published on failure")
}

func TestPublish_subject_format_and_dedup_message_ID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	js := &mockJetStream{}

	p := NewPoller(nil, js, PollerConfig{})
	event := domain.DomainEvent{
		ID:            "ev-dedup-123",
		EventType:     domain.EventStatusCreated,
		AggregateType: "status",
		AggregateID:   "s1",
		Payload:       json.RawMessage(`{"status_id":"s1"}`),
	}

	err := p.publish(ctx, event)
	require.NoError(t, err)

	require.Len(t, js.published, 1)
	msg := js.published[0]
	assert.Equal(t, SubjectPrefix+domain.EventStatusCreated, msg.subject)
	assert.Equal(t, 1, msg.numOpts, "should pass exactly one PublishOpt (WithMsgID)")

	var decoded domain.DomainEvent
	err = json.Unmarshal(msg.data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, event.ID, decoded.ID)
	assert.Equal(t, event.EventType, decoded.EventType)
	assert.Equal(t, event.AggregateType, decoded.AggregateType)
	assert.Equal(t, event.AggregateID, decoded.AggregateID)
	assert.JSONEq(t, string(event.Payload), string(decoded.Payload))
}
