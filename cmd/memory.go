package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"kairo/internal/app"
	"kairo/internal/store"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "See and edit what Kairo remembers about you",
}

var memoryListCmd = &cobra.Command{
	Use:   "ls",
	Short: "List memories",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *store.Store) error {
			list, err := s.Memories(100)
			for _, m := range list {
				fmt.Printf("#%-4d [%s] %s\n", m.ID, m.Kind, m.Text)
			}
			return err
		})
	},
}

var memoryAddCmd = &cobra.Command{
	Use:   "add [text]",
	Short: "Teach Kairo a fact about you",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *store.Store) error {
			_, err := s.AddMemory("fact", strings.Join(args, " "))
			return err
		})
	},
}

var memoryForgetCmd = &cobra.Command{
	Use:   "forget [id]",
	Short: "Delete a memory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return err
		}
		return withStore(func(s *store.Store) error { return s.DeleteMemory(id) })
	},
}

var memoryCompactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Summarize old conversations and merge long-term memory now",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Build(false)
		if err != nil {
			return err
		}
		defer a.Store.Close()
		return a.Memory.Compact(cmd.Context())
	},
}

func init() {
	memoryCmd.AddCommand(memoryListCmd, memoryAddCmd, memoryForgetCmd, memoryCompactCmd)
	rootCmd.AddCommand(memoryCmd)
}
