package slack

import (
	"context"
	goslack "github.com/slack-go/slack"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const messagePrefix = "ogcapi-operator failed: "

type Sender interface {
	Send(message string, ctx context.Context)
}

type Slack struct {
	slackUrl string
}

func NewSlack(slackUrl string) *Slack {
	return &Slack{
		slackUrl: slackUrl,
	}
}

func (s *Slack) Send(text string, ctx context.Context) {
	lgr := log.FromContext(ctx)

	message := goslack.WebhookMessage{
		Text: messagePrefix + text,
	}
	err := goslack.PostWebhook(s.slackUrl, &message)

	if err != nil {
		lgr.Error(err, "unable to send Slack Error message", "message", message)
	}
}
