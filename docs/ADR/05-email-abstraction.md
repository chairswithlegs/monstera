# ADR 05 — Email Abstraction

> **Status:** Draft v0.1 — Feb 24, 2026  
> **Addresses:** `design-prompts/05-email-abstraction.md`

---

## Design Decisions

| Question | Decision |
|----------|----------|
| Implementation count | **Two drivers: noop + SMTP** — SendGrid, Postmark, SES, and Mailgun all expose SMTP endpoints, so a single SMTP driver covers every provider. Vendor-specific HTTP API drivers can be added later without changing the `Sender` interface. |
| SMTP library | **`github.com/jordan-wright/email`** — lightweight, handles STARTTLS and multipart MIME cleanly; no heavy framework needed for low-volume transactional email |
| Connection reuse vs. per-message dial | **Per-message dial** — transactional email volume is low (registrations, resets); connection pooling adds complexity with no measurable benefit |
| Token storage | **Dedicated `email_tokens` DB table** — tokens survive cache evictions, server restarts, and multi-replica deployments; negligible write volume |
| Token format | **32 bytes crypto/rand, base64url-encoded** — stored as SHA-256 hash in DB; raw token only in the email URL |
| Template engine | `html/template` for HTML variants, `text/template` for text variants — no external dependency |
| Factory input | `email.Config` struct (not `*config.Config`) — keeps the email package dependency-free, matching cache and media patterns |
| `Message.From` override | Optional — defaults to `Config.From`/`Config.FromName` when empty |

---

## File Layout

```
internal/email/
├── sender.go             — Message, Sender interface, ErrSendFailed, Config, New factory
├── templates.go          — embed.FS, Templates type, Render method, per-template data structs
├── templates/
│   ├── email_verification.html
│   ├── email_verification.txt
│   ├── password_reset.html
│   ├── password_reset.txt
│   ├── invite.html
│   ├── invite.txt
│   ├── moderation_action.html
│   └── moderation_action.txt
├── noop/
│   └── noop.go
└── smtp/
    └── smtp.go

internal/service/
└── email_service.go

internal/store/migrations/
└── 000023_create_email_tokens.up.sql
└── 000023_create_email_tokens.down.sql

internal/store/postgres/queries/
└── email_tokens.sql
```

---

## 1. `internal/email/sender.go`

```go
package email

import (
	"context"
	"fmt"
	"log/slog"
)

// Message represents a single outbound email.
// Both HTML and Text should be populated for maximum client compatibility.
type Message struct {
	To      string // recipient email address
	Subject string
	HTML    string // rich HTML body
	Text    string // plain-text fallback body
	From    string // optional: overrides Config.From when non-empty
}

// Sender is the email delivery abstraction used throughout Monstera.
// Implementations must be safe for concurrent use by multiple goroutines.
type Sender interface {
	// Send delivers a single email message. Returns ErrSendFailed wrapping the
	// provider-specific error on delivery failure.
	Send(ctx context.Context, msg Message) error
}

// ErrSendFailed wraps a provider-specific error with the driver name for
// structured logging and error inspection. Callers use errors.As to extract
// the underlying provider error when needed.
type ErrSendFailed struct {
	Provider string // "smtp" (or future drivers)
	Err      error  // underlying provider error
}

func (e *ErrSendFailed) Error() string {
	return fmt.Sprintf("email/%s: send failed: %v", e.Provider, e.Err)
}

func (e *ErrSendFailed) Unwrap() error {
	return e.Err
}

// Config holds the fields the email factory needs.
// Constructed by serve.go from *config.Config so that this package has
// no dependency on internal/config — matching the cache and media patterns.
type Config struct {
	Driver       string // "noop"|"smtp"
	From         string // default sender address (e.g. "noreply@example.com")
	FromName     string // default sender display name (e.g. "Monstera")
	SMTPHost     string // required when Driver == "smtp"
	SMTPPort     int    // default: 587
	SMTPUsername string
	SMTPPassword string
	Logger       *slog.Logger
}

// New returns the Sender implementation selected by cfg.Driver.
// Returns an error if the driver is unknown or if required configuration
// fields are missing for the selected driver.
//
// Note on managed services: SendGrid, Postmark, SES, and Mailgun all expose
// SMTP endpoints. Configure EMAIL_DRIVER=smtp with their SMTP host and
// credentials — no vendor-specific driver is needed.
//
// Examples:
//   SendGrid:  SMTP_HOST=smtp.sendgrid.net, USERNAME=apikey, PASSWORD=<API key>
//   Postmark:  SMTP_HOST=smtp.postmarkapp.com, USERNAME=<server token>, PASSWORD=<server token>
//   SES:       SMTP_HOST=email-smtp.us-east-1.amazonaws.com, USERNAME=<SMTP user>, PASSWORD=<SMTP pass>
//   Mailgun:   SMTP_HOST=smtp.mailgun.org, USERNAME=<user>, PASSWORD=<password>
func New(cfg Config) (Sender, error) {
	switch cfg.Driver {
	case "noop", "":
		return noop.New(cfg.Logger, cfg.From, cfg.FromName)
	case "smtp":
		if cfg.SMTPHost == "" {
			return nil, fmt.Errorf("email: EMAIL_SMTP_HOST is required when EMAIL_DRIVER=smtp")
		}
		port := cfg.SMTPPort
		if port == 0 {
			port = 587
		}
		return smtp.New(smtp.Config{
			Host:     cfg.SMTPHost,
			Port:     port,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.From,
			FromName: cfg.FromName,
		})
	default:
		return nil, fmt.Errorf("email: unknown driver %q (valid: noop, smtp)", cfg.Driver)
	}
}
```

