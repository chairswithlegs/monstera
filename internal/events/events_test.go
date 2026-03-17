package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestEmitEvent_marshals_payload_and_inserts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := testutil.NewFakeStore()

	type testPayload struct {
		StatusID string `json:"status_id"`
		Local    bool   `json:"local"`
	}
	payload := testPayload{StatusID: "01status", Local: true}

	err := EmitEvent(ctx, fs, "status.created", "status", "01status", payload)
	require.NoError(t, err)

	require.Len(t, fs.OutboxEvents, 1)
	ev := fs.OutboxEvents[0]
	assert.NotEmpty(t, ev.ID)
	assert.Equal(t, "status.created", ev.EventType)
	assert.Equal(t, "status", ev.AggregateType)
	assert.Equal(t, "01status", ev.AggregateID)

	var decoded testPayload
	err = json.Unmarshal(ev.Payload, &decoded)
	require.NoError(t, err)
	assert.Equal(t, payload, decoded)
}

func TestEmitEvent_returns_marshal_error(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := testutil.NewFakeStore()

	err := EmitEvent(ctx, fs, "test.event", "test", "01", make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal")
	assert.Empty(t, fs.OutboxEvents)
}

func TestEmitEvent_returns_store_error(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	storeErr := errors.New("db write failed")
	fs := &failingOutboxStore{FakeStore: testutil.NewFakeStore(), insertErr: storeErr}

	err := EmitEvent(ctx, fs, "status.created", "status", "01status", map[string]string{"key": "value"})
	require.Error(t, err)
	assert.ErrorIs(t, err, storeErr)
}

// failingOutboxStore wraps FakeStore but returns a configured error from InsertOutboxEvent.
type failingOutboxStore struct {
	*testutil.FakeStore
	insertErr error
}

func (f *failingOutboxStore) InsertOutboxEvent(_ context.Context, _ store.InsertOutboxEventInput) error {
	return f.insertErr
}
