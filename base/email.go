package base

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type Emailer interface {
	Send(address, subject, message string) error
}

type DummyEmailer struct{}

func (d DummyEmailer) Send(_, subject, _ string) error {
	fmt.Printf("subject: %s\n", subject)
	return nil
}

type SESEmailer struct {
	*DebugOutput
	sender string
	region string
	ses    *ses.Client
}

func NewSESEmailer(sender, region string, debugConfig *ChatDebugOutputConfig) *SESEmailer {
	return &SESEmailer{
		DebugOutput: NewDebugOutput("SESEmailer", debugConfig),
		sender:      sender,
		region:      region,
	}
}

func (e *SESEmailer) getClient() *ses.Client {
	if e.ses == nil {
		e.Debug("SESEmailer: getting SES client: region: %s", e.region)
		ctx := context.Background()
		cfg, err := GetAWSConfig(ctx, e.region)
		if err != nil {
			panic(fmt.Sprintf("unable to authenticate to AWS SES: %s", err.Error()))
		}
		e.ses = ses.NewFromConfig(cfg)
		e.Debug("SESEmailer: SES client created")
	}
	return e.ses
}

func (e *SESEmailer) Send(address, subject, message string) error {
	cli := e.getClient()
	ctx := context.Background()
	_, err := cli.SendEmail(ctx, &ses.SendEmailInput{
		Source: aws.String(e.sender),
		Destination: &types.Destination{
			ToAddresses: []string{address},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data: aws.String(subject),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data: aws.String(message),
				},
			},
		},
	})
	return err
}
