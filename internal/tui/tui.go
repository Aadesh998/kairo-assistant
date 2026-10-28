package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"kairo/internal/config"
	"kairo/internal/daemon"
	"kairo/internal/persona"
	"kairo/internal/store"
	"kairo/internal/tools"
	"kairo/internal/voice"
)

type dashModel struct {
	store   *store.Store
	cfg     *config.Config
	persona *persona.Persona
	view    string
}

const reset = "\033[0m"

type refresh time.Time

func RunDashboard() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	p, err := persona.Load()
	if err != nil {
		return err
	}
	s, err := store.Open()
	if err != nil {
		return err
	}
	defer s.Close()
	m := dashModel{store: s, cfg: cfg, persona: p}
	m.view = m.render()
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func every() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return refresh(t) })
}

func (m dashModel) Init() tea.Cmd {
	return every()
}

func (m dashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case refresh:
		m.view = m.render()
		return m, every()
	}
	return m, nil
}

func (m dashModel) View() string {
	return m.view
}

func (m dashModel) render() string {
	var b strings.Builder
	title := "\033[1;38;5;39m"
	dim := "\033[38;5;245m"
	b.WriteString(fmt.Sprintf("%s  %s — dashboard%s   %s(q to quit)%s\n\n", title, strings.ToUpper(m.cfg.Name), reset, dim, reset))

	b.WriteString(title + "  STATUS" + reset + "\n")
	b.WriteString("  " + StatusLine() + "\n\n")

	b.WriteString(title + "  ROUTINE" + reset + "\n")
	for _, r := range m.persona.Routine {
		b.WriteString(fmt.Sprintf("  %s  %-16s %s\n", r.At, r.Do, r.Title))
	}

	b.WriteString("\n" + title + "  OPEN TASKS" + reset + "\n")
	tasks, _ := m.store.OpenTasks()
	if len(tasks) == 0 {
		b.WriteString(dim + "  none" + reset + "\n")
	}
	for _, t := range tasks {
		b.WriteString(fmt.Sprintf("  #%-3d %s %s\n", t.ID, t.Title, tools.DueText(t.DueAt)))
	}

	b.WriteString("\n" + title + "  LEARNED SCORES" + reset + "\n")
	scores, _ := m.store.Scores()
	for i, sc := range scores {
		if i == 6 {
			break
		}
		b.WriteString(fmt.Sprintf("  %5.2f  (%2d)  %s\n", sc.Score, sc.Count, sc.Action))
	}

	b.WriteString("\n" + title + "  ACTIVITY TODAY" + reset + "\n")
	n := time.Now()
	list, _ := m.store.Activity(time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.Local), 0, 15)
	for _, a := range list {
		text := a.Text
		if len(text) > 90 {
			text = text[:90] + "…"
		}
		b.WriteString(fmt.Sprintf("  %s%s%s  %-9s %s\n", dim, a.At.Local().Format("15:04:05"), reset, a.Kind, text))
	}
	return b.String()
}

func StatusLine() string {
	daemonText := "stopped"
	if pid, ok := daemon.RunningPID(); ok {
		daemonText = fmt.Sprintf("running (pid %d)", pid)
	}
	return fmt.Sprintf("daemon: %s   voice: %s", daemonText, voice.ReadState().Mode)
}
