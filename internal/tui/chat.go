package tui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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

// The orb's palette on a deep indigo glass background (the window itself is
// themed to match in cmd/chat.go).
var (
	cText     = lipgloss.Color("#e8e5fb")
	cDim      = lipgloss.Color("#8a86b3")
	cFaint    = lipgloss.Color("#3a3666")
	cPanel    = lipgloss.Color("#1e1c3d")
	cLavender = lipgloss.Color("#a08cff")
	cPurple   = lipgloss.Color("#c08cff")
	cBlue     = lipgloss.Color("#6fb3ff")
	cCyan     = lipgloss.Color("#78e1f5")
	cGreen    = lipgloss.Color("#5fd3a0")
	cAmber    = lipgloss.Color("#f5b14c")
	cRed      = lipgloss.Color("#ff7a8a")

	textStyle = lipgloss.NewStyle().Foreground(cText)
	dimStyle  = lipgloss.NewStyle().Foreground(cDim)
	bold      = lipgloss.NewStyle().Bold(true)
	rounded   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
)

const sideW = 30

type entry struct {
	who  string // you, kairo, notice, error
	text string
}

type chatModel struct {
	name      string
	store     *store.Store
	client    *chat.Client
	orb       *miniOrb  // half-block fallback
	kitty     *kittyOrb // real frames in Ghostty / kitty
	start     time.Time
	tab       int
	width     int
	height    int
	entries   []entry
	input     []rune
	busy      bool
	step      string
	ask       *chat.Event
	offline   bool
	repliedAt time.Time
	scroll    int
	tasks     []store.Task
	cursor    int
	activity  []store.Activity
	actTop    int
}

