package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"kairo/internal/agent"
	"kairo/internal/approval"
	"kairo/internal/calendar"
	"kairo/internal/channels"
	"kairo/internal/chat"
	"kairo/internal/config"
	"kairo/internal/google"
	"kairo/internal/jev"
	"kairo/internal/learning"
	"kairo/internal/llm"
	"kairo/internal/mail"
	"kairo/internal/memory"
	"kairo/internal/persona"
	"kairo/internal/policy"
	"kairo/internal/skills"
	"kairo/internal/store"
	"kairo/internal/tools"
	"kairo/internal/voice"
)

type App struct {
	Config    *config.Config
	Persona   *persona.Persona
	Store     *store.Store
	Learner   *learning.Learner
	Approvals *approval.Broker
	Notifier  *channels.Notifier
	Agent     *agent.Agent
	Voice     *voice.Session
	Memory    *memory.Manager
	Mail      *mail.Watcher
	Calendar  *calendar.Client
	Events    *chat.Hub

	cardMu sync.Mutex
	card   *approval.Request

	jobsMu sync.Mutex
	jobID  int
	jobs   map[int]context.CancelFunc
}

func Build(withVoice bool) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	p, err := persona.Load()
	if err != nil {
		return nil, err
	}
	s, err := store.Open()
	if err != nil {
		return nil, err
	}
	a := &App{Config: cfg, Persona: p, Store: s, Events: &chat.Hub{}}
	a.Notifier = channels.NewNotifier(cfg, s.Log)
	a.Learner = learning.New(s, cfg.Learning)
	a.Approvals = approval.NewBroker(time.Duration(cfg.Notify.ApprovalSeconds) * time.Second)
	a.Approvals.Send = a.sendApproval
	a.Approvals.Card = a.showCard
	a.Calendar = &calendar.Client{Config: cfg.Calendar, Store: s}
	policy.Protect(cfg.Guardrails.ProtectedPaths)
	library := skills.Load(cfg.Skills)
	registry := tools.New(tools.Deps{Config: cfg, Store: s, Calendar: a.Calendar, Notify: a.Update, StopJobs: a.StopJobs, Skills: library, SendFile: a.SendFile})
	brain := llm.New(cfg.Model, cfg.Effort)
	fast := jev.New(cfg.Jev.Model)
	a.Memory = &memory.Manager{Config: cfg, Store: s, LLM: brain, Jev: fast, Learner: a.Learner}
	a.Agent = &agent.Agent{
		Config:    cfg,
		Persona:   p,
		Store:     s,
		LLM:       brain,
		Tools:     registry,
		Approvals: a.Approvals,
		Learner:   a.Learner,
		Notifier:  a.Notifier,
		Memory:    a.Memory,
		Skills:    library,
		OnStep:    a.showStep,
	}
	a.Mail = &mail.Watcher{
		Config: cfg.Mail,
		Store:  s,
		Triage: &mail.Triage{Store: s, LLM: brain, Jev: fast, Threshold: cfg.Jev.Threshold, Notify: a.mailUpdate, Say: a.sayNow},
	}
	if withVoice && cfg.Voice.Enabled {
		a.Voice = voice.NewSession(cfg, a.voiceHandler, s.Log)
	}
	return a, nil
}

func (a *App) Track(cancel context.CancelFunc) func() {
	a.jobsMu.Lock()
	defer a.jobsMu.Unlock()
	if a.jobs == nil {
		a.jobs = map[int]context.CancelFunc{}
	}
	a.jobID++
	id := a.jobID
	a.jobs[id] = cancel
	return func() {
		a.jobsMu.Lock()
		delete(a.jobs, id)
		a.jobsMu.Unlock()
		cancel()
	}
}

func (a *App) SendFile(ctx context.Context, path, caption string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a folder, send a file", path)
	}
	if info.Size() <= 50<<20 {
		if err := a.Notifier.SendFile(ctx, path, caption); err != nil {
			return "", err
		}
		return fmt.Sprintf("sent %s (%d MB) to the owner's phone", filepath.Base(path), info.Size()>>20), nil
	}
	a.Store.Log("tool", fmt.Sprintf("uploading %s (%d MB) to Google Drive", filepath.Base(path), info.Size()>>20))
	link, err := google.UploadToDrive(ctx, path)
	if err != nil {
		return "", err
	}
	text := fmt.Sprintf("📎 %s (%d MB, too big for Telegram)\nOpen it from your Google Drive: %s", filepath.Base(path), info.Size()>>20, link)
	if caption != "" {
		text = caption + "\n" + text
	}
	a.Notifier.Send(ctx, channels.Update{Text: text, Urgent: true})
	return "uploaded to the owner's Google Drive and sent the link: " + link, nil
}

