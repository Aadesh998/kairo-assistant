package voice

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"kairo/internal/config"
)

func piperSpeak(ctx context.Context, cfg config.Voice, text string) error {
	f, err := os.CreateTemp(config.Home(), "reply-*.wav")
	if err != nil {
		return err
	}
	f.Close()
	defer os.Remove(f.Name())
	cmd := exec.CommandContext(ctx, cfg.PiperBin, "-m", cfg.PiperModel, "-f", f.Name())
	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return &speakError{err: err, out: string(out)}
	}
	return exec.CommandContext(ctx, "pw-play", f.Name()).Run()
}

type speakError struct {
	err error
	out string
}

func (e *speakError) Error() string {
	return "piper: " + e.err.Error() + " " + strings.TrimSpace(e.out)
}
