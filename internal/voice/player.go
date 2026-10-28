package voice

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"os"
	"os/exec"
)

func PlayStream(ctx context.Context, audio io.Reader) error {
	cmd := exec.CommandContext(ctx, "mpv", "--no-video", "--really-quiet", "--no-terminal", "-")
	cmd.Stdin = audio
	return cmd.Run()
}

func PlayFile(path string) error {
	return exec.Command("pw-play", path).Run()
}

func WriteChime(path string) error {
	const rate = 22050
	var pcm bytes.Buffer
	for _, note := range []struct {
		freq float64
		ms   int
	}{{880, 110}, {1320, 170}} {
		n := rate * note.ms / 1000
		for i := 0; i < n; i++ {
			fade := math.Min(1, math.Min(float64(i)/200, float64(n-i)/600))
			v := math.Sin(2*math.Pi*note.freq*float64(i)/rate) * 0.35 * fade
			binary.Write(&pcm, binary.LittleEndian, int16(v*32767))
		}
	}
	return os.WriteFile(path, WAV(pcm.Bytes(), rate), 0o644)
}
