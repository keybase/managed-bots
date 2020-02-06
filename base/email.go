package base

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
)

type Emailer interface {
	Send(address, subject, message string) error
}

type DummyEmailer struct {
}

func (d DummyEmailer) Send(address, subject, message string) error {
	fmt.Printf("subject: %s\n", subject)
	fmt.Println(message)
	return nil
}

type SESEmailer struct {
	*DebugOutput
	sender string
	region string
	ses    *ses.SES
}

func NewSESEmailer(sender, region string, debugConfig *ChatDebugOutputConfig) *SESEmailer {
	return &SESEmailer{
		DebugOutput: NewDebugOutput("SESEmailer", debugConfig),
		sender:      sender,
		region:      region,
	}
}

func (e *SESEmailer) getClient() *ses.SES {
	var err error
	if e.ses == nil {
		e.Debug("SESEmailer: getting SES client: region: %s", e.region)
		var auth *session.Session
		if auth, err = GetSession(e.region); err != nil {
			panic(fmt.Sprintf("unable to authenticate to AWS SES: %s", err.Error()))
		}
		e.ses = ses.New(auth)
		e.Debug("SESEmailer: SES client created")
	}
	return e.ses
}

func (e *SESEmailer) Send(address, subject, message string) error {
	cli := e.getClient()
	_, err := cli.SendEmail(&ses.SendEmailInput{
		Source: aws.String(e.sender),
		Destination: &ses.Destination{
			ToAddresses: aws.StringSlice([]string{address}),
		},
		Message: &ses.Message{
			Subject: &ses.Content{
				Data: aws.String(subject),
			},
			Body: &ses.Body{
				Html: &ses.Content{
					Data: aws.String(message),
				},
			},
		},
	})
	return err
}
