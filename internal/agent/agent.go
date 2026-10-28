package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"kairo/internal/approval"
	"kairo/internal/channels"
	"kairo/internal/config"
	"kairo/internal/learning"
	"kairo/internal/llm"
	"kairo/internal/memory"
	"kairo/internal/persona"
	"kairo/internal/policy"
	"kairo/internal/skills"
	"kairo/internal/store"
	"kairo/internal/tools"
)

type Input struct {
	Text       string
	Origin     string
	Reason     string
	Autonomous bool
}

type Agent struct {
	Config    *config.Config
	Persona   *persona.Persona
	Store     *store.Store
	LLM       *llm.Client
	Tools     *tools.Registry
	Approvals *approval.Broker
	Learner   *learning.Learner
	Notifier  *channels.Notifier
	Memory    *memory.Manager
	OnStep    func(text string)
	Skills    *skills.Library

	mu       sync.Mutex
	cancelMu sync.Mutex
	cancel   context.CancelFunc
	last     []string
}

type turn struct {
	actions []string
	auto    []string
}

func (a *Agent) Handle(ctx context.Context, in Input) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	a.cancelMu.Lock()
	a.cancel = cancel
	a.cancelMu.Unlock()

	a.Store.Log(in.Origin, "> "+in.Text)
	if !in.Autonomous {
		go a.Memory.Capture(context.WithoutCancel(ctx), in.Text, a.last)
	}

	messages := a.history(in.Origin)
	messages = append(messages, anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(in.Text)))
	t := &turn{}
	reply := ""
	for step := 0; step < a.Config.MaxSteps; step++ {
		resp, err := a.LLM.Send(ctx, a.stablePrompt(), a.volatilePrompt(in), messages, a.Tools.Specs())
		if err != nil {
			a.Store.Log("error", "claude: "+err.Error())
			reply = "Sorry, I could not reach my brain: " + err.Error()
			break
		}
		messages = append(messages, resp.ToParam())
		var results []anthropic.BetaContentBlockParamUnion
		for _, block := range resp.Content {
			if use, ok := block.AsAny().(anthropic.BetaToolUseBlock); ok {
				results = append(results, a.runTool(ctx, in, use, t))
			}
		}
		if resp.StopReason != anthropic.BetaStopReasonToolUse || len(results) == 0 {
			reply = llm.TextOf(resp)
			break
		}
		messages = append(messages, anthropic.NewBetaUserMessage(results...))
	}
	if reply == "" {
		reply = "Done."
	}
	a.Store.AddMessage(in.Origin, "user", in.Text)
	a.Store.AddMessage(in.Origin, "assistant", reply)
	a.Store.Log("reply", reply)
	a.last = t.actions
	voiceUpdate := in.Origin == "voice" && a.Config.Notify.VoiceCommands
	if in.Autonomous || len(t.auto) > 0 || voiceUpdate {
		a.sendUpdate(ctx, in, reply, t)
	}
	return reply
}

func (a *Agent) Stop() {
	a.cancelMu.Lock()
	defer a.cancelMu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *Agent) LastActions() []string {
	return a.last
}

func (a *Agent) history(thread string) []anthropic.BetaMessageParam {
	var list []anthropic.BetaMessageParam
	past, _ := a.Store.RecentMessages(thread, 10)
	for _, m := range past {
		block := anthropic.NewBetaTextBlock(m.Text)
		if m.Role == "user" {
			list = append(list, anthropic.NewBetaUserMessage(block))
		} else {
			list = append(list, anthropic.BetaMessageParam{Role: anthropic.BetaMessageParamRoleAssistant, Content: []anthropic.BetaContentBlockParamUnion{block}})
		}
	}
	if len(list) > 0 && list[0].Role != anthropic.BetaMessageParamRoleUser {
		list = list[1:]
	}
	return list
}

