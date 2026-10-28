package channels

import "context"

type Incoming struct {
	Channel string
	Text    string
}

type Button struct {
	Label string
	Data  string
}

type Update struct {
	Text    string
	Buttons []Button
	Urgent  bool
}

type Channel interface {
	Name() string
	Start(ctx context.Context, on func(Incoming)) error
	Send(ctx context.Context, u Update) error
	SendFile(ctx context.Context, path, caption string) error
}
