package testutil

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type MockJetstreamMsg struct {
	DataBytes        []byte
	SubjectString    string
	AckFn            func()
	NakFn            func()
	NumDeliveredUInt uint64
	NakWithDelayFn   func(d time.Duration)
}

func (m *MockJetstreamMsg) Data() []byte         { return m.DataBytes }
func (m *MockJetstreamMsg) Headers() nats.Header { return nil }
func (m *MockJetstreamMsg) Subject() string      { return m.SubjectString }
func (m *MockJetstreamMsg) Reply() string        { return "" }
func (m *MockJetstreamMsg) Ack() error {
	if m.AckFn != nil {
		m.AckFn()
	}
	return nil
}
func (m *MockJetstreamMsg) DoubleAck(context.Context) error { return nil }
func (m *MockJetstreamMsg) Nak() error {
	if m.NakFn != nil {
		m.NakFn()
	}
	return nil
}
func (m *MockJetstreamMsg) NakWithDelay(d time.Duration) error {
	if m.NakWithDelayFn != nil {
		m.NakWithDelayFn(d)
	}
	return nil
}
func (m *MockJetstreamMsg) InProgress() error           { return nil }
func (m *MockJetstreamMsg) Term() error                 { return nil }
func (m *MockJetstreamMsg) TermWithReason(string) error { return nil }
func (m *MockJetstreamMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{NumDelivered: m.NumDeliveredUInt}, nil
}
