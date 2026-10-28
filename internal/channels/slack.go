package channels

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type Slack struct {
	botToken string
	appToken string
	owner    string
	api      *slack.Client
	dm       string
}

func NewSlack(botToken, appToken, owner string) *Slack {
	return &Slack{
		botToken: botToken,
		appToken: appToken,
		owner:    owner,
		api:      slack.New(botToken, slack.OptionAppLevelToken(appToken)),
	}
}

func (s *Slack) Name() string {
	return "slack"
}

func (s *Slack) Start(ctx context.Context, on func(Incoming)) error {
	if s.botToken == "" || s.appToken == "" || s.owner == "" {
		return errors.New("set SLACK_BOT_TOKEN, SLACK_APP_TOKEN and channels.slack.owner_user_id")
	}
	client := socketmode.New(s.api)
	go func() {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				client.Ack(*evt.Request)
				event, _ := evt.Data.(slackevents.EventsAPIEvent)
				msg, ok := event.InnerEvent.Data.(*slackevents.MessageEvent)
				if ok && msg.User == s.owner && msg.BotID == "" && msg.ChannelType == "im" {
					on(Incoming{Channel: "slack", Text: msg.Text})
				}
			case socketmode.EventTypeInteractive:
				client.Ack(*evt.Request)
				callback, _ := evt.Data.(slack.InteractionCallback)
				if callback.User.ID == s.owner && len(callback.ActionCallback.BlockActions) > 0 {
					on(Incoming{Channel: "slack", Text: callback.ActionCallback.BlockActions[0].Value})
				}
			}
		}
	}()
	return client.RunContext(ctx)
}

func (s *Slack) SendFile(ctx context.Context, path, caption string) error {
	if s.dm == "" {
		if err := s.Send(ctx, Update{Text: caption}); err != nil {
			return err
		}
	}
	_, err := s.api.UploadFileContext(ctx, slack.UploadFileParameters{
		File:           path,
		Filename:       filepath.Base(path),
		Title:          filepath.Base(path),
		InitialComment: caption,
		Channel:        s.dm,
	})
	return err
}

func (s *Slack) Send(ctx context.Context, u Update) error {
	if s.dm == "" {
		channel, _, _, err := s.api.OpenConversationContext(ctx, &slack.OpenConversationParameters{Users: []string{s.owner}})
		if err != nil {
			return err
		}
		s.dm = channel.ID
	}
	blocks := []slack.Block{slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", u.Text, false, false), nil, nil)}
	if len(u.Buttons) > 0 {
		var buttons []slack.BlockElement
		for i, b := range u.Buttons {
			buttons = append(buttons, slack.NewButtonBlockElement("kairo_"+string(rune('a'+i)), b.Data, slack.NewTextBlockObject("plain_text", b.Label, true, false)))
		}
		blocks = append(blocks, slack.NewActionBlock("", buttons...))
	}
	_, _, err := s.api.PostMessageContext(ctx, s.dm, slack.MsgOptionText(u.Text, false), slack.MsgOptionBlocks(blocks...))
	return err
}