> **Note on imports:** The `New` factory references sub-packages `noop` and `smtp` via their full import paths (`github.com/yourorg/monstera/internal/email/noop` etc.). Import paths are omitted above for readability.

---

## 2. `internal/email/noop/noop.go`

```go
// Package noop provides a no-op email Sender that logs messages to stdout
// instead of delivering them. Used in development and test environments.
package noop

import (
	"context"
	"log/slog"

	"github.com/yourorg/monstera/internal/email"
)

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

// Send logs the full message content to stdout via slog. Never delivers.
//
// The complete HTML body is logged so that developers can inspect email
// content (e.g. click confirmation links) without configuring a real
// mail server. In production with EMAIL_DRIVER=noop this is intentional —
// the operator has explicitly chosen not to send email.
func (s *Sender) Send(_ context.Context, msg email.Message) error {
	from := msg.From
	if from == "" {
		from = s.from
	}

	s.logger.Info("email sent (noop)",
		"from", from,
		"to", msg.To,
		"subject", msg.Subject,
		"html_length", len(msg.HTML),
		"text_body", msg.Text,
	)
	return nil
}
```

---

## 3. `internal/email/smtp/smtp.go`

```go
// Package smtp provides an SMTP-based email Sender.
// Uses github.com/jordan-wright/email for multipart MIME construction and delivery.
package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	gosmtp "net/smtp"

	emaillib "github.com/jordan-wright/email"

	"github.com/yourorg/monstera/internal/email"
)

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

// New creates an SMTP Sender. No connection is established at construction
// time — connections are created per-message (see trade-off note below).
//
// Connection reuse trade-off:
//   Transactional email volume for a Mastodon instance is low — typically a few
//   messages per hour (registrations, password resets, moderation notices). A
//   persistent SMTP connection would require keepalive management, reconnection
//   logic, and idle timeout handling. Per-message dial is simpler and sufficient:
//   the ~100ms connection overhead is negligible for these use cases. If volume
//   grows (e.g. bulk invite sends), a pooled SMTP client can be added later
//   without changing the Sender interface.
func New(cfg Config) (*Sender, error) {
	return &Sender{cfg: cfg}, nil
}

// Send constructs a multipart MIME email (text/plain + text/html) and delivers
// it via the jordan-wright/email library.
//
// Port handling:
//   - Port 587: STARTTLS (RFC 3207).
//   - Port 465: implicit TLS (SMTPS).
//   - Port 25: plain SMTP (no TLS). Not recommended for production.
//
// Authentication:
//   - Uses PLAIN auth (RFC 4616) when username and password are configured.
//   - Skipped when username is empty (for local relay servers).
func (s *Sender) Send(ctx context.Context, msg email.Message) error {
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

	// Build auth (nil if no credentials — for local relays).
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
```

**Why `github.com/jordan-wright/email` over stdlib `net/smtp`:**

Constructing a proper multipart MIME message with both `text/plain` and `text/html` parts using only `net/smtp` requires manually building MIME boundaries, Content-Type headers, base64 encoding, and boundary delimiters. This is error-prone — incorrect boundary handling causes email clients to render raw MIME syntax instead of the message body.

`jordan-wright/email` wraps this in a clean API (`e.Text`, `e.HTML`, `e.SendWithStartTLS`) with:
- Correct multipart/alternative MIME construction
- Proper header encoding (RFC 2047 for non-ASCII subjects)
- STARTTLS negotiation
- Zero additional transitive dependencies

`gopkg.in/gomail.v2` was considered but is unmaintained (last commit 2016) and carries a heavier API surface. `jordan-wright/email` is actively maintained and sufficient for Monstera's transactional use case.

---

## 4. `internal/email/templates.go`

