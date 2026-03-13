package internal

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/activitypub/blocklist"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func TestOutboxDeliveryWorker_getActivityType(t *testing.T) {
	t.Parallel()
	var w outboxDeliveryWorker
	assert.Equal(t, "create", w.getActivityType(subjectPrefixDeliver+"create"))
	assert.Equal(t, "delete", w.getActivityType(subjectPrefixDeliver+"delete"))
	assert.Equal(t, activityTypeUnknown, w.getActivityType("other.subject"))
	assert.Equal(t, activityTypeUnknown, w.getActivityType(""))
}

func TestOutboxDeliveryWorker_processMessage_invalidJSON_acksMessage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	bl := blocklist.NewBlocklistCache(fake)
	_ = bl.Refresh(ctx)
	acked := false
	msg := &mockDeliveryMsg{
		data:    []byte("not valid json"),
		subject: subjectPrefixDeliver + "create",
		ack:     func() error { acked = true; return nil },
	}
	cfg := &config.Config{}
	w := NewOutboxDeliveryWorker(nil, bl, nil, cfg)
	// nil js and signer would panic if we reach deliverHTTP; invalid JSON path only unmarshals and acks
	w.processMessage(ctx, msg)
	assert.True(t, acked, "message should be acked when payload is invalid")
}

func TestOutboxDeliveryWorker_processMessage_validJSON_suspendedDomain_acksWithoutDelivery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := testutil.NewFakeStore()
	_, err := fake.CreateDomainBlock(ctx, store.CreateDomainBlockInput{
		ID: uid.New(), Domain: "evil.com", Severity: domain.DomainBlockSeveritySuspend, Reason: nil,
	})
	require.NoError(t, err)
	bl := blocklist.NewBlocklistCache(fake)
	err = bl.Refresh(ctx)
	require.NoError(t, err)

	payload, _ := json.Marshal(OutboxDeliveryMessage{
		ActivityID:  "https://example.com/activities/1",
		Activity:    json.RawMessage(`{"type":"Create"}`),
		TargetInbox: "https://evil.com/inbox",
		SenderID:    "01sender",
	})
	acked := false
	msg := &mockDeliveryMsg{
		data:    payload,
		subject: subjectPrefixDeliver + "create",
		ack:     func() error { acked = true; return nil },
	}
	cfg := &config.Config{}
	// Nil js and signer - processMessage checks blocklist first and acks without calling deliverHTTP
	w := NewOutboxDeliveryWorker(nil, bl, nil, cfg)
	w.processMessage(ctx, msg)
	assert.True(t, acked)
}

type mockDeliveryMsg struct {
	data    []byte
	subject string
	ack     func() error
}

func (m *mockDeliveryMsg) Data() []byte         { return m.data }
func (m *mockDeliveryMsg) Subject() string      { return m.subject }
func (m *mockDeliveryMsg) Reply() string        { return "" }
func (m *mockDeliveryMsg) Headers() nats.Header { return nil }
func (m *mockDeliveryMsg) Ack() error {
	if m.ack != nil {
		return m.ack()
	}
	return nil
}
func (m *mockDeliveryMsg) Nak() error                                { return nil }
func (m *mockDeliveryMsg) Term() error                               { return nil }
func (m *mockDeliveryMsg) InProgress() error                         { return nil }
func (m *mockDeliveryMsg) DoubleAck(context.Context) error           { return nil }
func (m *mockDeliveryMsg) NakWithDelay(time.Duration) error          { return nil }
func (m *mockDeliveryMsg) TermWithReason(string) error               { return nil }
func (m *mockDeliveryMsg) Metadata() (*jetstream.MsgMetadata, error) { return nil, nil }
