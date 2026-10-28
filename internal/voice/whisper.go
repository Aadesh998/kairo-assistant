package voice

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"kairo/internal/config"
)

var whisperTags = regexp.MustCompile(`\[[^\]]*\]|\([^)]*\)`)

func whisperTranscribe(ctx context.Context, cfg config.Voice, pcm []byte) (string, error) {
	f, err := os.CreateTemp(config.Home(), "speech-*.wav")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	f.Write(WAV(pcm, sampleRate))
	f.Close()
	out, err := exec.CommandContext(ctx, cfg.WhisperBin,
		"-m", cfg.WhisperModel, "-f", f.Name(), "-l", cfg.Language, "-nt", "-np").Output()
	if err != nil {
		return "", err
	}
	text := whisperTags.ReplaceAllString(string(out), " ")
	return strings.Join(strings.Fields(text), " "), nil
}
