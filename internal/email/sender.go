package email

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// Message represents a single outbound email.
// Both HTML and Text should be populated for maximum client compatibility.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
	From    string // optional: overrides Config.From when non-empty
}

// Sender is the email delivery abstraction used throughout Monstera-fed.
// Implementations must be safe for concurrent use by multiple goroutines.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// SendFailedError wraps a provider-specific error with the driver name.
// Callers use errors.As to extract the underlying provider error when needed.
type SendFailedError struct {
	Provider string
	Err      error
}

func (e *SendFailedError) Error() string {
	return fmt.Sprintf("email/%s: send failed: %v", e.Provider, e.Err)
}

func (e *SendFailedError) Unwrap() error {
	return e.Err
}

// Config holds the fields the email factory needs.
type Config struct {
	Driver       string
	From         string
	FromName     string
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	Logger       *slog.Logger
}

// DriverFunc is a constructor for a Sender implementation.
type DriverFunc func(cfg Config) (Sender, error)

var drivers = map[string]DriverFunc{}

// Register registers a Sender driver by name. Called from driver packages in init().
func Register(name string, fn DriverFunc) {
	drivers[name] = fn
}

// New returns the Sender implementation selected by cfg.Driver.
// Driver packages (noop, smtp) must be imported for side-effect registration before calling New.
func New(cfg Config) (Sender, error) {
	driver := cfg.Driver
	if driver == "" {
		driver = "noop"
	}
	fn, ok := drivers[driver]
	if !ok {
		return nil, fmt.Errorf("email: unknown driver %q (valid: noop, smtp)", cfg.Driver)
	}
	if driver == "smtp" && cfg.SMTPHost == "" {
		return nil, errors.New("email: EMAIL_SMTP_HOST is required when EMAIL_DRIVER=smtp")
	}
	return fn(cfg)
}
