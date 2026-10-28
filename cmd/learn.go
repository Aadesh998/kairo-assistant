package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"kairo/internal/store"
)

var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: "See what Kairo learned from your approvals and feedback",
}

var learnStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show learned scores per action",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *store.Store) error {
			scores, err := s.Scores()
			fmt.Println("score  count  action")
			for _, sc := range scores {
				fmt.Printf("%5.2f  %5d  %s\n", sc.Score, sc.Count, sc.Action)
			}
			return err
		})
	},
}

var learnResetCmd = &cobra.Command{
	Use:   "reset [action]",
	Short: "Forget what was learned for one action",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *store.Store) error { return s.ResetScore(args[0]) })
	},
}

func init() {
	learnCmd.AddCommand(learnStatsCmd, learnResetCmd)
	rootCmd.AddCommand(learnCmd)
}
