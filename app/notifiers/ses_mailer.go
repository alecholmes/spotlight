package notifiers

import (
	"github.com/aws/aws-sdk-go/aws"
	aws_session "github.com/aws/aws-sdk-go/aws/session"
	aws_ses "github.com/aws/aws-sdk-go/service/ses"
	"github.com/go-errors/errors"
)

type SESMailer struct {
	client *aws_ses.SES
}

var _ Mailer = &SESMailer{}

func NewSESMailer() *SESMailer {
	sess := aws_session.Must(aws_session.NewSessionWithOptions(aws_session.Options{
		Config: aws.Config{Region: aws.String("us-west-2")}, // TODO: config
	}))

	return &SESMailer{
		client: aws_ses.New(sess),
	}
}

func (s *SESMailer) SendHTML(from string, recipients, cc, bcc []string, subject string, body string) error {
	request := &aws_ses.SendEmailInput{
		Source: &from,
		Destination: &aws_ses.Destination{
			ToAddresses:  s.stringPointers(recipients),
			CcAddresses:  s.stringPointers(cc),
			BccAddresses: s.stringPointers(bcc),
		},
		Message: &aws_ses.Message{
			Subject: &aws_ses.Content{Data: &subject},
			Body: &aws_ses.Body{
				Html: &aws_ses.Content{Data: &body},
			},
		},
	}

	if _, err := s.client.SendEmail(request); err != nil {
		return errors.WrapPrefix(err, "Error sending email", 0)
	}

	return nil
}

func (s *SESMailer) stringPointers(strs []string) []*string {
	if len(strs) == 0 {
		return nil
	}

	ptrs := make([]*string, len(strs))
	for i, str := range strs {
		ptrs[i] = &str
	}

	return ptrs
}
