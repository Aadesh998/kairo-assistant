package cmd

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"

	"kairo/internal/daemon"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running Kairo agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		pid, ok := daemon.RunningPID()
		if !ok {
			fmt.Println("kairo is not running")
			return nil
		}
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return err
		}
		fmt.Printf("stopped kairo (pid %d)\n", pid)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
