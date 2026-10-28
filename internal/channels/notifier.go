package channels

import (
	"context"
	"errors"
	"os"
	"time"

	"kairo/internal/config"
)

type Notifier struct {
	list []Channel
	cfg  config.Notify
	log  func(kind, text string)
}

func NewNotifier(cfg *config.Config, log func(kind, text string)) *Notifier {
	n := &Notifier{cfg: cfg.Notify, log: log}
	c := cfg.Channels
	if c.Telegram.Enabled {
		n.list = append(n.list, NewTelegram(os.Getenv("TELEGRAM_BOT_TOKEN"), c.Telegram.OwnerChatID))
	}
	if c.Slack.Enabled {
		n.list = append(n.list, NewSlack(os.Getenv("SLACK_BOT_TOKEN"), os.Getenv("SLACK_APP_TOKEN"), c.Slack.OwnerUserID))
	}
	if c.WhatsApp.Enabled {
		n.list = append(n.list, NewWhatsApp(c.WhatsApp.OwnerNumber))
	}
	return n
}

func (n *Notifier) Names() []string {
	var names []string
	for _, c := range n.list {
		names = append(names, c.Name())
	}
	return names
}

func (n *Notifier) Start(ctx context.Context, on func(Incoming)) {
	for _, c := range n.list {
		go func(c Channel) {
			for ctx.Err() == nil {
				if err := c.Start(ctx, on); err != nil && ctx.Err() == nil {
					n.log("error", c.Name()+": "+err.Error())
				}
				select {
				case <-ctx.Done():
				case <-time.After(15 * time.Second):
				}
			}
		}(c)
	}
}

func (n *Notifier) Send(ctx context.Context, u Update) {
	if !u.Urgent && quiet(n.cfg.QuietFrom, n.cfg.QuietTo, time.Now()) {
		n.log("notify", "held (quiet hours): "+u.Text)
		return
	}
	for _, c := range n.list {
		if n.cfg.SendTo == "primary" && c.Name() != n.cfg.Primary {
			continue
		}
		n.deliver(ctx, c, u)
	}
}

func (n *Notifier) SendTo(ctx context.Context, name string, u Update) {
	for _, c := range n.list {
		if c.Name() == name {
			n.deliver(ctx, c, u)
		}
	}
}

func (n *Notifier) SendFile(ctx context.Context, path, caption string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	var last error = errors.New("no channel is enabled")
	for _, c := range n.list {
		if n.cfg.SendTo == "primary" && c.Name() != n.cfg.Primary {
			continue
		}
		if err := c.SendFile(ctx, path, caption); err != nil {
			n.log("error", "send file "+c.Name()+": "+err.Error())
			last = err
			continue
		}
		n.log("notify", c.Name()+": file "+path)
		return nil
	}
	return last
}

func (n *Notifier) deliver(ctx context.Context, c Channel, u Update) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := c.Send(ctx, u); err != nil {
		n.log("error", "send "+c.Name()+": "+err.Error())
		return
	}
	n.log("notify", c.Name()+": "+firstLine(u.Text))
}

func quiet(from, to string, now time.Time) bool {
	if from == "" || to == "" {
		return false
	}
	clock := now.Format("15:04")
	if from < to {
		return clock >= from && clock < to
	}
	return clock >= from || clock < to
}

func firstLine(text string) string {
	for i, r := range text {
		if r == '\n' {
			return text[:i]
		}
	}
	return text
}
