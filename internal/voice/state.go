package voice

import (
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"kairo/internal/config"
)

type State struct {
	Mode  string    `json:"mode"`
	Level float64   `json:"level"`
	Text  string    `json:"text"`
	At    time.Time `json:"at"`
	AskID string    `json:"ask_id,omitempty"`
	Ask   string    `json:"ask,omitempty"`
}

const orbRule = `o.window({ class = "^(kairo-orb)$" }, { tag = "-default-opacity", opacity = "1 1", float = true, pin = true, no_focus = true, border_size = 0, no_shadow = true, no_blur = true, rounding = 0, size = { 400, 540 }, move = { "(monitor_w-window_w-40)", "(monitor_h*0.05)" } })`

var (
	stateMu   sync.Mutex
	current   = State{Mode: "sleeping"}
	lastWrite time.Time
)

func SetMode(mode, text string) {
	stateMu.Lock()
	defer stateMu.Unlock()
	current.Mode = mode
	current.Text = text
	current.Level = 0
	writeState()
}

// SetAsk shows an approval card on the orb; an empty id hides it.
func SetAsk(id, question string) {
	stateMu.Lock()
	defer stateMu.Unlock()
	current.AskID = id
	current.Ask = question
	writeState()
}

// WriteAnswer is called by the orb when a card button is clicked; the daemon
// picks it up with TakeAnswer after SIGHUP.
func WriteAnswer(id, answer string) error {
	tmp := config.Path("answer.tmp")
	if err := os.WriteFile(tmp, []byte(id+" "+answer), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, config.Path("answer"))
}

func TakeAnswer() (string, string, bool) {
	data, err := os.ReadFile(config.Path("answer"))
	if err != nil {
		return "", "", false
	}
	os.Remove(config.Path("answer"))
	id, answer, ok := strings.Cut(strings.TrimSpace(string(data)), " ")
	return id, answer, ok
}

func Mode() string {
	stateMu.Lock()
	defer stateMu.Unlock()
	return current.Mode
}

func ReadLevel() float64 {
	stateMu.Lock()
	defer stateMu.Unlock()
	return current.Level
}

func SetLevel(level float64) {
	stateMu.Lock()
	defer stateMu.Unlock()
	current.Level = level
	if time.Since(lastWrite) > 60*time.Millisecond {
		writeState()
	}
}

func writeState() {
	current.At = time.Now()
	lastWrite = current.At
	data, _ := json.Marshal(current)
	tmp := config.Path("state.json.tmp")
	if os.WriteFile(tmp, data, 0o600) == nil {
		os.Rename(tmp, config.Path("state.json"))
	}
}

func ReadState() State {
	s := State{Mode: "sleeping"}
	data, err := os.ReadFile(config.Path("state.json"))
	if err == nil {
		json.Unmarshal(data, &s)
	}
	return s
}

func LaunchOrb(ui config.UI) {
	if !ui.Orb || orbRunning() {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exec.Command("hyprctl", "eval", orbRule).Run()
	command := strconv.Quote(exe + " orb")
	exec.Command("hyprctl", "dispatch", "hl.dsp.exec_cmd("+command+")").Run()
}

func orbRunning() bool {
	out, err := exec.Command("hyprctl", "clients", "-j").Output()
	return err == nil && strings.Contains(string(out), "kairo-orb")
}
