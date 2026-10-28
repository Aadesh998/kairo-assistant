package daemon

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"kairo/internal/agent"
	"kairo/internal/app"
	"kairo/internal/approval"
	"kairo/internal/channels"
	"kairo/internal/tools"
)

func handleIncoming(ctx context.Context, a *app.App, in channels.Incoming) {
	text := strings.TrimSpace(in.Text)
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return
	}
	reply := func(msg string) {
		a.Notifier.SendTo(ctx, in.Channel, channels.Update{Text: msg, Urgent: true})
	}
	arg := func(i int) string {
		if len(fields) > i {
			return fields[i]
		}
		return ""
	}
	switch fields[0] {
	case "/approve", "/deny", "/later":
		answers := map[string]string{"/approve": approval.Yes, "/deny": approval.No, "/later": approval.Later}
		if !a.Approvals.Resolve(arg(1), answers[fields[0]]) {
			reply("That approval is no longer waiting: it timed out, or Kairo restarted. Ask me again.")
		}
	case "/stop":
		a.StopAll()
		reply("Stopped everything I was doing.")
	case "/good":
		for _, action := range a.Agent.LastActions() {
			a.Learner.Reward(action, 1, "owner liked")
		}
		reply("Thanks, noted 👍")
	case "/bad":
		for _, action := range a.Agent.LastActions() {
			a.Learner.Reward(action, -2, "owner disliked")
		}
		reply("Noted. I will ask you before doing that again.")
	case "/unlearn":
		action := strings.TrimSpace(strings.TrimPrefix(text, "/unlearn"))
		a.Learner.Reward(action, -2, "owner unlearned")
		reply("Okay, I will ask first next time.")
	case "/done":
		id, _ := strconv.ParseInt(arg(1), 10, 64)
		a.Store.CompleteTask(id)
		reply("✅ Marked done.")
	case "/snooze":
		id, _ := strconv.ParseInt(arg(1), 10, 64)
		minutes, _ := strconv.Atoi(arg(2))
		if minutes == 0 {
			minutes = 10
		}
		a.Store.SnoozeTask(id, time.Now().Add(time.Duration(minutes)*time.Minute))
		reply(fmt.Sprintf("⏰ I will remind you again in %d minutes.", minutes))
	case "/tasks":
		tasks, _ := a.Store.OpenTasks()
		var lines []string
		for _, t := range tasks {
			lines = append(lines, fmt.Sprintf("#%d %s %s", t.ID, t.Title, tools.DueText(t.DueAt)))
		}
		if len(lines) == 0 {
			lines = []string{"No open tasks."}
		}
		reply(strings.Join(lines, "\n"))
	case "/wake":
		if a.Voice != nil {
			a.Voice.Wake()
		}
	case "/start", "/help":
		reply("Talk to me normally, or use /stop /good /bad /tasks /done <id> /snooze <id> <min> /wake")
	default:
		if answer := approval.ParseAnswer(text); answer != "" && len(fields) <= 3 && a.Approvals.ResolveOnly(answer) {
			return
		}
		go func() {
			answer := a.Agent.Handle(ctx, agent.Input{Text: text, Origin: in.Channel})
			reply(answer)
		}()
	}
}