```go
package email

import (
	"bytes"
	"embed"
	"fmt"
	htmltpl "html/template"
	texttpl "text/template"
)

//go:embed templates/*.html templates/*.txt
var templateFS embed.FS

// ---- Per-template data structs ----

// VerificationData holds the fields for the email_verification template.
type VerificationData struct {
	InstanceName string
	Username     string
	ConfirmURL   string
}

// PasswordResetData holds the fields for the password_reset template.
type PasswordResetData struct {
	InstanceName string
	Username     string
	ResetURL     string
	ExpiresIn    string // human-readable duration, e.g. "1 hour"
}

// InviteData holds the fields for the invite template.
type InviteData struct {
	InstanceName    string
	InviterUsername string
	InviteURL       string
	Code            string
}

// ModerationActionData holds the fields for the moderation_action template.
type ModerationActionData struct {
	InstanceName string
	Username     string
	Action       string // "warning", "silence", "suspension"
	Reason       string
	AppealURL    string
}

// Templates holds parsed HTML and text templates for all email types.
// Constructed once at startup; safe for concurrent use (html/template and
// text/template Execute methods are safe for concurrent use on a parsed tree).
type Templates struct {
	html *htmltpl.Template
	text *texttpl.Template
}

// NewTemplates parses all embedded template files and returns a Templates
// instance. Returns an error if any template file is missing or malformed.
//
// Template naming convention:
//   - HTML: templates/{name}.html
//   - Text: templates/{name}.txt
//
// Render looks up templates by the {name} portion (e.g. "email_verification").
func NewTemplates() (*Templates, error) {
	html, err := htmltpl.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("email: parse HTML templates: %w", err)
	}

	text, err := texttpl.ParseFS(templateFS, "templates/*.txt")
	if err != nil {
		return nil, fmt.Errorf("email: parse text templates: %w", err)
	}

	return &Templates{html: html, text: text}, nil
}

// Render executes the named template with the given data and returns the
// rendered HTML and plain-text strings.
//
// name must match the template filename without extension (e.g. "email_verification").
// data should be the corresponding typed struct (VerificationData, PasswordResetData, etc.).
//
// Returns an error if the template is not found or execution fails.
func (t *Templates) Render(name string, data any) (htmlOut, textOut string, err error) {
	var htmlBuf bytes.Buffer
	if err := t.html.ExecuteTemplate(&htmlBuf, name+".html", data); err != nil {
		return "", "", fmt.Errorf("email: render HTML template %q: %w", name, err)
	}

	var textBuf bytes.Buffer
	if err := t.text.ExecuteTemplate(&textBuf, name+".txt", data); err != nil {
		return "", "", fmt.Errorf("email: render text template %q: %w", name, err)
	}

	return htmlBuf.String(), textBuf.String(), nil
}
```

---

## 5. Template Files

### `templates/email_verification.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Confirm your email</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f5;">
    <tr>
      <td align="center" style="padding:40px 20px;">
        <table role="presentation" width="560" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;overflow:hidden;">
          <tr>
            <td style="padding:32px 40px 24px;text-align:center;">
              <h1 style="margin:0 0 8px;font-size:22px;font-weight:600;color:#1a1a2e;">{{.InstanceName}}</h1>
              <p style="margin:0;font-size:14px;color:#6b7280;">Confirm your email address</p>
            </td>
          </tr>
          <tr>
            <td style="padding:0 40px 32px;">
              <p style="margin:0 0 16px;font-size:15px;line-height:1.6;color:#374151;">
                Hello <strong>{{.Username}}</strong>,
              </p>
              <p style="margin:0 0 24px;font-size:15px;line-height:1.6;color:#374151;">
                Please confirm your email address to complete your registration on {{.InstanceName}}.
              </p>
              <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto;">
                <tr>
                  <td style="border-radius:6px;background-color:#6366f1;">
                    <a href="{{.ConfirmURL}}" target="_blank" style="display:inline-block;padding:12px 32px;font-size:15px;font-weight:600;color:#ffffff;text-decoration:none;border-radius:6px;">
                      Confirm Email
                    </a>
                  </td>
                </tr>
              </table>
              <p style="margin:24px 0 0;font-size:13px;line-height:1.5;color:#9ca3af;">
                If the button doesn't work, copy and paste this link into your browser:<br>
                <a href="{{.ConfirmURL}}" style="color:#6366f1;word-break:break-all;">{{.ConfirmURL}}</a>
              </p>
              <p style="margin:16px 0 0;font-size:13px;line-height:1.5;color:#9ca3af;">
                If you did not create an account, you can safely ignore this email.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>
```

### `templates/email_verification.txt`

```
{{.InstanceName}} — Confirm your email address

Hello {{.Username}},

