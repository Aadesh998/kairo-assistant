package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"kairo/internal/tui"
)

const chatRule = `o.window("kairo-chat", { float = true, center = true, size = { 860, 640 } })`

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Chat window with Chat, Tasks and Activity tabs (Space + K opens it when voice is off)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunChat()
	},
}

// openChatWindow opens the chat in a floating terminal, or focuses it if it
// is already open.
func openChatWindow() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exec.Command("hyprctl", "eval", chatRule).Run()
	launch := exec.Command("omarchy-launch-or-focus-tui", "--app-id=kairo-chat", exe, "chat")
	if err := launch.Start(); err != nil {
		return err
	}
	return launch.Process.Release()
}

func init() {
	rootCmd.AddCommand(chatCmd)
}
