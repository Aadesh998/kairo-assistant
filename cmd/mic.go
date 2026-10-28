package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kairo/internal/config"
	"kairo/internal/voice"
)

var micCmd = &cobra.Command{
	Use:   "mic",
	Short: "Test the microphone: speak, then see what Kairo heard",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Println("Say something now, for example \"what's on my calendar today\". Waiting up to 10 seconds...")
		done := make(chan struct{})
		go meter(done)
		pcm, err := voice.Record(cmd.Context(), voice.RecordOptions{
			WaitStart: 10 * time.Second,
			MaxLength: 8 * time.Second,
			Silence:   time.Second,
			Threshold: cfg.Voice.SilenceLevel,
			Mic:       cfg.Voice.Mic,
		})
		close(done)
		fmt.Println()
		if err != nil {
			return err
		}
		if pcm == nil {
			fmt.Printf("No speech detected. Loudest sound was %.3f, speech starts above %.3f (voice.silence_level).\n", voice.LastPeak(), cfg.Voice.SilenceLevel)
			fmt.Println("Check the mic: wpctl status, unmute or pick the right source, or lower voice.silence_level in config.yaml.")
			return nil
		}
		fmt.Printf("Recorded %.1f seconds. Transcribing with %s...\n", float64(len(pcm))/32000, cfg.Voice.STT)
		text, err := voice.Transcribe(cmd.Context(), cfg.Voice, pcm)
		if err != nil {
			return err
		}
		fmt.Printf("Heard: %q\n", text)
		return nil
	},
}

func meter(done chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-time.After(100 * time.Millisecond):
			level := voice.ReadLevel()
			bar := strings.Repeat("█", int(level*40))
			fmt.Printf("\r  level %-40s %.2f", bar, level)
		}
	}
}

func init() {
	rootCmd.AddCommand(micCmd)
}