Please confirm your email address to complete your registration on {{.InstanceName}}.

Click the link below to confirm:

{{.ConfirmURL}}

If you did not create an account, you can safely ignore this email.
```

### `templates/password_reset.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Reset your password</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f5;">
    <tr>
      <td align="center" style="padding:40px 20px;">
        <table role="presentation" width="560" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;overflow:hidden;">
          <tr>
            <td style="padding:32px 40px 24px;text-align:center;">
              <h1 style="margin:0 0 8px;font-size:22px;font-weight:600;color:#1a1a2e;">{{.InstanceName}}</h1>
              <p style="margin:0;font-size:14px;color:#6b7280;">Password reset request</p>
            </td>
          </tr>
          <tr>
            <td style="padding:0 40px 32px;">
              <p style="margin:0 0 16px;font-size:15px;line-height:1.6;color:#374151;">
                Hello <strong>{{.Username}}</strong>,
              </p>
              <p style="margin:0 0 24px;font-size:15px;line-height:1.6;color:#374151;">
                We received a request to reset your password on {{.InstanceName}}. Click the button below to choose a new password. This link expires in <strong>{{.ExpiresIn}}</strong>.
              </p>
              <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto;">
                <tr>
                  <td style="border-radius:6px;background-color:#6366f1;">
                    <a href="{{.ResetURL}}" target="_blank" style="display:inline-block;padding:12px 32px;font-size:15px;font-weight:600;color:#ffffff;text-decoration:none;border-radius:6px;">
                      Reset Password
                    </a>
                  </td>
                </tr>
              </table>
              <p style="margin:24px 0 0;font-size:13px;line-height:1.5;color:#9ca3af;">
                If the button doesn't work, copy and paste this link into your browser:<br>
                <a href="{{.ResetURL}}" style="color:#6366f1;word-break:break-all;">{{.ResetURL}}</a>
              </p>
              <p style="margin:16px 0 0;font-size:13px;line-height:1.5;color:#9ca3af;">
                If you did not request a password reset, you can safely ignore this email. Your password will not be changed.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>
```

### `templates/password_reset.txt`

```
{{.InstanceName}} — Password Reset

Hello {{.Username}},

We received a request to reset your password on {{.InstanceName}}.

Click the link below to choose a new password. This link expires in {{.ExpiresIn}}.

{{.ResetURL}}

If you did not request a password reset, you can safely ignore this email. Your password will not be changed.
```

### `templates/invite.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>You're invited</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f5;">
    <tr>
      <td align="center" style="padding:40px 20px;">
        <table role="presentation" width="560" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;overflow:hidden;">
          <tr>
            <td style="padding:32px 40px 24px;text-align:center;">
              <h1 style="margin:0 0 8px;font-size:22px;font-weight:600;color:#1a1a2e;">{{.InstanceName}}</h1>
              <p style="margin:0;font-size:14px;color:#6b7280;">You've been invited!</p>
            </td>
          </tr>
          <tr>
            <td style="padding:0 40px 32px;">
              <p style="margin:0 0 16px;font-size:15px;line-height:1.6;color:#374151;">
                <strong>{{.InviterUsername}}</strong> has invited you to join {{.InstanceName}}.
              </p>
              <p style="margin:0 0 24px;font-size:15px;line-height:1.6;color:#374151;">
                Use the link below to create your account.
              </p>
              <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto;">
                <tr>
                  <td style="border-radius:6px;background-color:#6366f1;">
                    <a href="{{.InviteURL}}" target="_blank" style="display:inline-block;padding:12px 32px;font-size:15px;font-weight:600;color:#ffffff;text-decoration:none;border-radius:6px;">
                      Accept Invite
                    </a>
                  </td>
                </tr>
              </table>
              <p style="margin:24px 0 0;font-size:13px;line-height:1.5;color:#9ca3af;">
                Your invite code: <strong style="color:#374151;">{{.Code}}</strong>
              </p>
              <p style="margin:8px 0 0;font-size:13px;line-height:1.5;color:#9ca3af;">
                If the button doesn't work, copy and paste this link into your browser:<br>
                <a href="{{.InviteURL}}" style="color:#6366f1;word-break:break-all;">{{.InviteURL}}</a>
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>
```

### `templates/invite.txt`

```
{{.InstanceName}} — You're Invited!

{{.InviterUsername}} has invited you to join {{.InstanceName}}.

Use the link below to create your account:

{{.InviteURL}}

