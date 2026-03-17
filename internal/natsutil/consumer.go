package natsutil

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// ConsumeOpt configures RunConsumer behaviour.
type ConsumeOpt func(*consumeConfig)

type consumeConfig struct {
	maxMessages int
	pullExpiry  time.Duration
	label       string
}

// WithMaxMessages sets the PullMaxMessages count. Default: 10.
func WithMaxMessages(n int) ConsumeOpt {
	return func(c *consumeConfig) { c.maxMessages = n }
}

// WithLabel overrides the label used in log messages. Default: consumer name.
func WithLabel(label string) ConsumeOpt {
	return func(c *consumeConfig) { c.label = label }
}

// RunConsumer obtains the named durable consumer from the given stream, starts
// consuming messages with the provided handler, and blocks until ctx is
// cancelled. On cancellation it stops the consumer and waits for in-flight
// message processing to complete.
func RunConsumer(ctx context.Context, js jetstream.JetStream, stream, consumer string, handler func(jetstream.Msg), opts ...ConsumeOpt) error {
	cfg := consumeConfig{
		maxMessages: 10,
		pullExpiry:  5 * time.Second,
		label:       consumer,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	cons, err := js.Consumer(ctx, stream, consumer)
	if err != nil {
		return fmt.Errorf("%s: get consumer: %w", cfg.label, err)
	}

	slog.Info(cfg.label+" started", slog.String("consumer", consumer))

	consCtx, err := cons.Consume(
		handler,
		jetstream.PullMaxMessages(cfg.maxMessages),
		jetstream.PullExpiry(cfg.pullExpiry),
		jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
			if ctx.Err() == nil {
				slog.Warn(cfg.label+" consume error", slog.Any("error", err))
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("%s: consume: %w", cfg.label, err)
	}

	<-ctx.Done()
	slog.Info(cfg.label + " stopping")
	consCtx.Stop()
	<-consCtx.Closed()
	return nil
}
