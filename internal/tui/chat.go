package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"kairo/internal/approval"
	"kairo/internal/chat"
	"kairo/internal/config"
	"kairo/internal/store"
	"kairo/internal/tools"
)

var tabs = []string{"Chat", "Tasks", "Activity"}

var (
	accent    = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	dimmed    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	youStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	tabOn     = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("4")).Bold(true).Padding(0, 1)
	tabOff    = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Padding(0, 1)
)

type entry struct {
	who  string // you, kairo, notice, error
	text string
}

type chatModel struct {
	name     string
	store    *store.Store
	client   *chat.Client
	tab      int
	width    int
	height   int
	entries  []entry
	input    []rune
	busy     bool
	step     string
	ask      *chat.Event
	scroll   int
	tasks    []store.Task
	cursor   int
	activity []store.Activity
	actTop   int
}

type (
	replyMsg struct {
		text string
		err  error
	}
	eventMsg chat.Event
	errMsg   struct{ err error }
	tickMsg  time.Time
)

// RunChat opens the chat window: Chat, Tasks and Activity tabs. Messages go
// to the running daemon; tasks and activity are read from the database.
func RunChat() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	s, err := store.Open()
	if err != nil {
		return err
	}
	defer s.Close()
	m := &chatModel{name: cfg.Name, store: s, client: chat.NewClient()}
	past, _ := s.RecentMessages("chat", 40)
	for _, msg := range past {
		who := "kairo"
		if msg.Role == "user" {
			who = "you"
		}
		m.entries = append(m.entries, entry{who, msg.Text})
	}
	m.refresh()
	p := tea.NewProgram(m, tea.WithAltScreen())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.watch(ctx, p)
	_, err = p.Run()
	return err
}

// watch follows daemon events, reconnecting if the daemon restarts.
func (m *chatModel) watch(ctx context.Context, p *tea.Program) {
	warned := false
	for ctx.Err() == nil {
		err := m.client.Events(ctx, func(e chat.Event) {
			warned = false
			p.Send(eventMsg(e))
		})
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, chat.ErrNoDaemon) && !warned {
			p.Send(errMsg{err})
			warned = true
		}
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
		}
	}
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *chatModel) refresh() {
	m.tasks, _ = m.store.OpenTasks()
	if m.cursor >= len(m.tasks) {
		m.cursor = max(0, len(m.tasks)-1)
	}
	n := time.Now()
	m.activity, _ = m.store.Activity(time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.Local), 0, 300)
}

func (m *chatModel) Init() tea.Cmd {
	return tick()
}

func (m *chatModel) send(text string) tea.Cmd {
	return func() tea.Msg {
		reply, err := m.client.Send(context.Background(), text)
		return replyMsg{reply, err}
	}
}

func (m *chatModel) answer(a string) tea.Cmd {
	id := m.ask.ID
	m.ask = nil
	return func() tea.Msg {
		if err := m.client.Answer(context.Background(), id, a); err != nil {
			return errMsg{err}
		}
		return nil
	}
}

func (m *chatModel) stop() tea.Cmd {
	return func() tea.Msg {
		if err := m.client.Stop(context.Background()); err != nil {
			return errMsg{err}
		}
		return eventMsg{Kind: "notice", Text: "Stopped everything."}
	}
}

func (m *chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		m.refresh()
		return m, tick()
	case replyMsg:
		m.busy, m.step = false, ""
		if msg.err != nil {
			m.entries = append(m.entries, entry{"error", msg.err.Error()})
		} else {
			m.entries = append(m.entries, entry{"kairo", msg.text})
		}
		m.scroll = 0
	case eventMsg:
		switch msg.Kind {
		case "step":
			m.step = msg.Text
		case "ask":
			e := chat.Event(msg)
			m.ask = &e
		case "asked":
			m.ask = nil
		case "notice":
			m.entries = append(m.entries, entry{"notice", msg.Text})
			m.scroll = 0
		}
	case errMsg:
		m.entries = append(m.entries, entry{"error", msg.err.Error()})
		m.scroll = 0
	case tea.KeyMsg:
		return m, m.key(msg)
	}
	return m, nil
}

