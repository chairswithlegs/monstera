package natsutil

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

// MsgHandler is a callback for NATS core pub/sub messages.
// It receives the subject and raw message data, decoupling callers from
// the nats.go library's *nats.Msg type.
type MsgHandler func(subject string, data []byte)

// Publisher publishes messages to NATS core subjects.
// *nats.Conn satisfies this interface directly.
type Publisher interface {
	Publish(subject string, data []byte) error
}

// Subscription represents an active NATS subscription that can be unsubscribed.
type Subscription interface {
	Unsubscribe() error
}

// Subscriber subscribes to NATS core subjects.
type Subscriber interface {
	Subscribe(subject string, handler MsgHandler) (Subscription, error)
}

// ConnSubscriber adapts *nats.Conn to the Subscriber interface by converting
// nats.MsgHandler callbacks to MsgHandler callbacks.
type ConnSubscriber struct {
	conn *nats.Conn
}

// NewConnSubscriber wraps a *nats.Conn as a Subscriber.
func NewConnSubscriber(conn *nats.Conn) *ConnSubscriber {
	return &ConnSubscriber{conn: conn}
}

func (a *ConnSubscriber) Subscribe(subject string, handler MsgHandler) (Subscription, error) {
	sub, err := a.conn.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Subject, msg.Data)
	})
	if err != nil {
		return nil, fmt.Errorf("nats subscribe %s: %w", subject, err)
	}
	return sub, nil
}
