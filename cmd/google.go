package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"kairo/internal/config"
	"kairo/internal/google"
)

var googleCmd = &cobra.Command{
	Use:   "google",
	Short: "Google account used for Gmail and Calendar",
}

var googleLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to Google (Gmail over IMAP + Google Calendar)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := google.Login(cmd.Context(), cfg.Browser.Bin, cfg.Browser.ProfileDir); err != nil {
			return err
		}
		fmt.Println("signed in. Now set mail.enabled / mail.address and calendar.enabled in", config.Path("config.yaml"))
		return nil
	},
}

func init() {
	googleCmd.AddCommand(googleLoginCmd)
	rootCmd.AddCommand(googleCmd)
}