Your invite code: {{.Code}}
```

### `templates/moderation_action.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Account notice</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f5;">
    <tr>
      <td align="center" style="padding:40px 20px;">
        <table role="presentation" width="560" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;overflow:hidden;">
          <tr>
            <td style="padding:32px 40px 24px;text-align:center;">
              <h1 style="margin:0 0 8px;font-size:22px;font-weight:600;color:#1a1a2e;">{{.InstanceName}}</h1>
              <p style="margin:0;font-size:14px;color:#6b7280;">Account notice</p>
            </td>
          </tr>
          <tr>
            <td style="padding:0 40px 32px;">
              <p style="margin:0 0 16px;font-size:15px;line-height:1.6;color:#374151;">
                Hello <strong>{{.Username}}</strong>,
              </p>
              <p style="margin:0 0 16px;font-size:15px;line-height:1.6;color:#374151;">
                The moderation team at {{.InstanceName}} has taken the following action on your account:
              </p>
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 16px;border-radius:6px;background-color:#fef2f2;border:1px solid #fecaca;">
                <tr>
                  <td style="padding:16px 20px;">
                    <p style="margin:0 0 4px;font-size:13px;font-weight:600;color:#991b1b;text-transform:uppercase;letter-spacing:0.05em;">{{.Action}}</p>
                    <p style="margin:0;font-size:14px;line-height:1.5;color:#374151;">{{.Reason}}</p>
                  </td>
                </tr>
              </table>
              <p style="margin:0 0 24px;font-size:15px;line-height:1.6;color:#374151;">
                If you believe this action was taken in error, you may submit an appeal.
              </p>
              <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto;">
                <tr>
                  <td style="border-radius:6px;background-color:#6366f1;">
                    <a href="{{.AppealURL}}" target="_blank" style="display:inline-block;padding:12px 32px;font-size:15px;font-weight:600;color:#ffffff;text-decoration:none;border-radius:6px;">
                      Submit Appeal
                    </a>
                  </td>
                </tr>
              </table>
              <p style="margin:24px 0 0;font-size:13px;line-height:1.5;color:#9ca3af;">
                If the button doesn't work, copy and paste this link into your browser:<br>
                <a href="{{.AppealURL}}" style="color:#6366f1;word-break:break-all;">{{.AppealURL}}</a>
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>
```

### `templates/moderation_action.txt`

```
{{.InstanceName}} — Account Notice

Hello {{.Username}},

The moderation team at {{.InstanceName}} has taken the following action on your account:

Action: {{.Action}}
Reason: {{.Reason}}

If you believe this action was taken in error, you may submit an appeal:

{{.AppealURL}}
```

---

## 6. Token Storage Design

### Comparison

| Criteria | Cache (`confirm:{token}` → userID) | DB table (`email_tokens`) |
|----------|-------------------------------------|---------------------------|
| **Durability** | Lost on cache eviction, memory driver restart, or Redis flush | Persists until explicitly consumed or expired |
| **Multi-replica** | Requires `CACHE_DRIVER=redis`; memory driver would lose tokens on the replica that didn't write them | Works on all replicas via shared PostgreSQL |
| **Replay prevention** | Delete key on use; race condition possible without atomic get-and-delete (Redis supports `GETDEL`; memory driver does not) | `consumed_at IS NOT NULL` check with row-level locking; no race condition |
| **Auditability** | No record of issued/consumed tokens | Full audit trail: issued, consumed, expired |
| **Cleanup** | Automatic via TTL | Periodic `DELETE WHERE expires_at < NOW()` reaper query |
| **Write volume** | Negligible | Negligible (one row per registration or password reset) |
| **Complexity** | Simpler — no migration needed | Requires a new table and migration |

### Recommendation: DB table

A confirmation token that survives for 24 hours must not be lost to cache eviction. A user who registers and checks their email 12 hours later expects the link to work. The memory cache driver has no durability guarantees, and even Redis under memory pressure may evict keys before their TTL expires (when `maxmemory-policy` allows it).

The DB table also provides atomic consume-or-reject semantics: a `SELECT ... WHERE consumed_at IS NULL FOR UPDATE` inside a transaction guarantees that a token can only be used once, even under concurrent requests. The cache approach requires `GETDEL` (Redis 6.2+), which the `cache.Store` interface does not expose.

The write volume is negligible — one row per registration, one per password reset. A daily reaper query (`DELETE FROM email_tokens WHERE expires_at < NOW()`) keeps the table small.

### Token Generation and Storage

```go
// internal/service/email_service.go (token helpers)

