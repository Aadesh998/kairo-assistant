package cmd

import (
	"github.com/spf13/cobra"

	"kairo/internal/orb"
)

var orbCmd = &cobra.Command{
	Use:    "orb",
	Short:  "Show the animated orb window",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return orb.Run()
	},
}

func init() {
	rootCmd.AddCommand(orbCmd)
}
