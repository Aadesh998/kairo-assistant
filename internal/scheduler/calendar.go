package scheduler

import (
	"context"
	"fmt"
	"time"

	"kairo/internal/channels"
	"kairo/internal/persona"
	"kairo/internal/store"
)

func (s *Scheduler) calendar(ctx context.Context, now time.Time) {
	cfg := s.app.Config.Calendar
	if !cfg.Enabled {
		return
	}
	events, err := s.app.Store.EventsBetween(now, now.Add(time.Hour))
	if err != nil {
		return
	}
	for _, e := range events {
		if e.AllDay || e.Start.Before(now.Add(-5*time.Minute)) {
			continue
		}
		if e.Link != "" && s.app.Store.HasJoinFor(e.Link) {
			continue
		}
		key := e.ID + "#" + e.Start.Format(time.RFC3339)
		if e.Link != "" && cfg.JoinMeetings {
			if !now.Before(e.Start.Add(-time.Duration(cfg.AskMinutesBefore)*time.Minute)) && !s.app.Store.RoutineRan("cal-join#"+key) {
				go s.meeting(ctx, meetingRoutine(e, cfg.AskBeforeJoin, cfg.WaitSeconds, cfg.IfNoAnswer))
			}
			continue
		}
		if !now.Before(e.Start.Add(-time.Duration(cfg.RemindMinutes)*time.Minute)) && !s.app.Store.RoutineRan("cal-remind#"+key) {
			s.remindEvent(ctx, e)
		}
	}
}

func (s *Scheduler) remindEvent(ctx context.Context, e store.Event) {
	minutes := int(time.Until(e.Start).Round(time.Minute).Minutes())
	text := fmt.Sprintf("📅 In %d min: %s (%s)", minutes, e.Title, e.Start.Format("15:04"))
	if e.Location != "" {
		text += "\n📍 " + e.Location
	}
	s.app.Store.Log("calendar", text)
	s.app.Notifier.Send(ctx, channels.Update{Text: text, Urgent: true})
	s.app.Announce(fmt.Sprintf("%s starts in %d minutes.", e.Title, minutes))
}

func meetingRoutine(e store.Event, ask bool, wait int, ifNoAnswer string) persona.Routine {
	return persona.Routine{
		At:          e.Start.Format("15:04"),
		Do:          "join_meeting",
		Title:       e.Title,
		MeetingURL:  e.Link,
		AskBefore:   ask,
		WaitSeconds: wait,
		IfNoAnswer:  ifNoAnswer,
	}
}