func (m *chatModel) key(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "ctrl+c", "esc":
		return tea.Quit
	case "tab":
		m.tab = (m.tab + 1) % len(tabs)
		return nil
	case "shift+tab":
		m.tab = (m.tab + len(tabs) - 1) % len(tabs)
		return nil
	case "alt+1", "alt+2", "alt+3":
		m.tab = int(k.String()[4] - '1')
		return nil
	case "ctrl+x":
		return m.stop()
	}
	if m.ask != nil && (m.tab != 0 || len(m.input) == 0) {
		switch k.String() {
		case "y":
			return m.answer(approval.Yes)
		case "n":
			return m.answer(approval.No)
		case "l":
			return m.answer(approval.Later)
		}
	}
	switch m.tab {
	case 0:
		return m.chatKey(k)
	case 1:
		return m.taskKey(k)
	}
	return m.activityKey(k)
}

func (m *chatModel) chatKey(k tea.KeyMsg) tea.Cmd {
	switch k.Type {
	case tea.KeyEnter:
		text := strings.TrimSpace(string(m.input))
		if text == "" || m.busy {
			return nil
		}
		m.input = nil
		m.entries = append(m.entries, entry{"you", text})
		m.busy, m.step, m.scroll = true, "", 0
		return m.send(text)
	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	case tea.KeyCtrlU:
		m.input = nil
	case tea.KeyCtrlW:
		t := strings.TrimRight(string(m.input), " ")
		m.input = []rune(t[:strings.LastIndex(t, " ")+1])
	case tea.KeySpace:
		m.input = append(m.input, ' ')
	case tea.KeyRunes:
		m.input = append(m.input, k.Runes...)
	case tea.KeyPgUp, tea.KeyUp:
		m.scroll += 3
	case tea.KeyPgDown, tea.KeyDown:
		m.scroll = max(0, m.scroll-3)
	}
	return nil
}

func (m *chatModel) taskKey(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = min(len(m.tasks)-1, m.cursor+1)
	case "d", "enter":
		if len(m.tasks) > 0 {
			m.store.CompleteTask(m.tasks[m.cursor].ID)
			m.refresh()
		}
	case "s":
		if len(m.tasks) > 0 {
			m.store.SnoozeTask(m.tasks[m.cursor].ID, time.Now().Add(10*time.Minute))
			m.refresh()
		}
	}
	return nil
}

func (m *chatModel) activityKey(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "up", "k":
		m.actTop = max(0, m.actTop-1)
	case "down", "j":
		m.actTop = min(max(0, len(m.activity)-1), m.actTop+1)
	case "pgup":
		m.actTop = max(0, m.actTop-10)
	case "pgdown":
		m.actTop = min(max(0, len(m.activity)-1), m.actTop+10)
	case "home", "g":
		m.actTop = 0
	}
	return nil
}

func (m *chatModel) View() string {
	if m.width == 0 {
		return ""
	}
	header := m.header()
	var footer []string
	if m.ask != nil {
		footer = append(footer, m.askBar())
	}
	footer = append(footer, m.help())
	if m.tab == 0 {
		footer = append(footer, m.inputLine())
	}
	bodyH := m.height - lipgloss.Height(header) - lipgloss.Height(strings.Join(footer, "\n"))
	var body []string
	switch m.tab {
	case 0:
		body = m.chatBody(bodyH)
	case 1:
		body = m.taskBody()
	case 2:
		body = m.activityBody(bodyH)
	}
	if len(body) > bodyH {
		body = body[:max(0, bodyH)]
	}
	for len(body) < bodyH {
		body = append(body, "")
	}
	return header + "\n" + strings.Join(body, "\n") + "\n" + strings.Join(footer, "\n")
}

