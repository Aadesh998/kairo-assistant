package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"kairo/internal/app"
	"kairo/internal/config"
	"kairo/internal/daemon"
)

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Wake Kairo now, or open the chat window when voice is off (this is what Space + K runs)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg, err := config.Load(); err == nil && !cfg.Voice.Enabled {
			return openChatWindow()
		}
		if pid, ok := daemon.RunningPID(); ok {
			fmt.Println("Kairo is listening")
			return syscall.Kill(pid, syscall.SIGUSR1)
		}
		a, err := app.Build(true)
		if err != nil {
			return err
		}
		defer a.Store.Close()
		if a.Voice == nil {
			return errors.New("voice is disabled in config.yaml")
		}
		// Without a daemon the orb watches this process instead.
		os.WriteFile(config.Path("listen.pid"), []byte(strconv.Itoa(os.Getpid())), 0o600)
		defer os.Remove(config.Path("listen.pid"))
		a.Voice.Converse(cmd.Context())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listenCmd)
}
