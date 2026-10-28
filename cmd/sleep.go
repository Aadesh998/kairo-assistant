package cmd

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"

	"kairo/internal/daemon"
)

var sleepCmd = &cobra.Command{
	Use:     "sleep",
	Aliases: []string{"cancel"},
	Short:   "Stop everything now: cancels the current task, meeting joins and closes the orb",
	RunE: func(cmd *cobra.Command, args []string) error {
		pid, ok := daemon.RunningPID()
		if !ok {
			fmt.Println("kairo is not running")
			return nil
		}
		return syscall.Kill(pid, syscall.SIGUSR2)
	},
}

func init() {
	rootCmd.AddCommand(sleepCmd)
}
