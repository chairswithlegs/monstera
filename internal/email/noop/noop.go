// Package noop provides a no-op email Sender that logs messages via slog
// instead of delivering them. Used in development and test environments.
package noop

import (
	"context"
	"log/slog"

	"github.com/chairswithlegs/monstera-fed/internal/email"
)

func init() {
	email.Register("noop", func(cfg email.Config) (email.Sender, error) {
		return New(cfg.Logger, cfg.From, cfg.FromName)
	})
}

// Sender is the no-op email implementation.
type Sender struct {
	logger   *slog.Logger
	from     string
	fromName string
}

// New creates a no-op Sender. Logs a startup message indicating that emails
// will not be delivered.
func New(logger *slog.Logger, from, fromName string) (*Sender, error) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("email driver: noop — emails will be logged to stdout only")
	return &Sender{logger: logger, from: from, fromName: fromName}, nil
}

// Send logs the message via slog. Never delivers.
func (s *Sender) Send(_ context.Context, msg email.Message) error {
	from := msg.From
	if from == "" {
		from = s.from
	}
	s.logger.Info("email sent (noop)",
		slog.String("from", from),
		slog.String("to", msg.To),
		slog.String("subject", msg.Subject),
		slog.Int("html_length", len(msg.HTML)),
		slog.String("text_body", msg.Text),
	)
	return nil
}
