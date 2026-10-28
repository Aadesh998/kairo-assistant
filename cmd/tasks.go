package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"kairo/internal/store"
	"kairo/internal/tools"
)

var taskDue string

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Manage tasks and reminders",
}

var tasksAddCmd = &cobra.Command{
	Use:   "add [title]",
	Short: "Add a task, e.g. kairo tasks add \"pay rent\" --due \"2026-11-01 10:00\"",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		due, err := tools.ParseDue(taskDue)
		if err != nil {
			return err
		}
		return withStore(func(s *store.Store) error {
			id, err := s.AddTask(strings.Join(args, " "), due)
			fmt.Printf("task #%d added %s\n", id, tools.DueText(due))
			return err
		})
	},
}

var tasksListCmd = &cobra.Command{
	Use:   "ls",
	Short: "List open tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *store.Store) error {
			tasks, err := s.OpenTasks()
			for _, t := range tasks {
				fmt.Printf("#%-4d %s %s\n", t.ID, t.Title, tools.DueText(t.DueAt))
			}
			return err
		})
	},
}

var tasksDoneCmd = &cobra.Command{
	Use:   "done [id]",
	Short: "Mark a task as done",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return err
		}
		return withStore(func(s *store.Store) error { return s.CompleteTask(id) })
	},
}

func withStore(fn func(s *store.Store) error) error {
	s, err := store.Open()
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(s)
}

func init() {
	tasksAddCmd.Flags().StringVar(&taskDue, "due", "", "due time: 'YYYY-MM-DD HH:MM', 'HH:MM' or '30m'")
	tasksCmd.AddCommand(tasksAddCmd, tasksListCmd, tasksDoneCmd)
	rootCmd.AddCommand(tasksCmd)
}
