package voice

import (
	"context"

	"kairo/internal/config"
)

func Transcribe(ctx context.Context, cfg config.Voice, pcm []byte) (string, error) {
	if cfg.STT == "elevenlabs" {
		return elevenTranscribe(ctx, cfg, pcm)
	}
	return whisperTranscribe(ctx, cfg, pcm)
}

func Speak(ctx context.Context, cfg config.Voice, text string) error {
	if cfg.TTS == "elevenlabs" {
		return elevenSpeak(ctx, cfg, text)
	}
	return piperSpeak(ctx, cfg, text)
}
