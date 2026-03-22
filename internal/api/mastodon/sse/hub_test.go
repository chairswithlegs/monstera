package sse

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/natsutil"
	"github.com/chairswithlegs/monstera/internal/observability"
)

func TestHub_Subscribe_Cancel_ClosesChannelAndDecrementsGauge(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	mock := newMockNatsConn()
	hub := NewHub(mock, metrics)

	ch, cancel := hub.Subscribe(StreamUserPrefix + "acc1")
	require.NotNil(t, ch)
	require.NotNil(t, cancel)

	gauge, _ := metrics.ActiveSSEConnections.GetMetricWithLabelValues("user")
	assert.InDelta(t, 1.0, testutil.ToFloat64(gauge), 0.01)

	cancel()

	_, open := <-ch
	assert.False(t, open, "channel should be closed after cancel")
	assert.InDelta(t, 0.0, testutil.ToFloat64(gauge), 0.01)
}

func TestHub_Subscribe_OnDemand_DeliversToChannel(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	mock := newMockNatsConn()
	hub := NewHub(mock, metrics)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
	go func() { _ = hub.Start(ctx) }()

	ch, cancel := hub.Subscribe(StreamUserPrefix + "acc1")
	defer cancel()

	ev := SSEEvent{Stream: "user", Event: EventUpdate, Data: `{"id":"1"}`}
	data, err := json.Marshal(ev)
	require.NoError(t, err)

	mock.Deliver(StreamKeyToSubject(StreamUserPrefix+"acc1"), data)

	select {
	case got := <-ch:
		assert.Equal(t, ev.Stream, got.Stream)
		assert.Equal(t, ev.Event, got.Event)
		assert.Equal(t, ev.Data, got.Data)
	case <-time.After(time.Second):
		t.Fatal("expected event on channel")
	}
}

func TestHub_Subscribe_TwoClientsSameStream_BothReceive(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	mock := newMockNatsConn()
	hub := NewHub(mock, metrics)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
	go func() { _ = hub.Start(ctx) }()

	ch1, cancel1 := hub.Subscribe(StreamUserPrefix + "acc1")
	defer cancel1()
	ch2, cancel2 := hub.Subscribe(StreamUserPrefix + "acc1")
	defer cancel2()

	ev := SSEEvent{Stream: "user", Event: EventNotification, Data: `{}`}
	data, err := json.Marshal(ev)
	require.NoError(t, err)
	mock.Deliver(StreamKeyToSubject(StreamUserPrefix+"acc1"), data)

	for _, ch := range []<-chan SSEEvent{ch1, ch2} {
		select {
		case got := <-ch:
			assert.Equal(t, ev.Event, got.Event)
		case <-time.After(time.Second):
			t.Fatal("expected event on channel")
		}
	}
}

func TestHub_Start_UnsubscribesOnContextCancel(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	mock := newMockNatsConn()
	hub := NewHub(mock, metrics)

	ctx, cancelCtx := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = hub.Start(ctx)
		close(done)
	}()

	ch, cancelSub := hub.Subscribe(StreamUserPrefix + "acc1")
	cancelSub()
	cancelCtx()
	<-done

	_, open := <-ch
	assert.False(t, open)
	assert.Equal(t, 0, mock.subscriptionCount())
}

type mockSub struct {
	conn    *mockNatsConn
	subject string
	entry   *mockSubEntry
}

func (m *mockSub) Unsubscribe() error {
	m.conn.removeSub(m.subject, m.entry)
	return nil
}

type mockSubEntry struct {
	handler natsutil.MsgHandler
}

type mockNatsConn struct {
	mu    sync.Mutex
	subs  map[string][]*mockSubEntry
	index map[*mockSubEntry]int
}

func newMockNatsConn() *mockNatsConn {
	return &mockNatsConn{
		subs:  make(map[string][]*mockSubEntry),
		index: make(map[*mockSubEntry]int),
	}
}

func (m *mockNatsConn) Subscribe(subject string, handler natsutil.MsgHandler) (natsutil.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := &mockSubEntry{handler: handler}
	m.subs[subject] = append(m.subs[subject], entry)
	m.index[entry] = len(m.subs[subject]) - 1
	return &mockSub{conn: m, subject: subject, entry: entry}, nil
}

func (m *mockNatsConn) removeSub(subject string, entry *mockSubEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.subs[subject]
	i := m.index[entry]
	last := list[len(list)-1]
	list[i] = last
	m.index[last] = i
	m.subs[subject] = list[:len(list)-1]
	delete(m.index, entry)
	if len(m.subs[subject]) == 0 {
		delete(m.subs, subject)
	}
}

func (m *mockNatsConn) Deliver(subject string, data []byte) {
	m.mu.Lock()
	list := make([]*mockSubEntry, len(m.subs[subject]))
	copy(list, m.subs[subject])
	m.mu.Unlock()
	for _, entry := range list {
		entry.handler(subject, data)
	}
}

func (m *mockNatsConn) subscriptionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, list := range m.subs {
		n += len(list)
	}
	return n
}