// generateToken produces a cryptographically random, URL-safe token string.
// 32 bytes of entropy → 43 characters base64url-encoded.
func generateToken() (raw string, hashed string, err error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", "", fmt.Errorf("generate token: %w", err)
    }
    raw = base64.RawURLEncoding.EncodeToString(b)
    h := sha256.Sum256([]byte(raw))
    hashed = hex.EncodeToString(h[:])
    return raw, hashed, nil
}
```

**Why hash the token before storage:**

The raw token is sent in the email URL. The database stores only the SHA-256 hash. If the database is compromised, an attacker cannot reconstruct valid confirmation/reset URLs from the hashed values. This is the same pattern used for OAuth access tokens in the cache (ADR 03, §6).

### URL Construction

```
Confirm: https://{INSTANCE_DOMAIN}/auth/confirm?token={rawToken}
Reset:   https://{INSTANCE_DOMAIN}/auth/reset?token={rawToken}
Appeal:  https://{INSTANCE_DOMAIN}/auth/appeal
Invite:  https://{INSTANCE_DOMAIN}/auth/register?invite={code}
```

These are thin server-side routes that validate the token and render a form (confirmation) or redirect (email verified). They are NOT API endpoints — they serve HTML and are accessed by clicking the email link in a browser.

---

## 7. Migration: `email_tokens`

### `000023_create_email_tokens.up.sql`

```sql
CREATE TABLE email_tokens (
    id          TEXT PRIMARY KEY,
    token_hash  TEXT NOT NULL UNIQUE,            -- SHA-256 of the raw token
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     TEXT NOT NULL,                   -- 'confirmation'|'password_reset'
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,                     -- set on use; NULL until consumed
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Token lookup on confirmation/reset: find valid, unconsumed token.
CREATE INDEX idx_email_tokens_hash ON email_tokens (token_hash)
    WHERE consumed_at IS NULL;

-- Reaper query: delete expired tokens.
CREATE INDEX idx_email_tokens_expires ON email_tokens (expires_at)
    WHERE consumed_at IS NULL;
```

### `000023_create_email_tokens.down.sql`

```sql
DROP TABLE IF EXISTS email_tokens CASCADE;
```

### `sqlc` Queries: `email_tokens.sql`

```sql
-- name: CreateEmailToken :one
INSERT INTO email_tokens (id, token_hash, user_id, purpose, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetEmailToken :one
-- Returns the token row only if it has not been consumed and has not expired.
SELECT * FROM email_tokens
WHERE token_hash = $1
  AND consumed_at IS NULL
  AND expires_at > NOW();

-- name: ConsumeEmailToken :exec
-- Atomically marks a token as consumed. The WHERE clause ensures idempotency:
-- a second call with the same hash is a no-op (0 rows affected).
UPDATE email_tokens SET consumed_at = NOW()
WHERE token_hash = $1 AND consumed_at IS NULL;

-- name: DeleteExpiredEmailTokens :exec
-- Called by a periodic reaper (e.g. daily cron or startup cleanup).
DELETE FROM email_tokens WHERE expires_at < NOW();

-- name: DeleteEmailTokensForUser :exec
-- Invalidates all pending tokens for a user (e.g. on password change).
DELETE FROM email_tokens WHERE user_id = $1 AND consumed_at IS NULL;
```

### Store Interface Addition

```go
// Added to internal/store/store.go:

type EmailTokenStore interface {
    CreateEmailToken(ctx context.Context, arg db.CreateEmailTokenParams) (EmailToken, error)
    GetEmailToken(ctx context.Context, tokenHash string) (EmailToken, error)
    ConsumeEmailToken(ctx context.Context, tokenHash string) error
    DeleteExpiredEmailTokens(ctx context.Context) error
    DeleteEmailTokensForUser(ctx context.Context, userID string) error
}
```

The root `Store` interface gains `EmailTokenStore` as an additional embedded interface.

---

## 8. `internal/service/email_service.go`

```go
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/yourorg/monstera/internal/email"
	"github.com/yourorg/monstera/internal/store"
	db "github.com/yourorg/monstera/internal/store/postgres/generated"
	"github.com/yourorg/monstera/internal/uid"
)

// Token TTLs.
const (
	confirmationTokenTTL  = 24 * time.Hour
	passwordResetTokenTTL = 1 * time.Hour
)

// EmailService orchestrates transactional email sending: token generation,
// template rendering, and delivery via the pluggable Sender.
type EmailService struct {
	sender    email.Sender
	templates *email.Templates
	store     store.Store // for email_tokens table access
	from      string
	fromName  string
	baseURL   string // "https://{INSTANCE_DOMAIN}" — used to construct confirmation/reset URLs
}

// NewEmailService constructs an EmailService.
// baseURL should be the full scheme + domain (e.g. "https://social.example.com")
// with no trailing slash.
func NewEmailService(
	sender email.Sender,
	templates *email.Templates,
	s store.Store,
	from, fromName, baseURL string,
) *EmailService {
	return &EmailService{
		sender:    sender,
		templates: templates,
		store:     s,
		from:      from,
		fromName:  fromName,
		baseURL:   baseURL,
	}
}

// SendVerification generates a confirmation token, stores it, renders the
// email_verification template, and sends the email.
//
// Called during account registration (both approval and invite modes).
// The token is valid for 24 hours. On click, the /auth/confirm handler
// validates the token and sets users.confirmed_at.
func (s *EmailService) SendVerification(ctx context.Context, to, username, userID string) error {
	rawToken, tokenHash, err := generateToken()
	if err != nil {
		return err
	}

	if _, err := s.store.CreateEmailToken(ctx, db.CreateEmailTokenParams{
		ID:        uid.New(),
		TokenHash: tokenHash,
		UserID:    userID,
		Purpose:   "confirmation",
		ExpiresAt: time.Now().Add(confirmationTokenTTL),
	}); err != nil {
		return fmt.Errorf("email: store confirmation token: %w", err)
	}

	confirmURL := fmt.Sprintf("%s/auth/confirm?token=%s", s.baseURL, rawToken)

	htmlBody, textBody, err := s.templates.Render("email_verification", email.VerificationData{
		InstanceName: s.fromName,
		Username:     username,
		ConfirmURL:   confirmURL,
	})
	if err != nil {
		return err
	}

	return s.sender.Send(ctx, email.Message{
		To:      to,
		Subject: fmt.Sprintf("Confirm your email — %s", s.fromName),
		HTML:    htmlBody,
		Text:    textBody,
	})
}

// SendPasswordReset generates a reset token, stores it, renders the
// password_reset template, and sends the email.
//
// The token is valid for 1 hour. Any existing pending reset tokens for the
// user are NOT invalidated — the new token works independently. On password
// change, all pending tokens are deleted via DeleteEmailTokensForUser.
func (s *EmailService) SendPasswordReset(ctx context.Context, to, username, userID string) error {
	rawToken, tokenHash, err := generateToken()
	if err != nil {
		return err
	}

	if _, err := s.store.CreateEmailToken(ctx, db.CreateEmailTokenParams{
		ID:        uid.New(),
		TokenHash: tokenHash,
		UserID:    userID,
		Purpose:   "password_reset",
		ExpiresAt: time.Now().Add(passwordResetTokenTTL),
	}); err != nil {
		return fmt.Errorf("email: store reset token: %w", err)
	}

	resetURL := fmt.Sprintf("%s/auth/reset?token=%s", s.baseURL, rawToken)

	htmlBody, textBody, err := s.templates.Render("password_reset", email.PasswordResetData{
		InstanceName: s.fromName,
		Username:     username,
		ResetURL:     resetURL,
		ExpiresIn:    "1 hour",
	})
	if err != nil {
		return err
	}

	return s.sender.Send(ctx, email.Message{
		To:      to,
		Subject: fmt.Sprintf("Reset your password — %s", s.fromName),
		HTML:    htmlBody,
		Text:    textBody,
	})
}

// SendInvite renders the invite template and sends the email.
//
// No token is generated — the invite code itself is the credential,
// stored in the invites table (see ADR 02, migration 000014).
// The invite URL includes the code as a query parameter so the registration
// form can pre-fill it.
func (s *EmailService) SendInvite(ctx context.Context, to, inviterUsername, code string) error {
	inviteURL := fmt.Sprintf("%s/auth/register?invite=%s", s.baseURL, code)

	htmlBody, textBody, err := s.templates.Render("invite", email.InviteData{
		InstanceName:    s.fromName,
		InviterUsername: inviterUsername,
		InviteURL:       inviteURL,
		Code:            code,
	})
	if err != nil {
		return err
	}

	return s.sender.Send(ctx, email.Message{
		To:      to,
		Subject: fmt.Sprintf("You're invited to %s", s.fromName),
		HTML:    htmlBody,
		Text:    textBody,
	})
}

// SendModerationAction renders the moderation_action template and sends the email.
//
// Called by the moderation service when an admin warns, silences, or suspends
// an account. The appeal URL is a static page — no token is needed because
// the appeal is submitted by an authenticated user from the client.
func (s *EmailService) SendModerationAction(ctx context.Context, to, username, action, reason string) error {
	appealURL := fmt.Sprintf("%s/auth/appeal", s.baseURL)

	htmlBody, textBody, err := s.templates.Render("moderation_action", email.ModerationActionData{
		InstanceName: s.fromName,
		Username:     username,
		Action:       action,
		Reason:       reason,
		AppealURL:    appealURL,
	})
	if err != nil {
		return err
	}

	return s.sender.Send(ctx, email.Message{
		To:      to,
		Subject: fmt.Sprintf("Account notice — %s", s.fromName),
		HTML:    htmlBody,
		Text:    textBody,
	})
}

// generateToken produces a cryptographically random, URL-safe token and its
// SHA-256 hash. The raw token is placed in the email URL; the hash is stored
// in the database. A database compromise does not reveal usable tokens.
func generateToken() (raw string, hashed string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hashed = hex.EncodeToString(h[:])
	return raw, hashed, nil
}
```

### Token Verification (consumed by auth handlers)

The `/auth/confirm` and `/auth/reset` handlers — not part of the email package, but documented here for completeness — follow this pattern:

```go
// In the auth handler (internal/api/auth.go or similar):

func (h *AuthHandler) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
    rawToken := r.URL.Query().Get("token")
    if rawToken == "" {
        // render error page
        return
    }

    tokenHash := sha256Hex(rawToken)

    err := h.store.WithTx(r.Context(), func(tx store.Store) error {
        tok, err := tx.GetEmailToken(r.Context(), tokenHash)
        if err != nil {
            return err // token not found, expired, or already consumed
        }
        if tok.Purpose != "confirmation" {
            return fmt.Errorf("invalid token purpose")
        }
        if err := tx.ConsumeEmailToken(r.Context(), tokenHash); err != nil {
            return err
        }
        return tx.ConfirmUser(r.Context(), tok.UserID)
    })
    // render success or error page based on err
}
```

The transaction ensures that consuming the token and confirming the user happen atomically — a crash between the two operations cannot leave the system in an inconsistent state.

---

## 9. Startup Wiring

The email subsystem is initialised in `cmd/monstera/serve.go` at step 9 (see ADR 01, §4):

```go
// Step 9: Build email sender + templates
emailCfg := email.Config{
    Driver:       cfg.EmailDriver,
    From:         cfg.EmailFrom,
    FromName:     cfg.EmailFromName,
    SMTPHost:     cfg.EmailSMTPHost,
    SMTPPort:     cfg.EmailSMTPPort,
    SMTPUsername: cfg.EmailSMTPUsername,
    SMTPPassword: cfg.EmailSMTPPassword,
    Logger:       logger,
}
emailSender, err := email.New(emailCfg)
if err != nil {
    logger.Error("failed to initialise email sender", "error", err)
    os.Exit(1)
}

