package controller

import (
	"context"
	smoothoperatorslack "github.com/pdok/smooth-operator/pkg/integrations/slack"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SlackSender interface {
	Send(message string, ctx context.Context)
}

type Slack struct {
	SlackUrl string
}

func (s *Slack) Send(message string, ctx context.Context) {
	lgr := log.FromContext(ctx)
	slackRequest := smoothoperatorslack.GetSimpleSlackErrorMessage(message)

	err := smoothoperatorslack.SendSlackRequest(slackRequest, s.SlackUrl)
	if err != nil {
		lgr.Error(err, "unable to send Slack Error message", "message", message)
	}
}
