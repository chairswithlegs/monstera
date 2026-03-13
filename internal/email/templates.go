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
	ExpiresIn    string
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
	Action       string
	Reason       string
	AppealURL    string
}

// Templates holds parsed HTML and text templates for all email types.
type Templates struct {
	html *htmltpl.Template
	text *texttpl.Template
}

// NewTemplates parses all embedded template files and returns a Templates instance.
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

// Render executes the named template with the given data.
// name is the template filename without extension (e.g. "email_verification").
func (t *Templates) Render(name string, data any) (htmlOut, textOut string, err error) {
	htmlName := name + ".html"
	textName := name + ".txt"
	var htmlBuf bytes.Buffer
	if err := t.html.ExecuteTemplate(&htmlBuf, htmlName, data); err != nil {
		return "", "", fmt.Errorf("email: render HTML template %q: %w", name, err)
	}
	var textBuf bytes.Buffer
	if err := t.text.ExecuteTemplate(&textBuf, textName, data); err != nil {
		return "", "", fmt.Errorf("email: render text template %q: %w", name, err)
	}
	return htmlBuf.String(), textBuf.String(), nil
}
