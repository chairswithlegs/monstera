package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMsg is a hand-written fake for jetstream.Msg.
type mockMsg struct {
	ackCalled          atomic.Bool
	nakCalled          atomic.Bool
	nakWithDelayCalled atomic.Bool
}

func (m *mockMsg) Metadata() (*jetstream.MsgMetadata, error) { return nil, nil }
func (m *mockMsg) Data() []byte                              { return nil }
func (m *mockMsg) Headers() nats.Header                      { return nil }
func (m *mockMsg) Subject() string                           { return "" }
func (m *mockMsg) Reply() string                             { return "" }
func (m *mockMsg) Ack() error                                { m.ackCalled.Store(true); return nil }
func (m *mockMsg) DoubleAck(_ context.Context) error         { return nil }
func (m *mockMsg) Nak() error                                { m.nakCalled.Store(true); return nil }
func (m *mockMsg) NakWithDelay(_ time.Duration) error        { m.nakWithDelayCalled.Store(true); return nil }
func (m *mockMsg) InProgress() error                         { return nil }
func (m *mockMsg) Term() error                               { return nil }
func (m *mockMsg) TermWithReason(_ string) error             { return nil }

func TestSlotAlignment(t *testing.T) {
	t.Parallel()
	interval := time.Minute

	// Two times within the same minute window should produce the same slot.
	t1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 12, 0, 45, 0, time.UTC)
	assert.Equal(t, t1.Truncate(interval).Unix(), t2.Truncate(interval).Unix(),
		"two times in the same minute window should map to the same slot")

	// Two times in different minute windows should produce different slots.
	t3 := time.Date(2026, 1, 1, 12, 1, 0, 0, time.UTC)
	assert.NotEqual(t, t1.Truncate(interval).Unix(), t3.Truncate(interval).Unix(),
		"times in different minute windows should map to different slots")
}

func TestConsumerConfigForJob(t *testing.T) {
	t.Parallel()
	job := Job{
		Name:     "my-job",
		Interval: 5 * time.Minute,
	}

	cfg := consumerConfigForJob(job)

	assert.Equal(t, "scheduler-my-job", cfg.Durable)
	assert.Equal(t, subjectPrefix+"my-job", cfg.FilterSubject)
	assert.Equal(t, 2*job.Interval, cfg.AckWait)
	assert.Equal(t, jetstream.AckExplicitPolicy, cfg.AckPolicy)
	assert.Equal(t, 1, cfg.MaxAckPending)
}

func TestNatsScheduler_Start_NoJobs(t *testing.T) {
	t.Parallel()
	sched := &natsScheduler{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	err := sched.Start(ctx)
	require.NoError(t, err)
}

func TestProcessMessage_HandlerSuccess(t *testing.T) {
	t.Parallel()
	s := &natsScheduler{}
	msg := &mockMsg{}
	job := Job{
		Name:    "test-job",
		Handler: func(_ context.Context) error { return nil },
	}

	s.processMessage(context.Background(), job, msg)

	assert.True(t, msg.ackCalled.Load(), "Ack should be called on success")
	assert.False(t, msg.nakCalled.Load(), "Nak should not be called on success")
	assert.False(t, msg.nakWithDelayCalled.Load(), "NakWithDelay should not be called on success")
}

func TestProcessMessage_HandlerError(t *testing.T) {
	t.Parallel()
	s := &natsScheduler{}
	msg := &mockMsg{}
	job := Job{
		Name:    "test-job",
		Handler: func(_ context.Context) error { return errors.New("boom") },
	}

	s.processMessage(context.Background(), job, msg)

	assert.False(t, msg.ackCalled.Load(), "Ack should not be called on error")
	assert.True(t, msg.nakWithDelayCalled.Load(), "NakWithDelay should be called on error")
}

func TestProcessMessage_HandlerPanic(t *testing.T) {
	t.Parallel()
	s := &natsScheduler{}
	msg := &mockMsg{}
	job := Job{
		Name:    "test-job",
		Handler: func(_ context.Context) error { panic("something went wrong") },
	}

	// Should not propagate the panic.
	require.NotPanics(t, func() {
		s.processMessage(context.Background(), job, msg)
	})

	assert.True(t, msg.nakCalled.Load(), "Nak should be called after panic")
	assert.False(t, msg.ackCalled.Load(), "Ack should not be called after panic")
}
