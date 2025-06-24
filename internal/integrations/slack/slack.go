package slack

import (
	"context"

	goslack "github.com/slack-go/slack"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const messagePrefix = "ogcapi-operator failed: "

type Sender interface {
	Send(ctx context.Context, message string)
}

type Slack struct {
	slackURL string
}

func NewSlack(slackURL string) *Slack {
	return &Slack{
		slackURL: slackURL,
	}
}

func (s *Slack) Send(ctx context.Context, text string) {
	lgr := log.FromContext(ctx)
	if s.slackURL == "" {
		lgr.Info("no Slack URL configured, skipping Slack message")
	}

	message := goslack.WebhookMessage{
		Text: messagePrefix + text,
	}
	err := goslack.PostWebhook(s.slackURL, &message)

	if err != nil {
		lgr.Error(err, "unable to send Slack Error message", "message", message)
	}
}
