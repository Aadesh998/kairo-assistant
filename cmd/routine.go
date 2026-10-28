package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"kairo/internal/app"
	"kairo/internal/persona"
	"kairo/internal/scheduler"
)

var routineCmd = &cobra.Command{
	Use:   "routine",
	Short: "Show or test your daily routine",
}

var routineShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the routine from persona.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := persona.Load()
		if err != nil {
			return err
		}
		for i, r := range p.Routine {
			fmt.Printf("%d  %s (runs %s)  %-16s %-20s %s\n", i, r.At, r.TriggerTime(), r.Do, r.Title, strings.Join(r.Days, ","))
		}
		return nil
	},
}

var routineRunCmd = &cobra.Command{
	Use:   "run [number]",
	Short: "Run one routine entry right now to test it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		a, err := app.Build(true)
		if err != nil {
			return err
		}
		defer a.Store.Close()
		if n < 0 || n >= len(a.Persona.Routine) {
			return fmt.Errorf("no routine number %d", n)
		}
		scheduler.RunRoutine(cmd.Context(), a, a.Persona.Routine[n])
		return nil
	},
}

func init() {
	routineCmd.AddCommand(routineShowCmd, routineRunCmd)
	rootCmd.AddCommand(routineCmd)
}
