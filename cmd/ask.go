package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"kairo/internal/agent"
	"kairo/internal/app"
	"kairo/internal/approval"
)

var askCmd = &cobra.Command{
	Use:   "ask [request]",
	Short: "Ask Kairo to do something from the terminal",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Build(false)
		if err != nil {
			return err
		}
		defer a.Store.Close()
		a.Approvals.Send = nil
		a.Approvals.Voice = approval.Terminal{}
		reply := a.Agent.Handle(cmd.Context(), agent.Input{Text: strings.Join(args, " "), Origin: "cli"})
		fmt.Println(reply)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(askCmd)
}