func (m *chatModel) header() string {
	var parts []string
	for i, t := range tabs {
		if i == m.tab {
			parts = append(parts, tabOn.Render(t))
		} else {
			parts = append(parts, tabOff.Render(t))
		}
	}
	title := accent.Render(" " + strings.ToUpper(m.name) + " ")
	return title + "  " + strings.Join(parts, " ") + "\n" + dimmed.Render(strings.Repeat("─", m.width))
}

func (m *chatModel) askBar() string {
	text := warnStyle.Render("Needs your OK: ") + m.ask.Text
	keys := youStyle.Render("[y] Yes") + "  " + errStyle.Render("[n] No") + "  " + dimmed.Render("[l] Later")
	return lipgloss.NewStyle().Width(m.width).Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("3")).Render(text + "\n" + keys)
}

func (m *chatModel) help() string {
	var text string
	switch m.tab {
	case 0:
		text = "enter send · ↑/↓ scroll · ctrl+x stop · tab switch · esc close"
		if m.busy {
			step := m.step
			if step == "" {
				step = "Thinking…"
			}
			return warnStyle.Render("● ") + truncate(step, m.width-20) + dimmed.Render("  (ctrl+x stop)")
		}
	case 1:
		text = "↑/↓ select · d done · s snooze 10 min · tab switch · esc close"
	case 2:
		text = "↑/↓ scroll · g top · tab switch · esc close"
	}
	return dimmed.Render(text)
}

func (m *chatModel) inputLine() string {
	return accent.Render("› ") + string(m.input) + "█"
}

func (m *chatModel) chatBody(height int) []string {
	wrap := lipgloss.NewStyle().Width(max(10, m.width-2))
	var lines []string
	if len(m.entries) == 0 {
		lines = append(lines, "", dimmed.Render(" Ask "+m.name+" anything, e.g. “what's on my calendar today?”"))
	}
	for _, e := range m.entries {
		var label string
		switch e.who {
		case "you":
			label = youStyle.Render("You")
		case "kairo":
			label = accent.Render(m.name)
		case "notice":
			label = warnStyle.Render("Update")
		default:
			label = errStyle.Render("Error")
		}
		lines = append(lines, "", " "+label)
		for _, l := range strings.Split(wrap.Render(e.text), "\n") {
			lines = append(lines, " "+l)
		}
	}
	if m.busy {
		lines = append(lines, "", " "+dimmed.Render(m.name+" is working…"))
	}
	end := len(lines) - min(m.scroll, max(0, len(lines)-height))
	m.scroll = len(lines) - end
	return lines[max(0, end-height):end]
}

func (m *chatModel) taskBody() []string {
	lines := []string{""}
	if len(m.tasks) == 0 {
		return append(lines, dimmed.Render(" No open tasks."))
	}
	for i, t := range m.tasks {
		line := fmt.Sprintf(" #%-4d %s %s", t.ID, t.Title, dimmed.Render(tools.DueText(t.DueAt)))
		if !t.DueAt.IsZero() && t.DueAt.Before(time.Now()) {
			line += " " + errStyle.Render("overdue")
		}
		if i == m.cursor {
			line = accent.Render("▸") + line
		} else {
			line = " " + line
		}
		lines = append(lines, truncate(line, m.width))
	}
	return lines
}

func (m *chatModel) activityBody(height int) []string {
	lines := []string{""}
	if len(m.activity) == 0 {
		return append(lines, dimmed.Render(" Nothing yet today."))
	}
	for _, a := range m.activity[min(m.actTop, len(m.activity)):] {
		text := strings.Join(strings.Fields(a.Text), " ")
		line := fmt.Sprintf(" %s  %-9s %s", dimmed.Render(a.At.Local().Format("15:04:05")), kindStyle(a.Kind).Render(a.Kind), text)
		lines = append(lines, truncate(line, m.width))
		if len(lines) >= height {
			break
		}
	}
	return lines
}

func kindStyle(kind string) lipgloss.Style {
	switch kind {
	case "error", "blocked", "denied":
		return errStyle
	case "reply", "tool":
		return accent
	}
	return dimmed
}

func truncate(s string, width int) string {
	if width <= 1 || lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width-1).Render(s) + "…"
}
