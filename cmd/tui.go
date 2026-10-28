package cmd

import (
	"github.com/spf13/cobra"

	"kairo/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Live dashboard: status, routine, tasks, learning and activity",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunDashboard()
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
