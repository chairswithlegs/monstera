package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/api/mastodon/sse"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/natsutil"
	"github.com/chairswithlegs/monstera/internal/observability"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

// deliverNatsConn is a mock NATS subscriber that allows tests to inject events.
type deliverNatsConn struct {
	mu   sync.Mutex
	subs map[string][]natsutil.MsgHandler
}

func newDeliverNatsConn() *deliverNatsConn {
	return &deliverNatsConn{subs: make(map[string][]natsutil.MsgHandler)}
}

func (m *deliverNatsConn) Subscribe(subject string, handler natsutil.MsgHandler) (natsutil.Subscription, error) {
	m.mu.Lock()
	idx := len(m.subs[subject])
	m.subs[subject] = append(m.subs[subject], handler)
	m.mu.Unlock()
	return &deliverNatsSub{conn: m, subject: subject, idx: idx}, nil
}

func (m *deliverNatsConn) Deliver(subject string, ev sse.SSEEvent) {
	data, _ := json.Marshal(ev)
	m.mu.Lock()
	handlers := make([]natsutil.MsgHandler, len(m.subs[subject]))
	copy(handlers, m.subs[subject])
	m.mu.Unlock()
	for _, h := range handlers {
		h(subject, data)
	}
}

func (m *deliverNatsConn) subCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, list := range m.subs {
		n += len(list)
	}
	return n
}

type deliverNatsSub struct {
	conn    *deliverNatsConn
	subject string
	idx     int
}

func (s *deliverNatsSub) Unsubscribe() error {
	s.conn.mu.Lock()
	defer s.conn.mu.Unlock()
	list := s.conn.subs[s.subject]
	if s.idx < len(list) {
		list[s.idx] = list[len(list)-1]
		s.conn.subs[s.subject] = list[:len(list)-1]
	}
	if len(s.conn.subs[s.subject]) == 0 {
		delete(s.conn.subs, s.subject)
	}
	return nil
}

