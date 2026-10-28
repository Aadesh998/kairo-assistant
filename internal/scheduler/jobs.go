package scheduler

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"kairo/internal/agent"
	"kairo/internal/channels"
	"kairo/internal/store"
	"kairo/internal/tools"
)

func (s *Scheduler) jobs(ctx context.Context, now time.Time) {
	due, err := s.app.Store.DueScheduled(now)
	if err != nil {
		return
	}
	for _, j := range due {
		s.app.Store.SetScheduledStatus(j.ID, "running")
		go s.runJob(ctx, j)
	}
}

func (s *Scheduler) runJob(parent context.Context, j store.Scheduled) {
	a := s.app
	ctx, cancel := context.WithCancel(parent)
	defer a.Track(cancel)()
	switch j.Kind {
	case "join":
		title := j.Title
		if title == "" {
			title = "the meeting"
		}
		joined, err := tools.JoinMeeting(ctx, a.Config.Browser, j.Payload)
		if err != nil {
			if ctx.Err() != nil {
				a.Store.SetScheduledStatus(j.ID, "cancelled")
				a.Store.Log("meeting", "join of "+title+" stopped by you")
				return
			}
			a.Store.SetScheduledStatus(j.ID, "failed")
			a.Store.Log("error", "scheduled join: "+err.Error())
			a.Notifier.Send(ctx, channels.Update{Text: fmt.Sprintf("❌ Could not join %s: %s\n%s", title, err, j.Payload), Urgent: true})
			exec.Command("notify-send", "-u", "critical", "-a", "Kairo", "❌ Could not join "+title, j.Payload).Run()
			a.Announce("I could not join " + title + ".")
			return
		}
		a.Store.SetScheduledStatus(j.ID, "done")
		message := fmt.Sprintf("✅ Joined %s (mic and camera off), waiting for you", title)
		if !joined {
			message = fmt.Sprintf("📅 %s is open in your browser with mic and camera off. Click Join now.", title)
		}
		a.Store.Log("meeting", message)
		a.Notifier.Send(ctx, channels.Update{Text: message, Urgent: !joined})
		exec.Command("notify-send", "-u", "critical", "-a", "Kairo", message).Run()
		a.Announce(message)
	case "instruction":
		a.Agent.Handle(ctx, agent.Input{
			Text:       j.Payload,
			Origin:     "schedule",
			Reason:     "you scheduled this for " + j.RunAt.Format("15:04"),
			Autonomous: true,
		})
		a.Store.SetScheduledStatus(j.ID, "done")
	}
}
