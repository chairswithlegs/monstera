package nats

import (
	"errors"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera-fed/internal/config"
)

var ErrEmptyURL = errors.New("nats: NATS_URL is required")

// Client wraps the NATS connection and JetStream context.
type Client struct {
	Conn *nats.Conn
	JS   jetstream.JetStream
}

// New creates a NATS client with reconnect and error handlers.
func New(cfg *config.Config, logger *slog.Logger) (*Client, error) {
	if cfg == nil || cfg.NATSUrl == "" {
		return nil, ErrEmptyURL
	}
	if logger == nil {
		logger = slog.Default()
	}

	opts := []nats.Option{
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.ReconnectBufSize(16 * 1024 * 1024),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			logger.Warn("nats: disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("nats: reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			logger.Error("nats: error", "error", err, "subject", sub.Subject)
		}),
	}
	if cfg.NATSCredsFile != "" {
		opts = append(opts, nats.UserCredentials(cfg.NATSCredsFile))
	}

	nc, err := nats.Connect(cfg.NATSUrl, opts...)
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, err
	}

	return &Client{Conn: nc, JS: js}, nil
}

// Ping checks connection health. Returns an error if the connection is not healthy.
func (c *Client) Ping() error {
	return c.Conn.FlushTimeout(2 * time.Second)
}

// Close drains the connection: flushes pending publishes and waits for active subscriptions to finish.
func (c *Client) Close() {
	c.Conn.Drain()
}
