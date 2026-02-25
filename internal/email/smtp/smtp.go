// Package smtp provides an SMTP-based email Sender using github.com/jordan-wright/email.
package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	gosmtp "net/smtp"

	emaillib "github.com/jordan-wright/email"

	"github.com/chairswithlegs/monstera-fed/internal/email"
)

func init() {
	email.Register("smtp", func(cfg email.Config) (email.Sender, error) {
		port := cfg.SMTPPort
		if port == 0 {
			port = 587
		}
		return New(Config{
			Host:     cfg.SMTPHost,
			Port:     port,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.From,
			FromName: cfg.FromName,
		})
	})
}

// Config holds SMTP-specific configuration.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
}

// Sender is the SMTP email implementation.
type Sender struct {
	cfg Config
}

// New creates an SMTP Sender. No connection is established at construction time.
func New(cfg Config) (*Sender, error) {
	return &Sender{cfg: cfg}, nil
}

// Send delivers the message via SMTP. Port 587 uses STARTTLS; 465 uses implicit TLS; 25 is plain.
func (s *Sender) Send(ctx context.Context, msg email.Message) error {
	_ = ctx
	e := emaillib.NewEmail()

	from := msg.From
	if from == "" {
		from = s.cfg.From
	}
	if s.cfg.FromName != "" && msg.From == "" {
		e.From = fmt.Sprintf("%s <%s>", s.cfg.FromName, from)
	} else {
		e.From = from
	}

	e.To = []string{msg.To}
	e.Subject = msg.Subject
	e.Text = []byte(msg.Text)
	e.HTML = []byte(msg.HTML)

	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.cfg.Port))

	var auth gosmtp.Auth
	if s.cfg.Username != "" {
		auth = gosmtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	tlsCfg := &tls.Config{ServerName: s.cfg.Host}

	var sendErr error
	switch s.cfg.Port {
	case 465:
		sendErr = e.SendWithTLS(addr, auth, tlsCfg)
	case 25:
		sendErr = e.Send(addr, auth)
	default:
		sendErr = e.SendWithStartTLS(addr, auth, tlsCfg)
	}
	if sendErr != nil {
		return &email.ErrSendFailed{Provider: "smtp", Err: sendErr}
	}
	return nil
}
