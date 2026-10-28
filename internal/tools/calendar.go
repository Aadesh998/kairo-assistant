package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"kairo/internal/calendar"
	"kairo/internal/store"
)

func calendarTools(s *store.Store, c *calendar.Client) []Tool {
	return []Tool{
		{
			Name:        "calendar_events",
			Description: "List the owner's Google Calendar events for the next days.",
			Params:      map[string]any{"days": prop("integer", "How many days ahead, default 1")},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				days := num(in, "days")
				if days < 1 {
					days = 1
				}
				now := time.Now()
				start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
				list, err := s.EventsBetween(start, start.AddDate(0, 0, int(days)))
				if err != nil {
					return Result{}, err
				}
				return Result{Text: FormatEvents(list)}, nil
			},
		},
		{
			Name:        "create_event",
			Description: "Create an event in the owner's Google Calendar, optionally inviting people by email.",
			Params: map[string]any{
				"title":     prop("string", "Event title"),
				"start":     prop("string", "Start as 'YYYY-MM-DD HH:MM' in local time"),
				"minutes":   prop("integer", "Duration in minutes, default 30"),
				"attendees": prop("string", "Comma separated emails to invite, empty for none"),
			},
			Required: []string{"title", "start"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				if c == nil {
					return Result{}, errors.New("calendar is not set up")
				}
				start, err := time.ParseInLocation("2006-01-02 15:04", str(in, "start"), time.Local)
				if err != nil {
					return Result{}, err
				}
				minutes := int(num(in, "minutes"))
				if minutes <= 0 {
					minutes = 30
				}
				link, err := c.Create(ctx, str(in, "title"), start, minutes, strings.Split(str(in, "attendees"), ","))
				return Result{Text: "created " + link}, err
			},
		},
	}
}

func FormatEvents(list []store.Event) string {
	if len(list) == 0 {
		return "no events"
	}
	var lines []string
	for _, e := range list {
		when := e.Start.Format("Mon 02 Jan 15:04") + "-" + e.End.Format("15:04")
		if e.AllDay {
			when = e.Start.Format("Mon 02 Jan") + " all day"
		}
		line := fmt.Sprintf("%s  %s", when, e.Title)
		if e.Link != "" {
			line += "  (" + e.Link + ")"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
