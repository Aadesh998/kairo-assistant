package voice

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os/exec"
	"sync/atomic"
	"time"
)

const (
	sampleRate = 16000
	chunkBytes = 3200
	chunkTime  = 100 * time.Millisecond
)

type RecordOptions struct {
	WaitStart time.Duration
	MaxLength time.Duration
	Silence   time.Duration
	Threshold float64
	Mic       string
}

var lastPeak atomic.Uint64

func LastPeak() float64 {
	return math.Float64frombits(lastPeak.Load())
}

func Record(ctx context.Context, o RecordOptions) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	args := []string{"--rate", "16000", "--channels", "1", "--format", "s16"}
	if o.Mic != "" {
		args = append(args, "--target", o.Mic)
	}
	cmd := exec.CommandContext(ctx, "pw-record", append(args, "-")...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() {
		cancel()
		cmd.Wait()
	}()
	var stalled atomic.Bool
	watchdog := time.AfterFunc(3*time.Second, func() {
		stalled.Store(true)
		cancel()
	})
	defer watchdog.Stop()

	var audio bytes.Buffer
	var preroll [][]byte
	started := false
	var waited, quiet, length time.Duration
	floor, peak := -1.0, 0.0
	defer func() { lastPeak.Store(math.Float64bits(peak)) }()
	chunk := make([]byte, chunkBytes)
	for {
		if _, err := io.ReadFull(out, chunk); err != nil {
			if stalled.Load() {
				return nil, errors.New("the microphone sends no audio, check the default input in wpctl status")
			}
			if ctx.Err() != nil {
				return nil, nil
			}
			return nil, err
		}
		watchdog.Reset(3 * time.Second)
		level := rms(chunk)
		SetLevel(displayLevel(level))
		peak = math.Max(peak, level)
		if floor < 0 {
			floor = math.Min(level, o.Threshold)
		}
		loud := level > math.Max(o.Threshold, floor*4)
		if !loud {
			floor = floor*0.9 + level*0.1
		}
		if !started {
			preroll = append(preroll, append([]byte(nil), chunk...))
			if len(preroll) > 3 {
				preroll = preroll[1:]
			}
			waited += chunkTime
			if loud {
				started = true
				for _, p := range preroll {
					audio.Write(p)
				}
				continue
			}
			if waited >= o.WaitStart {
				return nil, nil
			}
			continue
		}
		audio.Write(chunk)
		length += chunkTime
		if loud {
			quiet = 0
		} else {
			quiet += chunkTime
		}
		if quiet >= o.Silence || length >= o.MaxLength {
			return audio.Bytes(), nil
		}
	}
}

func WAV(pcm []byte, rate int) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(pcm)))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(rate))
	binary.Write(&b, binary.LittleEndian, uint32(rate*2))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}
