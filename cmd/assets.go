package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"kairo/internal/config"
	"kairo/internal/orb"
)

var assetsCmd = &cobra.Command{
	Use:   "assets [dir]",
	Short: "Render the orb animation frames (default ~/.kairo/orb)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := config.Path("orb")
		if len(args) == 1 {
			dir = args[0]
		}
		if err := orb.Generate(dir); err != nil {
			return err
		}
		fmt.Printf("rendered %d frames for %v in %s\n", orb.Frames, orb.Modes, dir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(assetsCmd)
}
