package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/spf13/cobra"

	"kairo/internal/app"
	"kairo/internal/channels"
	"kairo/internal/config"
)

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Set up and test Telegram, Slack and WhatsApp",
}

var channelsListCmd = &cobra.Command{
	Use:   "ls",
	Short: "List enabled channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Build(false)
		if err != nil {
			return err
		}
		defer a.Store.Close()
		for _, name := range a.Notifier.Names() {
			fmt.Println(name)
		}
		return nil
	},
}

var channelsTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Send a test message to all enabled channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Build(false)
		if err != nil {
			return err
		}
		defer a.Store.Close()
		a.Notifier.Send(cmd.Context(), channels.Update{Text: "👋 Test message from " + a.Config.Name, Urgent: true})
		fmt.Println("sent, check: kairo activity -n 5")
		return nil
	},
}

var telegramIDCmd = &cobra.Command{
	Use:   "telegram-id",
	Short: "Show your Telegram chat id (send /start to your bot first)",
	RunE: func(cmd *cobra.Command, args []string) error {
		config.LoadEnv(config.Path(".env"))
		chats, err := channels.TelegramChats(cmd.Context(), os.Getenv("TELEGRAM_BOT_TOKEN"))
		if err != nil {
			return err
		}
		if len(chats) == 0 {
			fmt.Println("no messages yet: open your bot in Telegram and send /start")
		}
		for _, c := range chats {
			fmt.Println(c)
		}
		return nil
	},
}

var telegramSetupCmd = &cobra.Command{
	Use:   "telegram-setup",
	Short: "Wait for your message to the bot, then save your chat id and turn Telegram on",
	RunE: func(cmd *cobra.Command, args []string) error {
		config.LoadEnv(config.Path(".env"))
		token := os.Getenv("TELEGRAM_BOT_TOKEN")
		if token == "" {
			return fmt.Errorf("set TELEGRAM_BOT_TOKEN in %s first", config.Path(".env"))
		}
		fmt.Println("Now send any message (for example: hi) to your bot in Telegram. Waiting up to 3 minutes...")
		ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Minute)
		defer cancel()
		chatID, user, err := channels.WaitForChat(ctx, token)
		if err != nil {
			return err
		}
		path := config.Path("config.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		block := regexp.MustCompile(`(?m)^  telegram:\n    enabled: \w+\n    owner_chat_id: -?\d+`)
		updated := block.ReplaceAllString(string(data), fmt.Sprintf("  telegram:\n    enabled: true\n    owner_chat_id: %d", chatID))
		if updated == string(data) {
			return fmt.Errorf("got chat id %d but could not update %s, set channels.telegram by hand", chatID, path)
		}
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return err
		}
		fmt.Printf("Paired with @%s (chat id %d). Telegram is on. Restart Kairo: systemctl --user restart kairo\n", user, chatID)
		return nil
	},
}

var pairWhatsAppCmd = &cobra.Command{
	Use:   "pair-whatsapp",
	Short: "Link Kairo to WhatsApp by scanning a QR code",
	RunE: func(cmd *cobra.Command, args []string) error {
		return channels.PairWhatsApp(cmd.Context())
	},
}

func init() {
	channelsCmd.AddCommand(channelsListCmd, channelsTestCmd, telegramIDCmd, telegramSetupCmd, pairWhatsAppCmd)
	rootCmd.AddCommand(channelsCmd)
}
