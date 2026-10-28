package cmd

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/spf13/cobra"

	"kairo/internal/app"
	"kairo/internal/daemon"
)

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Wake Kairo now (this is what Space + K runs)",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		a.Voice.Converse(cmd.Context())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listenCmd)
}
