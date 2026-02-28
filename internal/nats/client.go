package nats

import (
	"errors"
	"fmt"
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
func New(cfg *config.Config) (*Client, error) {
	if cfg == nil || cfg.NATSUrl == "" {
		return nil, ErrEmptyURL
	}

	opts := []nats.Option{
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.ReconnectBufSize(16 * 1024 * 1024),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			slog.Warn("nats: disconnected", slog.Any("error", err))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("nats: reconnected", slog.String("url", nc.ConnectedUrl()))
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			slog.Error("nats: error", slog.Any("error", err), slog.String("subject", sub.Subject))
		}),
	}
	if cfg.NATSCredsFile != "" {
		opts = append(opts, nats.UserCredentials(cfg.NATSCredsFile))
	}

	nc, err := nats.Connect(cfg.NATSUrl, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats: connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats: jetstream: %w", err)
	}

	return &Client{Conn: nc, JS: js}, nil
}

// Ping checks connection health. Returns an error if the connection is not healthy.
func (c *Client) Ping() error {
	if err := c.Conn.FlushTimeout(2 * time.Second); err != nil {
		return fmt.Errorf("nats: ping: %w", err)
	}
	return nil
}

// Close drains the connection: flushes pending publishes and waits for active subscriptions to finish.
func (c *Client) Close() {
	_ = c.Conn.Drain()
}
