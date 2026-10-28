package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"kairo/internal/config"
	"kairo/internal/orb"
	"kairo/internal/voice"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create ~/.kairo with config, persona and .env files",
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, dir := range []string{config.Home(), config.Path("models"), config.Path("logs"), config.Path("skills")} {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
		}
		files := []struct {
			name    string
			content string
			mode    os.FileMode
		}{
			{"config.yaml", config.DefaultConfig, 0o644},
			{"persona.yaml", config.DefaultPersona, 0o644},
			{".env", config.DefaultEnv, 0o600},
		}
		for _, f := range files {
			path := config.Path(f.name)
			if _, err := os.Stat(path); err == nil {
				fmt.Println("exists  ", path)
				continue
			}
			if err := os.WriteFile(path, []byte(f.content), f.mode); err != nil {
				return err
			}
			fmt.Println("created ", path)
		}
		if err := voice.WriteChime(config.Path("chime.wav")); err != nil {
			return err
		}
		fmt.Println("rendering orb animation...")
		if err := orb.Generate(config.Path("orb")); err != nil {
			return err
		}
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Put your API keys in", config.Path(".env"))
		fmt.Println("  2. Download voice models: make models")
		fmt.Println("  3. Connect Google: kairo google login, then set mail.enabled, mail.address and calendar.enabled in", config.Path("config.yaml"))
		fmt.Println("  4. Write your daily routine in", config.Path("persona.yaml"))
		fmt.Println("  5. Try: kairo ask \"what is on my screen?\"")
		fmt.Println("  6. Start the agent: kairo start")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
