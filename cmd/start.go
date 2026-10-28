package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"

	"kairo/internal/config"
	"kairo/internal/daemon"
)

var background bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Kairo: Space + K voice, routine, reminders, mail, calendar and channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pid, ok := daemon.RunningPID(); ok {
			return fmt.Errorf("kairo is already running (pid %d)", pid)
		}
		if !background {
			fmt.Println("Kairo is running. Hold Space + K to talk, or press Ctrl+C to stop.")
			return daemon.Run(cmd.Context())
		}
		os.MkdirAll(config.Path("logs"), 0o700)
		logFile, err := os.OpenFile(config.Path("logs/daemon.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		child := exec.Command(exe, "start")
		child.Stdout = logFile
		child.Stderr = logFile
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := child.Start(); err != nil {
			return err
		}
		fmt.Printf("Kairo started in background (pid %d). Logs: %s\n", child.Process.Pid, config.Path("logs/daemon.log"))
		return nil
	},
}

func init() {
	startCmd.Flags().BoolVarP(&background, "daemon", "d", false, "run in background")
	rootCmd.AddCommand(startCmd)
}
