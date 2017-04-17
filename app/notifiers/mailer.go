package notifiers

type MailerConfig struct {
	SMTP *SMTPConfig `yaml:"smtp"`
}

type Mailer interface {
	SendHTML(from string, recipients, cc, bcc []string, subject string, body string) error
}

func NewMailerFromConfig(config *MailerConfig) Mailer {
	if config != nil && config.SMTP != nil {
		return NewSMTPMailer(config.SMTP)
	}

	return NewSESMailer()
}
