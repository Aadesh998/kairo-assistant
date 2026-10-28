package calendar

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"kairo/internal/config"
	"kairo/internal/google"
	"kairo/internal/store"
)

const syncAhead = 48 * time.Hour

var meetingLink = regexp.MustCompile(`https://(meet\.google\.com|[a-z0-9.-]*zoom\.us|teams\.microsoft\.com)/[^\s"<>]+`)

type Client struct {
	Config config.Calendar
	Store  *store.Store
}

func (c *Client) service(ctx context.Context) (*gcal.Service, error) {
	ts, err := google.TokenSource(ctx)
	if err != nil {
		return nil, err
	}
	return gcal.NewService(ctx, option.WithTokenSource(ts))
}

func (c *Client) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := c.Sync(ctx); err != nil && ctx.Err() == nil {
			c.Store.Log("error", "calendar: "+hint(err))
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Minute):
		}
	}
}

func (c *Client) Sync(ctx context.Context) error {
	srv, err := c.service(ctx)
	if err != nil {
		return err
	}
	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(syncAhead)
	resp, err := srv.Events.List(c.Config.CalendarID).
		SingleEvents(true).
		OrderBy("startTime").
		TimeMin(from.Format(time.RFC3339)).
		TimeMax(to.Format(time.RFC3339)).
		MaxResults(100).
		Context(ctx).
		Do()
	if err != nil {
		return err
	}
	var list []store.Event
	for _, item := range resp.Items {
		if item.Status == "cancelled" || declined(item) {
			continue
		}
		list = append(list, toEvent(item))
	}
	return c.Store.ReplaceEvents(from, to, list)
}

func (c *Client) Create(ctx context.Context, title string, start time.Time, minutes int, attendees []string) (string, error) {
	srv, err := c.service(ctx)
	if err != nil {
		return "", err
	}
	event := &gcal.Event{
		Summary: title,
		Start:   &gcal.EventDateTime{DateTime: start.Format(time.RFC3339)},
		End:     &gcal.EventDateTime{DateTime: start.Add(time.Duration(minutes) * time.Minute).Format(time.RFC3339)},
	}
	for _, a := range attendees {
		if a = strings.TrimSpace(a); a != "" {
			event.Attendees = append(event.Attendees, &gcal.EventAttendee{Email: a})
		}
	}
	created, err := srv.Events.Insert(c.Config.CalendarID, event).Context(ctx).Do()
	if err != nil {
		return "", errors.New(hint(err))
	}
	c.Sync(ctx)
	return created.HtmlLink, nil
}

func toEvent(item *gcal.Event) store.Event {
	e := store.Event{ID: item.Id, Title: item.Summary, Location: item.Location}
	if e.Title == "" {
		e.Title = "(no title)"
	}
	e.Start, e.AllDay = parseWhen(item.Start)
	e.End, _ = parseWhen(item.End)
	e.Link = item.HangoutLink
	if e.Link == "" && item.ConferenceData != nil {
		for _, p := range item.ConferenceData.EntryPoints {
			if p.EntryPointType == "video" {
				e.Link = p.Uri
				break
			}
		}
	}
	if e.Link == "" {
		e.Link = meetingLink.FindString(item.Location + " " + item.Description)
	}
	return e
}

func parseWhen(w *gcal.EventDateTime) (time.Time, bool) {
	if w == nil {
		return time.Time{}, false
	}
	if w.DateTime != "" {
		t, _ := time.Parse(time.RFC3339, w.DateTime)
		return t.Local(), false
	}
	t, _ := time.ParseInLocation("2006-01-02", w.Date, time.Local)
	return t, true
}

func declined(item *gcal.Event) bool {
	for _, a := range item.Attendees {
		if a.Self && a.ResponseStatus == "declined" {
			return true
		}
	}
	return false
}

func hint(err error) string {
	text := err.Error()
	if strings.Contains(text, "insufficient") || strings.Contains(text, "403") {
		text += " (run kairo google login again to allow Calendar access)"
	}
	return text
}
