package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"kairo/internal/agent"
	"kairo/internal/app"
	"kairo/internal/channels"
	"kairo/internal/persona"
)

type Scheduler struct {
	app *app.App
}

func Run(ctx context.Context, a *app.App) {
	s := &Scheduler{app: a}
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		s.check(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func RunRoutine(ctx context.Context, a *app.App, r persona.Routine) {
	(&Scheduler{app: a}).runRoutine(ctx, r)
}

func (s *Scheduler) check(ctx context.Context, now time.Time) {
	for i, r := range s.app.Persona.Routine {
		key := now.Format("2006-01-02") + "#" + strconv.Itoa(i)
		if r.Due(now) && !s.app.Store.RoutineRan(key) {
			go s.runRoutine(ctx, r)
		}
	}
	s.reminders(ctx, now)
	s.jobs(ctx, now)
	s.calendar(ctx, now)
	if now.Format("15:04") == s.app.Config.Memory.CompactAt && !s.app.Store.RoutineRan("compact#"+now.Format("2006-01-02")) {
		go func() {
			if err := s.app.Memory.Compact(ctx); err != nil {
				s.app.Store.Log("error", "memory compact: "+err.Error())
			}
		}()
	}
}

func (s *Scheduler) runRoutine(ctx context.Context, r persona.Routine) {
	s.app.Store.Log("routine", r.At+" "+r.Label())
	switch r.Do {
	case "join_meeting":
		s.meeting(ctx, r)
	case "greet_and_brief":
		s.autonomous(ctx, r, s.app.Persona.Greeting+" Greet me and give me a short spoken brief for today: my calendar meetings, important mail and open tasks.")
	case "evening_summary":
		s.autonomous(ctx, r, "Give me a short evening summary: check recent_activity for what you did today and list what is still pending.")
	default:
		if r.Prompt != "" {
			s.autonomous(ctx, r, r.Prompt)
		}
	}
}

func (s *Scheduler) autonomous(ctx context.Context, r persona.Routine, prompt string) {
	reply := s.app.Agent.Handle(ctx, agent.Input{
		Text:       prompt,
		Origin:     "routine",
		Reason:     fmt.Sprintf("your routine at %s (%s)", r.At, r.Label()),
		Autonomous: true,
	})
	s.app.Announce(reply)
}

func (s *Scheduler) reminders(ctx context.Context, now time.Time) {
	due, err := s.app.Store.DueTasks(now)
	if err != nil {
		return
	}
	for _, t := range due {
		s.app.Store.MarkReminded(t.ID)
		s.app.Store.Log("reminder", t.Title)
		s.app.Notifier.Send(ctx, channels.Update{
			Text: fmt.Sprintf("⏰ Reminder: %s", t.Title),
			Buttons: []channels.Button{
				{Label: "✅ Done", Data: fmt.Sprintf("/done %d", t.ID)},
				{Label: "⏰ 10 min", Data: fmt.Sprintf("/snooze %d 10", t.ID)},
			},
			Urgent: true,
		})
		s.app.Announce("Reminder: " + t.Title)
	}
}
