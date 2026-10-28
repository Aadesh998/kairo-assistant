package mail

import (
	"context"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	_ "github.com/emersion/go-message/charset"
	gomail "github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"

	"kairo/internal/config"
	"kairo/internal/google"
	"kairo/internal/store"
)

type Email struct {
	UID     uint32
	From    string
	Subject string
	Date    time.Time
	Text    string
}

type Watcher struct {
	Config config.Mail
	Store  *store.Store
	Triage *Triage
}

var htmlTags = regexp.MustCompile(`<[^>]*>`)

func (w *Watcher) Run(ctx context.Context) {
	wait := 30 * time.Second
	for ctx.Err() == nil {
		started := time.Now()
		err := w.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			w.Store.Log("error", "mail: "+err.Error())
		}
		if time.Since(started) > 5*time.Minute {
			wait = 30 * time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, 10*time.Minute)
	}
}

func (w *Watcher) connect(ctx context.Context) (*client.Client, *imap.MailboxStatus, error) {
	ts, err := google.TokenSource(ctx)
	if err != nil {
		return nil, nil, err
	}
	token, err := ts.Token()
	if err != nil {
		return nil, nil, err
	}
	c, err := client.DialTLS(w.Config.Server, nil)
	if err != nil {
		return nil, nil, err
	}
	host, portText, _ := net.SplitHostPort(w.Config.Server)
	port, _ := strconv.Atoi(portText)
	auth := sasl.NewOAuthBearerClient(&sasl.OAuthBearerOptions{Username: w.Config.Address, Token: token.AccessToken, Host: host, Port: port})
	if err := c.Authenticate(auth); err != nil {
		c.Logout()
		return nil, nil, err
	}
	status, err := c.Select(w.Config.Mailbox, true)
	if err != nil {
		c.Logout()
		return nil, nil, err
	}
	return c, status, nil
}

func (w *Watcher) session(ctx context.Context) error {
	c, status, err := w.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Logout()
	key := "mail.last_uid." + w.Config.Mailbox
	last, _ := strconv.ParseUint(w.Store.Get(key), 10, 32)
	if last == 0 {
		last = uint64(status.UidNext) - 1
		w.Store.Set(key, strconv.FormatUint(last, 10))
	}
	w.Store.Log("mail", "watching "+w.Config.Address)
	started := time.Now()
	for ctx.Err() == nil && time.Since(started) < 45*time.Minute {
		emails, err := fetch(c, uint32(last)+1, 0)
		if err != nil {
			return err
		}
		for _, e := range emails {
			if uint64(e.UID) <= last {
				continue
			}
			w.Triage.Handle(ctx, w.Config.Mailbox, e)
			last = uint64(e.UID)
			w.Store.Set(key, strconv.FormatUint(last, 10))
		}
		if err := idle(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func idle(ctx context.Context, c *client.Client) error {
	updates := make(chan client.Update, 16)
	c.Updates = updates
	defer func() { c.Updates = nil }()
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- c.Idle(stop, nil) }()
	select {
	case <-updates:
	case err := <-done:
		return err
	case <-time.After(9 * time.Minute):
	case <-ctx.Done():
	}
	close(stop)
	return <-done
}

func (w *Watcher) Latest(ctx context.Context, n int) ([]Email, error) {
	c, status, err := w.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Logout()
	from := uint32(1)
	if status.UidNext > uint32(n) {
		from = status.UidNext - uint32(n)
	}
	return fetch(c, from, 0)
}

func fetch(c *client.Client, from, to uint32) ([]Email, error) {
	set := new(imap.SeqSet)
	set.AddRange(from, to)
	section := &imap.BodySectionName{Peek: true, Partial: []int{0, 200000}}
	messages := make(chan *imap.Message, 32)
	done := make(chan error, 1)
	go func() {
		done <- c.UidFetch(set, []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, section.FetchItem()}, messages)
	}()
	var list []Email
	for msg := range messages {
		e := Email{UID: msg.Uid}
		if env := msg.Envelope; env != nil {
			e.Subject = env.Subject
			e.Date = env.Date
			if len(env.From) > 0 {
				e.From = strings.TrimSpace(env.From[0].PersonalName + " <" + env.From[0].Address() + ">")
			}
		}
		if body := msg.GetBody(section); body != nil {
			e.Text = readText(body)
		}
		list = append(list, e)
	}
	return list, <-done
}

func readText(r io.Reader) string {
	reader, err := gomail.CreateReader(r)
	if err != nil {
		return ""
	}
	var plain, html string
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		header, ok := part.Header.(*gomail.InlineHeader)
		if !ok {
			continue
		}
		kind, _, _ := header.ContentType()
		data, _ := io.ReadAll(io.LimitReader(part.Body, 20000))
		switch kind {
		case "text/plain":
			plain += string(data)
		case "text/html":
			html += htmlTags.ReplaceAllString(string(data), " ")
		}
	}
	text := plain
	if strings.TrimSpace(text) == "" {
		text = html
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 4000 {
		text = text[:4000]
	}
	return text
}
