# Email Layer

Design doc: `docs/architecture/01-high-level-system-architecture.md`

## Conventions

- Implements the `email.Sender` interface with a single `Send(ctx, Message) error` method.
- Returns `email.ErrSendFailed{Provider, Err}` on delivery failure.
- Two drivers: `noop` (dev/testing) and `smtp` (production, covers SendGrid/Postmark/SES via SMTP relay).
- SMTP driver uses `github.com/jordan-wright/email` for MIME construction.
- Port 587 → STARTTLS, port 465 → implicit TLS, port 25 → plain SMTP.
- Templates live in `internal/email/templates/` and are embedded via `go:embed`.
- Per-message dial (no connection pooling) — volume is low for transactional email.
