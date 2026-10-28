package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"

	"kairo/internal/config"
)

const elevenBase = "https://api.elevenlabs.io/v1"

func elevenKey() (string, error) {
	key := os.Getenv("ELEVENLABS_API_KEY")
	if key == "" {
		return "", errors.New("ELEVENLABS_API_KEY is not set in ~/.kairo/.env")
	}
	return key, nil
}

func elevenTranscribe(ctx context.Context, cfg config.Voice, pcm []byte) (string, error) {
	key, err := elevenKey()
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	form.WriteField("model_id", cfg.ElevenSTTModel)
	if cfg.Language != "" {
		form.WriteField("language_code", cfg.Language)
	}
	file, _ := form.CreateFormFile("file", "speech.wav")
	file.Write(WAV(pcm, sampleRate))
	form.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", elevenBase+"/speech-to-text", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("xi-api-key", key)
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("elevenlabs speech-to-text %d: %s", resp.StatusCode, msg)
	}
	var out struct {
		Text string `json:"text"`
	}
	err = json.NewDecoder(resp.Body).Decode(&out)
	return out.Text, err
}

func elevenSpeak(ctx context.Context, cfg config.Voice, text string) error {
	key, err := elevenKey()
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"text": text, "model_id": cfg.ElevenTTSModel})
	url := fmt.Sprintf("%s/text-to-speech/%s/stream?output_format=pcm_22050", elevenBase, cfg.ElevenVoiceID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("xi-api-key", key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elevenlabs text-to-speech %d: %s", resp.StatusCode, msg)
	}
	player := exec.CommandContext(ctx, "pw-play", "--raw", "--rate", "22050", "--channels", "1", "--format", "s16", "-")
	player.Stdin = resp.Body
	return player.Run()
}
