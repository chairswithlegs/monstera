package service

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera-fed/internal/email"
)

// AccountApprovedData is the template data for account_approved emails.
type AccountApprovedData struct {
	InstanceName string
	Username     string
	InstanceURL  string
}

// RegistrationRejectedData is the template data for registration_rejected emails.
type RegistrationRejectedData struct {
	InstanceName string
	Username     string
	Reason       string
}

// RegistrationEmailSender sends account-approved and registration-rejected emails using the email package.
type RegistrationEmailSender struct {
	sender    email.Sender
	templates *email.Templates
	from      string
	fromName  string
}

// NewRegistrationEmailSender returns a sender that implements AccountApprovedMailer and RegistrationRejectedMailer.
func NewRegistrationEmailSender(sender email.Sender, templates *email.Templates, from, fromName string) *RegistrationEmailSender {
	return &RegistrationEmailSender{
		sender:    sender,
		templates: templates,
		from:      from,
		fromName:  fromName,
	}
}

// SendAccountApproved sends the account_approved email.
func (r *RegistrationEmailSender) SendAccountApproved(ctx context.Context, to, username, instanceName, instanceURL string) error {
	html, text, err := r.templates.Render("account_approved", AccountApprovedData{
		InstanceName: instanceName,
		Username:     username,
		InstanceURL:  instanceURL,
	})
	if err != nil {
		return fmt.Errorf("render account_approved: %w", err)
	}
	subject := fmt.Sprintf("Welcome to %s, @%s!", instanceName, username)
	msg := email.Message{
		To:      to,
		Subject: subject,
		HTML:    html,
		Text:    text,
		From:    r.from,
	}
	if err := r.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}

// SendRegistrationRejected sends the registration_rejected email.
func (r *RegistrationEmailSender) SendRegistrationRejected(ctx context.Context, to, username, instanceName, reason string) error {
	html, text, err := r.templates.Render("registration_rejected", RegistrationRejectedData{
		InstanceName: instanceName,
		Username:     username,
		Reason:       reason,
	})
	if err != nil {
		return fmt.Errorf("render registration_rejected: %w", err)
	}
	subject := "Your registration at " + instanceName
	msg := email.Message{
		To:      to,
		Subject: subject,
		HTML:    html,
		Text:    text,
		From:    r.from,
	}
	if err := r.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}

// Ensure RegistrationEmailSender implements the mailer interfaces.
var (
	_ AccountApprovedMailer      = (*RegistrationEmailSender)(nil)
	_ RegistrationRejectedMailer = (*RegistrationEmailSender)(nil)
)