func (a *Agent) runTool(ctx context.Context, in Input, use anthropic.BetaToolUseBlock, t *turn) anthropic.BetaContentBlockParamUnion {
	var input map[string]any
	json.Unmarshal([]byte(use.JSON.Input.Raw()), &input)
	detail := describe(use.Name, input)
	tool, ok := a.Tools.Get(use.Name)
	if !ok {
		return toolResult(use.ID, "unknown tool", nil, true)
	}
	level, action := policy.Check(use.Name, input)
	switch level {
	case policy.Never:
		a.Store.Log("blocked", detail)
		return toolResult(use.ID, "Blocked by Kairo's safety policy. Do not try this again.", nil, true)
	case policy.Ask:
		if auto, score := a.Learner.Auto(action); auto {
			t.auto = append(t.auto, fmt.Sprintf("%s (learned from your approvals, score %.2f)", detail, score))
		} else {
			answer := a.Approvals.Ask(ctx, approval.Request{Action: action, Question: "May I " + detail + "?", Reason: in.Reason})
			if answer != approval.Yes {
				if answer == approval.No {
					a.Learner.Reward(action, -1, "denied")
				}
				a.Store.Log("denied", detail+" ("+answer+")")
				return toolResult(use.ID, "The owner did not approve this action ("+answer+"). Do not retry it.", nil, true)
			}
			a.Learner.Reward(action, 1, "approved")
		}
	}
	if a.OnStep != nil {
		a.OnStep(stepLabel(use.Name, input))
	}
	res, err := tool.Run(ctx, input)
	t.actions = append(t.actions, action)
	if err != nil {
		a.Store.Log("tool", detail+" failed: "+err.Error())
		return toolResult(use.ID, "error: "+err.Error(), nil, true)
	}
	a.Store.Log("tool", detail)
	return toolResult(use.ID, res.Text, res.Image, false)
}

func (a *Agent) sendUpdate(ctx context.Context, in Input, reply string, t *turn) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🤖 %s · %s\n%s", a.Config.Name, time.Now().Format("15:04"), reply))
	if in.Reason != "" {
		b.WriteString("\nWhy: " + in.Reason)
	}
	for _, note := range t.auto {
		b.WriteString("\nDone without asking: " + note)
	}
	a.Notifier.Send(ctx, channels.Update{
		Text:    b.String(),
		Buttons: []channels.Button{{Label: "👍 Good", Data: "/good"}, {Label: "👎 Don't do this", Data: "/bad"}},
	})
}

func stepLabel(name string, input map[string]any) string {
	arg := func(key string) string {
		v, _ := input[key].(string)
		if len(v) > 60 {
			v = v[:60] + "…"
		}
		return v
	}
	switch name {
	case "open_app":
		return "Opening " + arg("command") + "…"
	case "open_url", "browser_read":
		return "Opening " + arg("url") + "…"
	case "run_command":
		return "Running " + arg("command") + "…"
	case "screenshot":
		return "Looking at your screen…"
	case "list_windows", "focus_window", "switch_workspace":
		return "Checking your windows…"
	case "type_text", "press_keys":
		return "Typing…"
	case "join_meeting", "schedule_join":
		return "Setting up the meeting…"
	case "delegate_task":
		return "Handing to Claude Code: " + arg("task")
	case "calendar_events", "create_event":
		return "Checking your calendar…"
	case "recent_mail":
		return "Reading your mail…"
	case "add_task", "list_tasks", "complete_task":
		return "Updating your tasks…"
	case "remember", "recall":
		return "Checking my memory…"
	}
	return strings.ReplaceAll(name, "_", " ") + "…"
}

func describe(name string, input map[string]any) string {
	data, _ := json.Marshal(input)
	text := string(data)
	if len(text) > 200 {
		text = text[:200] + "..."
	}
	return name + " " + text
}

func toolResult(id, text string, image []byte, isError bool) anthropic.BetaContentBlockParamUnion {
	if strings.TrimSpace(text) == "" {
		text = "ok"
	}
	content := []anthropic.BetaToolResultBlockParamContentUnion{{OfText: &anthropic.BetaTextBlockParam{Text: text}}}
	if image != nil {
		content = append(content, anthropic.BetaToolResultBlockParamContentUnion{OfImage: &anthropic.BetaImageBlockParam{
			Source: anthropic.BetaImageBlockParamSourceUnion{OfBase64: &anthropic.BetaBase64ImageSourceParam{
				Data:      base64.StdEncoding.EncodeToString(image),
				MediaType: anthropic.BetaBase64ImageSourceMediaTypeImagePNG,
			}},
		}})
	}
	return anthropic.BetaContentBlockParamUnion{OfToolResult: &anthropic.BetaToolResultBlockParam{
		ToolUseID: id,
		Content:   content,
		IsError:   anthropic.Bool(isError),
	}}
}
