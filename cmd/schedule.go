package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"kairo/internal/store"
	"kairo/internal/tools"
)

var scheduleTitle string

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Scheduled jobs: join a meeting at a time, or run an instruction later",
}

var scheduleListCmd = &cobra.Command{
	Use:   "ls",
	Short: "List pending jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *store.Store) error {
			list, err := s.PendingScheduled()
			fmt.Println(tools.FormatScheduled(list))
			return err
		})
	},
}

var scheduleJoinCmd = &cobra.Command{
	Use:   "join [url] [time]",
	Short: "Join a meeting at a time with mic and camera off, e.g. kairo schedule join https://meet.google.com/abc 15:00",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		at, err := tools.ParseDue(args[1])
		if err != nil {
			return err
		}
		return withStore(func(s *store.Store) error {
			id, err := s.AddScheduled(at, "join", scheduleTitle, args[0])
			fmt.Printf("job #%d: Kairo will join %s at %s\n", id, args[0], at.Format("Mon 02 Jan 15:04"))
			return err
		})
	},
}

var scheduleCancelCmd = &cobra.Command{
	Use:   "cancel [id]",
	Short: "Cancel a pending job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return err
		}
		return withStore(func(s *store.Store) error { return s.SetScheduledStatus(id, "cancelled") })
	},
}

func init() {
	scheduleJoinCmd.Flags().StringVar(&scheduleTitle, "title", "", "meeting name")
	scheduleCmd.AddCommand(scheduleListCmd, scheduleJoinCmd, scheduleCancelCmd)
	rootCmd.AddCommand(scheduleCmd)
}
