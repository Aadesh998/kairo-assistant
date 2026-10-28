package scheduler

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"kairo/internal/approval"
	"kairo/internal/channels"
	"kairo/internal/persona"
	"kairo/internal/tools"
)

func (s *Scheduler) meeting(parent context.Context, r persona.Routine) {
	a := s.app
	ctx, cancel := context.WithCancel(parent)
	defer a.Track(cancel)()
	action := "join_meeting:" + r.Label()
	why := fmt.Sprintf("your routine says %s at %s", r.Label(), r.At)
	answer := approval.Yes
	if r.AskBefore {
		if auto, score := a.Learner.Auto(action); auto {
			why += fmt.Sprintf("; you approved this many times (score %.2f)", score)
		} else {
			answer = s.askJoin(ctx, r, action)
			if answer == approval.Later {
				time.Sleep(3 * time.Minute)
				answer = s.askJoin(ctx, r, action)
			}
			switch answer {
			case approval.Yes:
				a.Learner.Reward(action, 1, "approved meeting")
			case approval.No:
				a.Learner.Reward(action, -1, "skipped meeting")
			}
		}
	}
	if answer == approval.Timeout {
		switch r.IfNoAnswer {
		case "join_muted":
			answer = approval.Yes
			why += "; you did not answer, so I joined muted"
		case "notify_only":
			a.Notifier.Send(ctx, channels.Update{Text: "📅 " + r.Label() + " is starting: " + r.MeetingURL, Urgent: true})
			return
		}
	}
	if answer != approval.Yes {
		a.Store.Log("meeting", "skipped "+r.Label()+" ("+answer+")")
		a.Notifier.Send(ctx, channels.Update{Text: "⏭ Skipped " + r.Label()})
		return
	}
	joined, err := tools.JoinMeeting(ctx, a.Config.Browser, r.MeetingURL)
	if err != nil {
		if ctx.Err() != nil {
			a.Store.Log("meeting", "join of "+r.Label()+" stopped by you")
			return
		}
		a.Store.Log("error", "join meeting: "+err.Error())
		a.Notifier.Send(ctx, channels.Update{Text: "❌ Could not join " + r.Label() + ": " + err.Error() + "\n" + r.MeetingURL, Urgent: true})
		a.Announce("I could not join " + r.Label() + ". I sent you the link.")
		return
	}
	if !joined {
		message := fmt.Sprintf("📅 %s is open in your browser with mic and camera off. Click Join now.", r.Label())
		a.Store.Log("meeting", message)
		a.Notifier.Send(ctx, channels.Update{Text: message, Urgent: true})
		exec.Command("notify-send", "-u", "critical", "-a", "Kairo", message).Run()
		a.Announce(r.Label() + " is open. Click join when you are ready.")
		return
	}
	a.Store.Log("meeting", "joined "+r.Label())
	a.Notifier.Send(ctx, channels.Update{
		Text:    fmt.Sprintf("✅ Joined %s (mic and camera off)\nWhy: %s", r.Label(), why),
		Buttons: []channels.Button{{Label: "👎 Don't auto-join", Data: "/unlearn " + action}},
	})
	a.Announce("You are in the " + r.Label() + ".")
}

func (s *Scheduler) askJoin(ctx context.Context, r persona.Routine, action string) string {
	wait := time.Duration(r.WaitSeconds) * time.Second
	if wait == 0 {
		wait = time.Minute
	}
	return s.app.Approvals.Ask(ctx, approval.Request{
		Action:   action,
		Question: fmt.Sprintf("%s starts at %s. Shall I join it for you?", r.Label(), r.At),
		Reason:   "daily routine",
		Wait:     wait,
	})
}
