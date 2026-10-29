package cmd

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"kairo/internal/config"
	"kairo/internal/tui"
)

const (
	chatClass = "dev.kairo.chat"
	chatRule  = `o.window("^(dev\\.kairo\\.chat|kairo-chat)$", { float = true, center = true, size = { 920, 660 }, tag = "-default-opacity", opacity = "1 1" })`
)

// The chat window gets the orb's look: deep indigo, light lavender text. It is
// passed as a config file because Omarchy's theme (also a config-file) would
// otherwise win over plain flags.
const chatTheme = `background = #13122a
foreground = #e8e5fb
background-opacity = 1
window-padding-x = 16
window-padding-y = 12
window-padding-color = background
cursor-style = bar
confirm-close-surface = false
selection-background = #3a3666
selection-foreground = #e8e5fb
`

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Chat window with Chat, Tasks and Activity tabs (Space + K opens it when voice is off)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunChat()
	},
}

// openChatWindow focuses the chat window if it is open, or opens it in a
// themed Ghostty window (any terminal through Omarchy when Ghostty is missing).
func openChatWindow() error {
	if address := chatWindow(); address != "" {
		return exec.Command("hyprctl", "dispatch", `hl.dsp.focus({ window = "address:`+address+`" })`).Run()
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exec.Command("hyprctl", "eval", chatRule).Run()
	var launch *exec.Cmd
	theme := config.Path("chat-ghostty.conf")
	if ghostty, err := exec.LookPath("ghostty"); err == nil && os.WriteFile(theme, []byte(chatTheme), 0o644) == nil {
		launch = exec.Command("setsid", ghostty, "--class="+chatClass, "--title=Kairo", "--gtk-single-instance=false",
			"--config-file="+theme, "-e", exe, "chat")
	} else {
		launch = exec.Command("omarchy-launch-tui", "--app-id=kairo-chat", exe, "chat")
	}
	if err := launch.Start(); err != nil {
		return err
	}
	return launch.Process.Release()
}

func chatWindow() string {
	out, err := exec.Command("hyprctl", "clients", "-j").Output()
	if err != nil {
		return ""
	}
	var clients []struct {
		Address string `json:"address"`
		Class   string `json:"class"`
	}
	json.Unmarshal(out, &clients)
	for _, c := range clients {
		if c.Class == chatClass || c.Class == "kairo-chat" {
			return c.Address
		}
	}
	return ""
}

func init() {
	rootCmd.AddCommand(chatCmd)
}
