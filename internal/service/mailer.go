package service

import (
	"fmt"
	"log/slog"
	"net/smtp"
)

// Mailer sends emails.
type Mailer interface {
	SendMagicLink(to, link string) error
}

// LogMailer logs emails to stdout (dev mode).
type LogMailer struct {
	logger *slog.Logger
}

// NewLogMailer creates a mailer that logs to stdout instead of sending email.
func NewLogMailer(logger *slog.Logger) *LogMailer {
	return &LogMailer{logger: logger}
}

func (m *LogMailer) SendMagicLink(to, link string) error {
	m.logger.Info("magic link generated", "to", to, "link", link)
	return nil
}

// SMTPMailer sends emails via SMTP.
type SMTPMailer struct {
	host     string
	port     string
	username string
	password string
	from     string
}

// NewSMTPMailer creates a mailer that sends via SMTP.
func NewSMTPMailer(host, port, username, password, from string) *SMTPMailer {
	return &SMTPMailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

func (m *SMTPMailer) SendMagicLink(to, link string) error {
	subject := "Your Claude Cloud login link"
	body := fmt.Sprintf("Click to log in:\n\n%s\n\nThis link expires in 15 minutes.", link)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", m.from, to, subject, body)

	auth := smtp.PlainAuth("", m.username, m.password, m.host)
	addr := m.host + ":" + m.port
	return smtp.SendMail(addr, auth, m.from, []string{to}, []byte(msg))
}