func (a *App) StopJobs() int {
	a.jobsMu.Lock()
	defer a.jobsMu.Unlock()
	for _, cancel := range a.jobs {
		cancel()
	}
	return len(a.jobs)
}

func (a *App) StopAll() {
	a.Agent.Stop()
	a.StopJobs()
	if a.Voice != nil {
		a.Voice.Sleep()
	}
	a.Store.Log("stop", "stopped everything on your request")
}

func (a *App) voiceHandler(ctx context.Context, text string) (string, bool) {
	reply, actions := a.Agent.Run(ctx, agent.Input{Text: text, Origin: "voice"})
	return reply, len(actions) > 0
}

// showCard puts the oldest waiting approval in open chat windows and, with
// voice on, on the orb as a Yes / No / Later card (nil hides it). It never
// asks out loud: that would mean opening the mic to hear the answer.
func (a *App) showCard(r *approval.Request) {
	a.cardMu.Lock()
	a.card = r
	a.cardMu.Unlock()
	if r == nil {
		a.Events.Publish(chat.Event{Kind: "asked"})
		if a.Voice != nil {
			voice.SetAsk("", "")
		}
		return
	}
	a.Events.Publish(chat.Event{Kind: "ask", ID: r.ID, Text: r.Question})
	if a.Voice == nil {
		go exec.Command("notify-send", "-u", "critical", "-a", a.Config.Name, "Needs your OK (Space + K)", r.Question).Run()
		return
	}
	voice.SetAsk(r.ID, r.Question)
	go func() {
		if a.Config.UI.Orb {
			voice.LaunchOrb(a.Config.UI)
		} else {
			exec.Command("notify-send", "-u", "critical", "-a", a.Config.Name, "Needs your OK", r.Question).Run()
		}
		voice.PlayFile(config.Path("chime.wav"))
	}()
}

func (a *App) CurrentCard() *approval.Request {
	a.cardMu.Lock()
	defer a.cardMu.Unlock()
	return a.card
}

// AnswerCard applies a Yes / No / Later click from the orb.
func (a *App) AnswerCard() {
	if id, answer, ok := voice.TakeAnswer(); ok {
		a.Approvals.Resolve(id, answer)
	}
}

func (a *App) showStep(text string) {
	a.Events.Publish(chat.Event{Kind: "step", Text: text})
	if a.Voice != nil && a.Voice.Awake() {
		voice.SetMode("thinking", text)
	}
}

func (a *App) Update(text string) {
	a.Announce(text)
	a.Notifier.Send(context.Background(), channels.Update{Text: fmt.Sprintf("🤖 %s\n%s", a.Config.Name, text)})
}

func (a *App) Say(ctx context.Context, text string) {
	if a.Voice != nil {
		a.Voice.Say(ctx, text)
	}
}

func (a *App) mailUpdate(text string, urgent bool) {
	a.Notifier.Send(context.Background(), channels.Update{
		Text:    text,
		Buttons: []channels.Button{{Label: "📋 Show tasks", Data: "/tasks"}},
		Urgent:  urgent,
	})
}

func (a *App) sayNow(text string) {
	a.Announce(text)
}

func (a *App) Announce(text string) {
	a.Events.Publish(chat.Event{Kind: "notice", Text: text})
	if a.Voice != nil {
		go a.Voice.Announce(context.Background(), text)
	}
}

func (a *App) sendApproval(r approval.Request) {
	text := fmt.Sprintf("⚠️ %s needs your OK\n%s", a.Config.Name, r.Question)
	if r.Reason != "" {
		text += "\nWhy: " + r.Reason
	}
	a.Notifier.Send(context.Background(), channels.Update{
		Text: text,
		Buttons: []channels.Button{
			{Label: "✅ Yes", Data: "/approve " + r.ID},
			{Label: "❌ No", Data: "/deny " + r.ID},
			{Label: "⏰ Later", Data: "/later " + r.ID},
		},
		Urgent: true,
	})
}
