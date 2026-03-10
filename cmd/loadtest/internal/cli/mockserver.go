package cli

// mockserver.go — mock inbox server that counts incoming ActivityPub deliveries.

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// DeliveryRecord captures metadata about a single delivery.
type DeliveryRecord struct {
	ReceivedAt time.Time
	BodySize   int
}

// MockInboxServer listens on a random local port and records incoming POSTs.
type MockInboxServer struct {
	InboxURL   string
	listener   net.Listener
	server     *http.Server
	deliveries chan DeliveryRecord
	received   atomic.Int64
}

// NewMockInboxServer creates and starts a mock inbox server on a random local port.
// bufSize is the channel buffer for deliveries.
// hostIP, when non-empty, causes the server to bind on 0.0.0.0 and advertise
// http://hostIP:PORT as the InboxURL (needed when the delivery worker runs in Docker).
func NewMockInboxServer(bufSize int, hostIP string) (*MockInboxServer, error) {
	bindAddr := "127.0.0.1:0"
	if hostIP != "" {
		bindAddr = "0.0.0.0:0"
	}
	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", bindAddr)
	if err != nil {
		return nil, fmt.Errorf("mockserver: listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	advertise := ln.Addr().String()
	if hostIP != "" {
		advertise = fmt.Sprintf("%s:%d", hostIP, port)
	}
	ms := &MockInboxServer{
		InboxURL:   "http://" + advertise + "/inbox",
		listener:   ln,
		deliveries: make(chan DeliveryRecord, bufSize),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/inbox", ms.handleInbox)

	ms.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = ms.server.Serve(ln) }()
	return ms, nil
}

func (ms *MockInboxServer) handleInbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	ms.received.Add(1)
	select {
	case ms.deliveries <- DeliveryRecord{ReceivedAt: time.Now(), BodySize: len(body)}:
	default:
	}
	w.WriteHeader(http.StatusAccepted)
}

// WaitForDeliveries blocks until n deliveries are received or ctx is cancelled.
// Returns the slice of DeliveryRecords collected.
func (ms *MockInboxServer) WaitForDeliveries(ctx context.Context, n int) []DeliveryRecord {
	records := make([]DeliveryRecord, 0, n)
	for len(records) < n {
		select {
		case <-ctx.Done():
			return records
		case rec := <-ms.deliveries:
			records = append(records, rec)
		}
	}
	return records
}

// Received returns the total number of deliveries received so far.
func (ms *MockInboxServer) Received() int64 {
	return ms.received.Load()
}

// Shutdown gracefully stops the mock inbox server.
func (ms *MockInboxServer) Shutdown(ctx context.Context) error {
	if err := ms.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("mockserver shutdown: %w", err)
	}
	return nil
}
