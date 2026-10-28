package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"kairo/internal/app"
	"kairo/internal/tools"
)

var calendarDays int

var calendarCmd = &cobra.Command{
	Use:   "calendar",
	Short: "Google Calendar (sign in first with: kairo google login)",
}

var calendarListCmd = &cobra.Command{
	Use:   "ls",
	Short: "Sync and list your upcoming events",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Build(false)
		if err != nil {
			return err
		}
		defer a.Store.Close()
		if err := a.Calendar.Sync(cmd.Context()); err != nil {
			return err
		}
		now := time.Now()
		list, err := a.Store.EventsBetween(now, now.AddDate(0, 0, calendarDays))
		if err != nil {
			return err
		}
		fmt.Println(tools.FormatEvents(list))
		return nil
	},
}

func init() {
	calendarListCmd.Flags().IntVarP(&calendarDays, "days", "d", 2, "days ahead (max 2 are synced)")
	calendarCmd.AddCommand(calendarListCmd)
	rootCmd.AddCommand(calendarCmd)
}
