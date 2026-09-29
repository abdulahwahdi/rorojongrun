package channel

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"

	sharedpkg "monorepo/services/notification/pkg/shared"
)

// EmailSender sends mail over SMTP with STARTTLS. Works with any SMTP
// provider (Gmail, SES, Mailgun, a local dev catcher like Mailpit, ...) —
// deliberately not tied to a vendor SDK.
type EmailSender struct{}

// NewEmailSender constructor
func NewEmailSender() *EmailSender {
	return &EmailSender{}
}

// Send delivers an HTML email. subject is used as-is, body is sent as
// text/html.
func (s *EmailSender) Send(ctx context.Context, recipient, subject, body string) error {
	env := sharedpkg.GetEnv()
	addr := net.JoinHostPort(env.SMTPHost, fmt.Sprintf("%d", env.SMTPPort))

	msg := fmt.Sprintf(
		"From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		env.SMTPFromName, env.SMTPFromAddress, recipient, subject, body,
	)

	var auth smtp.Auth
	if env.SMTPUsername != "" {
		auth = smtp.PlainAuth("", env.SMTPUsername, env.SMTPPassword, env.SMTPHost)
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: env.SMTPHost}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(env.SMTPFromAddress); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return err
	}

	return client.Quit()
}