// wsTestServer creates an httptest.Server serving GETStreamingWS.
// If account is non-nil it is injected into the request context via middleware.
func wsTestServer(t *testing.T, hub *sse.Hub, listSvc service.ListService, account *domain.Account) *httptest.Server {
	t.Helper()
	h := NewStreamingHandler(hub, listSvc)
	r := chi.NewRouter()
	r.Get("/streaming", func(w http.ResponseWriter, req *http.Request) {
		if account != nil {
			req = req.WithContext(middleware.WithAccount(req.Context(), account))
		}
		h.GETStreamingWS(w, req)
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// wsURL converts an http test server URL to a WebSocket URL.
func wsURL(srv *httptest.Server, path string) string {
	return "ws" + srv.URL[len("http"):] + path
}

// wsConnect dials the WebSocket endpoint and returns the connection.
func wsConnect(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, url, nil)
	require.NoError(t, err)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

// wsSend sends a JSON client message.
func wsSend(t *testing.T, conn *websocket.Conn, msg wsClientMsg) {
	t.Helper()
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, conn.Write(ctx, websocket.MessageText, data))
}

// wsRecv reads one server message with a timeout.
func wsRecv(t *testing.T, conn *websocket.Conn) wsServerMsg {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	require.NoError(t, err)
	var msg wsServerMsg
	require.NoError(t, json.Unmarshal(data, &msg))
	return msg
}

// newHubNoStart creates a hub without starting it (no always-on subscriptions).
// This is useful for subscription-count tests where event delivery is not needed.
func newHubNoStart(t *testing.T) (*sse.Hub, *deliverNatsConn) {
	t.Helper()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	nc := newDeliverNatsConn()
	hub := sse.NewHub(nc, metrics)
	return hub, nc
}

func TestGETStreamingWS_NoUpgradeHeader_Returns426(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	hub := sse.NewHub(newMockHubConn(), metrics)
	h := NewStreamingHandler(hub, nil)

	req := httptest.NewRequest(http.MethodGet, "/streaming", nil)
	rec := httptest.NewRecorder()
	h.GETStreamingWS(rec, req)

	// coder/websocket returns 426 Upgrade Required when the Upgrade header is missing.
	assert.Equal(t, http.StatusUpgradeRequired, rec.Code)
}

func TestGETStreamingWS_UnauthenticatedSubscribePublic_ReceivesEvents(t *testing.T) {
	t.Parallel()
	hub, nc := newHubNoStart(t)
	srv := wsTestServer(t, hub, nil, nil)

	conn := wsConnect(t, wsURL(srv, "/streaming"))
	wsSend(t, conn, wsClientMsg{Type: "subscribe", Stream: "public"})
	require.Eventually(t, func() bool { return nc.subCount() == 1 }, time.Second, time.Millisecond)

	ev := sse.SSEEvent{Stream: "public", Event: sse.EventUpdate, Data: `{"id":"1"}`}
	nc.Deliver(sse.SubjectPrefixPublic, ev)

	msg := wsRecv(t, conn)
	assert.Equal(t, []string{"public"}, msg.Stream)
	assert.Equal(t, sse.EventUpdate, msg.Event)
	assert.Equal(t, ev.Data, msg.Payload)
}

func TestGETStreamingWS_AuthenticatedSubscribeUser_ReceivesEvents(t *testing.T) {
	t.Parallel()
	hub, nc := newHubNoStart(t)

	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(context.Background(), service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	srv := wsTestServer(t, hub, nil, acc)

	conn := wsConnect(t, wsURL(srv, "/streaming"))
	wsSend(t, conn, wsClientMsg{Type: "subscribe", Stream: "user"})
	require.Eventually(t, func() bool { return nc.subCount() == 1 }, time.Second, time.Millisecond)

	ev := sse.SSEEvent{Stream: "user", Event: sse.EventUpdate, Data: `{"id":"2"}`}
	nc.Deliver(sse.SubjectPrefixUser+acc.ID, ev)

	msg := wsRecv(t, conn)
	assert.Equal(t, []string{"user"}, msg.Stream)
	assert.Equal(t, sse.EventUpdate, msg.Event)
}

func TestGETStreamingWS_UnauthenticatedSubscribeUser_SubscriptionDropped(t *testing.T) {
	t.Parallel()
	// Use hub without Start so subCount == 0 initially (no always-on subscriptions).
	hub, nc := newHubNoStart(t)
	srv := wsTestServer(t, hub, nil, nil) // no account injected

	conn := wsConnect(t, wsURL(srv, "/streaming"))
	wsSend(t, conn, wsClientMsg{Type: "subscribe", Stream: "user"})

	// Send a public subscribe after the unauthenticated user subscribe. Messages are
	// processed in order, so once the public subscription is registered we know the
	// user subscribe was already processed (and silently dropped).
	wsSend(t, conn, wsClientMsg{Type: "subscribe", Stream: "public"})
	require.Eventually(t, func() bool { return nc.subCount() == 1 }, time.Second, time.Millisecond, "public subscription should create a NATS subscription")
	assert.Equal(t, 1, nc.subCount(), "unauthenticated user subscription should not have created a NATS subscription")
}

func TestGETStreamingWS_SubscribeThenUnsubscribe_ReleasesSubscription(t *testing.T) {
	t.Parallel()
	// Use hub without Start so subCount directly reflects on-demand subscriptions.
	hub, nc := newHubNoStart(t)
	srv := wsTestServer(t, hub, nil, nil)

	conn := wsConnect(t, wsURL(srv, "/streaming"))
	wsSend(t, conn, wsClientMsg{Type: "subscribe", Stream: "public"})
	require.Eventually(t, func() bool { return nc.subCount() == 1 }, time.Second, time.Millisecond, "subscribe should create a NATS subscription")

	wsSend(t, conn, wsClientMsg{Type: "unsubscribe", Stream: "public"})
	require.Eventually(t, func() bool { return nc.subCount() == 0 }, time.Second, time.Millisecond, "unsubscribe should release the NATS subscription")
}

func TestGETStreamingWS_MultiStream_BothReceive(t *testing.T) {
	t.Parallel()
	hub, nc := newHubNoStart(t)
	srv := wsTestServer(t, hub, nil, nil)

	conn := wsConnect(t, wsURL(srv, "/streaming"))
	wsSend(t, conn, wsClientMsg{Type: "subscribe", Stream: "public"})
	wsSend(t, conn, wsClientMsg{Type: "subscribe", Stream: "hashtag", Tag: "golang"})
	require.Eventually(t, func() bool { return nc.subCount() == 2 }, time.Second, time.Millisecond)

	nc.Deliver(sse.SubjectPrefixPublic, sse.SSEEvent{Stream: "public", Event: sse.EventUpdate, Data: `{"id":"pub"}`})
	nc.Deliver(sse.SubjectPrefixHashtag+"golang", sse.SSEEvent{Stream: "hashtag:golang", Event: sse.EventUpdate, Data: `{"id":"tag"}`})

	received := make(map[string]bool)
	for range 2 {
		msg := wsRecv(t, conn)
		received[msg.Stream[0]] = true
	}
	assert.True(t, received["public"], "expected public event")
	assert.True(t, received["hashtag"], "expected hashtag event")
}

func TestGETStreamingWS_InitialStreamParam_Subscribes(t *testing.T) {
	t.Parallel()
	hub, nc := newHubNoStart(t)
	srv := wsTestServer(t, hub, nil, nil)

	// Pass ?stream=public in the URL — no explicit subscribe message needed.
	conn := wsConnect(t, wsURL(srv, "/streaming?stream=public"))
	require.Eventually(t, func() bool { return nc.subCount() == 1 }, time.Second, time.Millisecond)

	nc.Deliver(sse.SubjectPrefixPublic, sse.SSEEvent{Stream: "public", Event: sse.EventUpdate, Data: `{"id":"5"}`})
	msg := wsRecv(t, conn)
	assert.Equal(t, []string{"public"}, msg.Stream)
}

func TestGETStreamingWS_MaxSubscriptions_ExtraSubscribeIgnored(t *testing.T) {
	t.Parallel()
	// Use hub without Start so subCount directly reflects on-demand subscriptions.
	hub, nc := newHubNoStart(t)

	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	listSvc := service.NewListService(st)
	acc, err := accountSvc.Register(context.Background(), service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	// Create wsMaxSubscriptions lists.
	listIDs := make([]string, 0, wsMaxSubscriptions)
	for i := range wsMaxSubscriptions {
		l, err := listSvc.CreateList(context.Background(), acc.ID, "List", "", false)
		require.NoError(t, err, i)
		listIDs = append(listIDs, l.ID)
	}

	srv := wsTestServer(t, hub, listSvc, acc)
	conn := wsConnect(t, wsURL(srv, "/streaming"))

	// Subscribe to exactly wsMaxSubscriptions streams.
	for _, id := range listIDs {
		wsSend(t, conn, wsClientMsg{Type: "subscribe", Stream: "list", List: id})
	}
	require.Eventually(t, func() bool { return nc.subCount() == wsMaxSubscriptions }, time.Second, time.Millisecond, "should have exactly wsMaxSubscriptions NATS subscriptions")

	// One more subscribe (public) should be silently ignored.
	wsSend(t, conn, wsClientMsg{Type: "subscribe", Stream: "public"})
	assert.Never(t, func() bool { return nc.subCount() > wsMaxSubscriptions }, 100*time.Millisecond, time.Millisecond, "extra subscribe beyond max should be ignored")
}

func TestGETStreamingWS_SubprotocolToken_IsEchoedBack(t *testing.T) {
	t.Parallel()
	hub, _ := newHubNoStart(t)

	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	acc, err := accountSvc.Register(context.Background(), service.RegisterInput{
		Username: "bob",
		Email:    "bob@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	// Server injects the account (simulating what OptionalAuth does after the
	// middleware extracts the token from Sec-WebSocket-Protocol).
	srv := wsTestServer(t, hub, nil, acc)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	const fakeToken = "fake-access-token"
	conn, resp, err := websocket.Dial(ctx, wsURL(srv, "/streaming"), &websocket.DialOptions{
		Subprotocols: []string{fakeToken},
	})
	require.NoError(t, err)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	assert.Equal(t, fakeToken, conn.Subprotocol(), "server must echo back the subprotocol")
}

func TestGETStreamingWS_ConnectionClose_CleansUpSubscriptions(t *testing.T) {
	t.Parallel()
	// Use hub without Start so subCount == 0 initially.
	hub, nc := newHubNoStart(t)
	srv := wsTestServer(t, hub, nil, nil)

	conn := wsConnect(t, wsURL(srv, "/streaming"))
	wsSend(t, conn, wsClientMsg{Type: "subscribe", Stream: "public"})
	require.Eventually(t, func() bool { return nc.subCount() == 1 }, time.Second, time.Millisecond, "subscribe should create a NATS subscription")

	// Close the connection from the client side.
	require.NoError(t, conn.Close(websocket.StatusNormalClosure, "bye"))
	require.Eventually(t, func() bool { return nc.subCount() == 0 }, time.Second, time.Millisecond, "NATS subscriptions should be released after connection close")
}