emailTemplates, err := email.NewTemplates()
if err != nil {
    logger.Error("failed to parse email templates", "error", err)
    os.Exit(1)
}

baseURL := fmt.Sprintf("https://%s", cfg.InstanceDomain)
emailSvc := service.NewEmailService(
    emailSender, emailTemplates, store, cfg.EmailFrom, cfg.EmailFromName, baseURL,
)
```

---

## 10. `go.mod` Additions

```
require (
    github.com/jordan-wright/email     v4.x.x
)
```

Run:
```bash
go get github.com/jordan-wright/email
```

**No new dependency for noop** — uses only `log/slog`.

---

## 11. Open Questions

| # | Question | Impact |
|---|----------|--------|
| 1 | **`List-Unsubscribe` header:** RFC 8058 recommends a `List-Unsubscribe-Post` header for transactional email to satisfy Gmail/Yahoo deliverability requirements (enforced since Feb 2024). Monstera's emails are account-lifecycle (not marketing), so the requirement is ambiguous. Should all outbound emails include this header? | Low — additive; can be added to Message as an optional header map |
| 2 | **Confirmation token reissuance:** If a user requests a new confirmation email before the first token expires, should the old token be invalidated? The current design allows multiple active tokens for the same user and purpose. Invalidating old tokens is stricter but may frustrate users who clicked the first email late. Mastodon allows multiple active tokens. | Low — current design matches Mastodon behaviour; tighter policy is a one-line WHERE clause change |
| 3 | **Email token reaper schedule:** The `DeleteExpiredEmailTokens` query needs a trigger. Options: (a) call it at startup, (b) run it on a `time.Ticker` goroutine (e.g. every 6 hours), (c) defer to an external cron. Option (b) is simplest for self-hosters. | Low — additive; does not affect the interface or schema |
| 4 | **SMTP connection validation at startup:** The current SMTP `New` does not verify connectivity (unlike the Redis cache driver which pings on init). Should `New` dial the SMTP server and issue a `NOOP` command to fail fast? Trade-off: the SMTP server may be temporarily unavailable at startup but recover by the time the first email is sent. | Low — can be added behind a `SMTPVerifyOnInit` config flag |
| 5 | **Vendor-specific HTTP API drivers:** If a deployment environment blocks outbound port 587 (rare for Kubernetes/VPS, but possible on some serverless platforms), a vendor-specific HTTP API driver (SendGrid, Postmark, etc.) would be needed. The `Sender` interface supports adding these without any breaking changes. | Low — deferred; the SMTP driver covers all common deployment targets |

---

*End of ADR 05 — Email Abstraction*
