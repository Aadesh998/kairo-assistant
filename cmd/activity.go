package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"kairo/internal/store"
)

var (
	activityToday  bool
	activityLimit  int
	activityFollow bool
)

var activityCmd = &cobra.Command{
	Use:   "activity",
	Short: "Show everything Kairo heard, did and sent",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Open()
		if err != nil {
			return err
		}
		defer s.Close()
		since := time.Time{}
		if activityToday {
			n := time.Now()
			since = time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.Local)
		}
		list, err := s.Activity(since, 0, activityLimit)
		if err != nil {
			return err
		}
		lastID := int64(0)
		for i := len(list) - 1; i >= 0; i-- {
			printActivity(list[i])
			lastID = max(lastID, list[i].ID)
		}
		for activityFollow {
			time.Sleep(time.Second)
			newer, _ := s.Activity(time.Time{}, lastID, 100)
			for i := len(newer) - 1; i >= 0; i-- {
				printActivity(newer[i])
				lastID = max(lastID, newer[i].ID)
			}
		}
		return nil
	},
}

func printActivity(a store.Activity) {
	fmt.Printf("%s  %-9s %s\n", a.At.Local().Format("01-02 15:04:05"), a.Kind, a.Text)
}

func init() {
	activityCmd.Flags().BoolVarP(&activityToday, "today", "t", false, "only today")
	activityCmd.Flags().IntVarP(&activityLimit, "limit", "n", 40, "number of entries")
	activityCmd.Flags().BoolVarP(&activityFollow, "follow", "f", false, "keep printing new activity")
	rootCmd.AddCommand(activityCmd)
}
