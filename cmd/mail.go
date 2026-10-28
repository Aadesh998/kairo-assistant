package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"kairo/internal/app"
	"kairo/internal/store"
)

var mailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Check Gmail (sign in first with: kairo google login)",
}

var mailCheckCmd = &cobra.Command{
	Use:   "check [count]",
	Short: "Read your latest emails and show how Kairo rates them",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		count := 5
		if len(args) == 1 {
			count, _ = strconv.Atoi(args[0])
		}
		a, err := app.Build(false)
		if err != nil {
			return err
		}
		defer a.Store.Close()
		emails, err := a.Mail.Latest(cmd.Context(), count)
		if err != nil {
			return err
		}
		for _, e := range emails {
			r := a.Mail.Triage.Decide(cmd.Context(), e)
			fmt.Printf("%-9s %s | %s\n", r.Importance, e.From, e.Subject)
			if r.Summary != "" {
				fmt.Println("          ", r.Summary)
			}
			if r.Task != "" {
				fmt.Println("           task:", r.Task, r.Due)
			}
		}
		return nil
	},
}

var mailListCmd = &cobra.Command{
	Use:   "ls",
	Short: "List mail Kairo has already seen",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withStore(func(s *store.Store) error {
			list, err := s.RecentMails(30)
			for _, m := range list {
				fmt.Printf("%s %-9s %s | %s\n", m.At.Local().Format("02 Jan 15:04"), m.Importance, m.From, m.Subject)
			}
			return err
		})
	},
}

func init() {
	mailCmd.AddCommand(mailCheckCmd, mailListCmd)
	rootCmd.AddCommand(mailCmd)
}
