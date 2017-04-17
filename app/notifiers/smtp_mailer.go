package notifiers

import (
	"bytes"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/go-errors/errors"
)

const (
	smtpPort = 587
	htmlMime = "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";"
)

type SMTPConfig struct {
	Host     string `yaml:"host"`
	UserName string `yaml:"user_name"`
	Password string `yaml:"password"`
}

type SMTPMailer struct {
	hostport string
	auth     smtp.Auth
}

func NewSMTPMailer(config *SMTPConfig) *SMTPMailer {
	auth := smtp.PlainAuth("", config.UserName, config.Password, config.Host)

	return &SMTPMailer{
		hostport: fmt.Sprintf("%s:%d", config.Host, smtpPort),
		auth:     auth,
	}
}

var _ Mailer = &SMTPMailer{}

func (s *SMTPMailer) SendHTML(from string, recipients, cc, bcc []string, subject string, body string) error {
	var smtpBody bytes.Buffer

	fmt.Fprintf(&smtpBody, "From: %s\n", from)
	fmt.Fprintf(&smtpBody, "To: %s\n", strings.Join(recipients, ", "))
	if len(cc) > 0 {
		fmt.Fprintf(&smtpBody, "Cc: %s\n", strings.Join(cc, ", "))
	}
	fmt.Fprintf(&smtpBody, "Subject: %s\n", subject)
	fmt.Fprintf(&smtpBody, "%s\n\n", htmlMime)
	smtpBody.WriteString(body)
	smtpBody.WriteString("\n")

	allRecipients := recipients
	if len(bcc) > 0 {
		allRecipients = append(allRecipients, bcc...)
	}

	if err := smtp.SendMail(s.hostport, s.auth, from, allRecipients, smtpBody.Bytes()); err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}
