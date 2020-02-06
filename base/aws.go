package base

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

func GetSession(region string) (sess *session.Session, err error) {
	if sess, err = session.NewSession(&aws.Config{
		Region: aws.String(region),
	}); err != nil {
		return nil, err
	}
	return sess, nil
}
