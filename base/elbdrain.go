package base

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb"
)

type Shutdowner interface {
	Shutdown() error
}

type ELBDrainPoller struct {
	*DebugOutput
	region  string
	elbName string
	auth    *session.Session

	delay      time.Duration
	shutdownCh chan struct{}
}

func NewELBDrainPoller(debug *DebugOutput, region string, elbName string) *ELBDrainPoller {
	return &ELBDrainPoller{
		DebugOutput: debug,
		region:      region,
		elbName:     elbName,
		shutdownCh:  make(chan struct{}),
		delay:       10 * time.Second,
	}
}

func (e *ELBDrainPoller) GetSession(region string) (sess *session.Session, err error) {
	if sess, err = session.NewSession(&aws.Config{
		Region: aws.String(region),
	}); err != nil {
		return nil, err
	}
	return sess, nil
}

func (e *ELBDrainPoller) getInstanceID() (string, error) {
	doc, err := ec2metadata.New(e.auth).GetInstanceIdentityDocument()
	if err != nil {
		return "", err
	}
	return doc.InstanceID, nil
}

func (e *ELBDrainPoller) checkForDraining(instanceID string) bool {
	cli := elb.New(e.auth)
	out, err := cli.DescribeInstanceHealth(&elb.DescribeInstanceHealthInput{
		Instances: []*elb.Instance{{
			InstanceId: aws.String(instanceID),
		}},
		LoadBalancerName: aws.String(e.elbName),
	})
	if err != nil {
		e.Debug("ELBDrainPoller: failed to poll ELB: %s", err)
		return false
	}
	if len(out.InstanceStates) == 0 {
		e.Debug("ELBDrainPoller: no instances in result")
		return false
	}
	instance := out.InstanceStates[0]
	state := "???"
	desc := "???"
	if instance != nil && instance.State != nil && instance.Description != nil {
		state = *instance.State
		desc = *instance.Description
	}
	e.Debug("ELBDrainPoller: instance state: %s desc: %s", state, desc)
	return state == "InService" &&
		desc == "Instance deregistration currently in progress."
}

func (e *ELBDrainPoller) WaitForDrain() (err error) {
	e.Debug("ELBDrainPoller: starting up")
	if e.auth, err = e.GetSession(e.region); err != nil {
		e.Debug("ELBDrainPoller: unable to get session: %s", err)
		return err
	}
	instanceID, err := e.getInstanceID()
	if err != nil {
		e.Debug("ELBDrainPoller: unable to get instance ID: %s", err)
		return err
	}
	for {
		select {
		case <-time.After(e.delay):
			if e.checkForDraining(instanceID) {
				e.Debug("ELBDrainPoller: draining state detected")
				return nil
			}
		case <-e.shutdownCh:
			return nil
		}
	}
}

func (e *ELBDrainPoller) Stop() {
	close(e.shutdownCh)
}
