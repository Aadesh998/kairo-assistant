package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kairo/internal/config"
	"kairo/internal/persona"
	"kairo/internal/store"
	"kairo/internal/tui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show if Kairo is running and what is coming next",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		p, err := persona.Load()
		if err != nil {
			return err
		}
		s, err := store.Open()
		if err != nil {
			return err
		}
		defer s.Close()

		fmt.Println(tui.StatusLine())
		var channels []string
		if cfg.Channels.Telegram.Enabled {
			channels = append(channels, "telegram")
		}
		if cfg.Channels.Slack.Enabled {
			channels = append(channels, "slack")
		}
		if cfg.Channels.WhatsApp.Enabled {
			channels = append(channels, "whatsapp")
		}
		fmt.Printf("wake: Space + K   voice: %s / %s   channels: %s\n", cfg.Voice.STT, cfg.Voice.TTS, strings.Join(channels, ", "))

		mailText := "off"
		if cfg.Mail.Enabled {
			mailText = "watching " + cfg.Mail.Address
		}
		jevText := "missing TYPESAFE_API_KEY (using fallback)"
		if os.Getenv("TYPESAFE_API_KEY") != "" {
			jevText = "ready (" + cfg.Jev.Model + ")"
		}
		calendarText := "off"
		if cfg.Calendar.Enabled {
			calendarText = "on (" + cfg.Calendar.CalendarID + ")"
		}
		fmt.Printf("mail: %s   calendar: %s   jev: %s\n", mailText, calendarText, jevText)

		clock := time.Now().Format("15:04")
		for _, r := range p.Routine {
			if r.At > clock {
				fmt.Printf("next: %s %s\n", r.At, r.Label())
				break
			}
		}
		tasks, _ := s.OpenTasks()
		scores, _ := s.Scores()
		auto := 0
		for _, sc := range scores {
			if sc.Score >= cfg.Learning.AutoScore && sc.Count >= cfg.Learning.AutoCount {
				auto++
			}
		}
		fmt.Printf("tasks: %d open   learning: %d actions tracked, %d auto-approved\n", len(tasks), len(scores), auto)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