type (
	replyMsg struct {
		text string
		err  error
	}
	eventMsg chat.Event
	errMsg   struct{ err error }
	tickMsg  time.Time
	animMsg  time.Time
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
	m := &chatModel{name: cfg.Name, store: s, client: chat.NewClient(), start: time.Now()}
	if !graphicsTerminal() {
		m.orb = newMiniOrb()
	}
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
	if m.kitty != nil {
		clearImages()
	}
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

func animate() tea.Cmd {
	return tea.Tick(70*time.Millisecond, func(t time.Time) tea.Msg { return animMsg(t) })
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
	return tea.Batch(tick(), animate())
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
		// Images are sent once the alternate screen is up: it keeps its own
		// image storage.
		if m.kitty == nil && m.orb == nil {
			m.kitty = newKittyOrb()
		}
	case tickMsg:
		m.refresh()
		return m, tick()
	case animMsg:
		return m, animate()
	case replyMsg:
		m.busy, m.step = false, ""
		if msg.err != nil {
			m.entries = append(m.entries, entry{"error", msg.err.Error()})
		} else {
			m.entries = append(m.entries, entry{"kairo", msg.text})
			m.repliedAt = time.Now()
		}
		m.scroll = 0
	case eventMsg:
		m.offline = false
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
		if errors.Is(msg.err, chat.ErrNoDaemon) {
			m.offline = true
		}
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

// status is what the orb and its pill show, mirroring the floating orb.
func (m *chatModel) status() (mode, title, text string, ink lipgloss.Color) {
	switch {
	case m.offline:
		return "awake", "• OFFLINE", "Start Kairo: systemctl --user start kairo", cRed
	case m.ask != nil:
		return "listening", "• NEEDS YOUR OK", "Answer with y, n or l", cAmber
	case m.busy:
		if m.step == "" {
			return "thinking", "• WORKING", "Thinking…", cPurple
		}
		return "thinking", "• WORKING", m.step, cPurple
	case time.Since(m.repliedAt) < 3*time.Second:
		return "speaking", "• REPLIED", "Done.", cGreen
	case len(m.input) > 0:
		return "listening", "• LISTENING", "Go ahead, I'm reading…", cBlue
	}
	return "awake", "• " + strings.ToUpper(m.name), "Ready when you are.", cLavender
}

func (m *chatModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	mainW := m.width
	var side string
	if m.width >= 72 && m.height >= 18 {
		mainW = m.width - sideW - 2
		side = m.sidebar()
	}
	main := m.mainPanel(mainW)
	if side == "" {
		return main
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, side, "  ", main)
}

func (m *chatModel) sidebar() string {
	mode, title, text, ink := m.status()
	var lines []string
	frame := int(time.Since(m.start).Seconds() * 24)
	var orbLines []string
	if m.kitty != nil {
		if orbLines = m.kitty.Frame(mode, frame); orbLines == nil {
			orbLines = make([]string, orbRows)
		}
	} else if m.orb != nil {
		rows := min(12, max(5, m.height-17))
		if orbLines = m.orb.Frame(mode, frame, rows*2); orbLines == nil {
			orbLines = make([]string, rows)
		}
	}
	lines = append(lines, "")
	for _, l := range orbLines {
		lines = append(lines, lipgloss.PlaceHorizontal(sideW, lipgloss.Center, l))
	}
	pill := rounded.BorderForeground(cFaint).Background(cPanel).BorderBackground(lipgloss.NoColor{}).
		Width(sideW-2).Padding(0, 1).
		Render(bold.Foreground(ink).Background(cPanel).Render(title) + "\n" +
			textStyle.Background(cPanel).Render(truncate(text, sideW-4)))
	lines = append(lines, "")
	lines = append(lines, strings.Split(pill, "\n")...)
	lines = append(lines, "")
	for i, t := range tabs {
		label := t
		if t == "Tasks" && len(m.tasks) > 0 {
			label += dimStyle.Render(fmt.Sprintf("  %d", len(m.tasks)))
		}
		if i == m.tab {
			lines = append(lines, "  "+bold.Foreground(cLavender).Render("● "+t)+strings.TrimPrefix(label, t))
		} else {
			lines = append(lines, "  "+dimStyle.Render("○ "+t)+strings.TrimPrefix(label, t))
		}
	}
	for len(lines) < m.height-1 {
		lines = append(lines, "")
	}
	lines = append(lines[:m.height-1], dimStyle.Render("  tab switch · esc close"))
	return lipgloss.NewStyle().Width(sideW).Render(strings.Join(lines, "\n"))
}

func (m *chatModel) mainPanel(w int) string {
	header := m.header(w)
	var footer []string
	if m.ask != nil {
		footer = append(footer, m.askBar(w))
	}
	if m.tab == 0 {
		footer = append(footer, m.inputBox(w))
	}
	footer = append(footer, m.help(w))
	foot := strings.Join(footer, "\n")
	bodyH := max(0, m.height-lipgloss.Height(header)-lipgloss.Height(foot))
	var body []string
	switch m.tab {
	case 0:
		body = m.chatBody(w, bodyH)
	case 1:
		body = m.taskBody(w)
	case 2:
		body = m.activityBody(w, bodyH)
	}
	if len(body) > bodyH {
		body = body[:bodyH]
	}
	for len(body) < bodyH {
		body = append(body, "")
	}
	return lipgloss.NewStyle().Width(w).MaxHeight(m.height).
		Render(header + "\n" + strings.Join(body, "\n") + "\n" + foot)
}

func (m *chatModel) header(w int) string {
	subtitle := map[int]string{
		0: "Talk to " + m.name,
		1: fmt.Sprintf("%d open", len(m.tasks)),
		2: "Today, newest first",
	}[m.tab]
	title := bold.Foreground(cText).Render(tabs[m.tab]) + "  " + dimStyle.Render(subtitle)
	return "\n" + title + "\n" + lipgloss.NewStyle().Foreground(cFaint).Render(strings.Repeat("─", w))
}

func (m *chatModel) askBar(w int) string {
	text := bold.Foreground(cAmber).Render("Needs your OK") + "\n" + textStyle.Render(m.ask.Text) + "\n\n" +
		keyCap("y", "Yes", cGreen) + "   " + keyCap("n", "No", cRed) + "   " + keyCap("l", "Later", cDim)
	return rounded.BorderForeground(cAmber).Width(w-2).Padding(0, 1).Render(text)
}

func keyCap(key, label string, c lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#13122a")).Background(c).Bold(true).Padding(0, 1).Render(key) +
		" " + lipgloss.NewStyle().Foreground(c).Render(label)
}

func (m *chatModel) inputBox(w int) string {
	border := cLavender
	content := textStyle.Render(string(m.input)) + lipgloss.NewStyle().Foreground(cCyan).Render("▎")
	if len(m.input) == 0 {
		content = lipgloss.NewStyle().Foreground(cCyan).Render("▎") + dimStyle.Render("Message "+m.name+"…")
	}
	if m.busy {
		border = cFaint
	}
	// Keep the end of a long message visible.
	inner := w - 7
	if lipgloss.Width(content) > inner {
		runes := m.input[max(0, len(m.input)-inner+1):]
		content = textStyle.Render(string(runes)) + lipgloss.NewStyle().Foreground(cCyan).Render("▎")
	}
	return rounded.BorderForeground(border).Width(w-2).Padding(0, 1).
		Render(bold.Foreground(cLavender).Render("› ") + content)
}

func (m *chatModel) help(w int) string {
	var text string
	switch m.tab {
	case 0:
		text = "enter send · ↑/↓ scroll · ctrl+x stop"
	case 1:
		text = "↑/↓ select · d done · s snooze 10 min"
	case 2:
		text = "↑/↓ scroll · g newest"
	}
	return dimStyle.Render(truncate(" "+text, w))
}

func (m *chatModel) chatBody(w, height int) []string {
	maxBubble := max(20, w*4/5)
	var lines []string
	if len(m.entries) == 0 {
		lines = append(lines, "", dimStyle.Render("Ask "+m.name+" anything, e.g. “what's on my calendar today?”"))
	}
	for _, e := range m.entries {
		lines = append(lines, "")
		lines = append(lines, strings.Split(m.bubble(e, w, maxBubble), "\n")...)
	}
	if m.busy {
		spin := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}[int(time.Since(m.start).Milliseconds()/90)%10]
		lines = append(lines, "", lipgloss.NewStyle().Foreground(cPurple).Render(spin+" ")+dimStyle.Render(m.name+" is working…"))
	}
	end := len(lines) - min(m.scroll, max(0, len(lines)-height))
	m.scroll = len(lines) - end
	return lines[max(0, end-height):end]
}

func (m *chatModel) bubble(e entry, w, maxBubble int) string {
	label, border, align := m.name, cLavender, lipgloss.Left
	switch e.who {
	case "you":
		label, border, align = "You", cBlue, lipgloss.Right
	case "notice":
		label, border = "Update", cAmber
	case "error":
		label, border = "Error", cRed
	}
	text := e.text
	if e.who != "you" {
		text = markdown(text)
	}
	textW := 0
	for _, l := range strings.Split(text, "\n") {
		textW = max(textW, lipgloss.Width(l))
	}
	box := rounded.BorderForeground(border).Padding(0, 1).Width(min(textW, maxBubble-4) + 2).
		Foreground(cText).Render(text)
	head := lipgloss.NewStyle().Foreground(border).Bold(true).Render(label)
	if e.who == "kairo" {
		head = lipgloss.NewStyle().Foreground(cCyan).Render("◉ ") + head
	}
	return lipgloss.NewStyle().Width(w).Align(align).Render(head + "\n" + box)
}

var (
	mdBold   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	mdCode   = regexp.MustCompile("`([^`]+)`")
	mdHeader = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	mdBullet = regexp.MustCompile(`(?m)^(\s*)[-*]\s+`)
)

// markdown renders the bits of Markdown that replies use: bold, code,
// headings and bullets.
func markdown(text string) string {
	text = mdHeader.ReplaceAllStringFunc(text, func(s string) string {
		return bold.Foreground(cLavender).Render(mdHeader.FindStringSubmatch(s)[1])
	})
	text = mdBullet.ReplaceAllString(text, "${1}• ")
	text = mdBold.ReplaceAllStringFunc(text, func(s string) string {
		return bold.Foreground(cText).Render(mdBold.FindStringSubmatch(s)[1])
	})
	return mdCode.ReplaceAllStringFunc(text, func(s string) string {
		return lipgloss.NewStyle().Foreground(cCyan).Render(mdCode.FindStringSubmatch(s)[1])
	})
}

func (m *chatModel) taskBody(w int) []string {
	lines := []string{""}
	if len(m.tasks) == 0 {
		return append(lines, dimStyle.Render("No open tasks. Ask "+m.name+" to remind you of something."))
	}
	for i, t := range m.tasks {
		due := tools.DueText(t.DueAt)
		dueStyle := dimStyle
		if !t.DueAt.IsZero() && t.DueAt.Before(time.Now()) {
			due, dueStyle = "overdue · "+t.DueAt.Local().Format("Mon 15:04"), lipgloss.NewStyle().Foreground(cRed)
		}
		marker, title, row := "  ", textStyle, lipgloss.NewStyle().Width(w)
		if i == m.cursor {
			marker = lipgloss.NewStyle().Foreground(cLavender).Background(cPanel).Render("▍ ")
			title = title.Background(cPanel).Bold(true)
			row = row.Background(cPanel)
			dueStyle = dueStyle.Background(cPanel)
		}
		lines = append(lines, row.Render(marker+title.Render(truncate(t.Title, w-4))))
		lines = append(lines, row.Render(marker+dueStyle.Render(strings.Trim(due, "()"))), "")
	}
	return lines
}

func (m *chatModel) activityBody(w, height int) []string {
	lines := []string{""}
	if len(m.activity) == 0 {
		return append(lines, dimStyle.Render("Nothing yet today."))
	}
	for _, a := range m.activity[min(m.actTop, len(m.activity)):] {
		text := strings.Join(strings.Fields(a.Text), " ")
		line := dimStyle.Render(a.At.Local().Format("15:04")) + "  " +
			lipgloss.NewStyle().Foreground(kindColor(a.Kind)).Width(9).Render(a.Kind) + " " + textStyle.Render(text)
		lines = append(lines, truncate(line, w))
		if len(lines) >= height {
			break
		}
	}
	return lines
}

func kindColor(kind string) lipgloss.Color {
	switch kind {
	case "error", "blocked", "denied":
		return cRed
	case "reply":
		return cGreen
	case "tool":
		return cLavender
	case "chat", "voice", "telegram", "cli":
		return cBlue
	case "notify", "meeting", "mail":
		return cAmber
	}
	return cDim
}

func truncate(s string, width int) string {
	if width <= 1 || lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width-1).Render(s) + "…"
}
