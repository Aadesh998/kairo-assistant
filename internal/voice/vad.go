package voice

import (
	"encoding/binary"
	"math"
)

func rms(chunk []byte) float64 {
	n := len(chunk) / 2
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		s := float64(int16(binary.LittleEndian.Uint16(chunk[i*2:]))) / 32768
		sum += s * s
	}
	return math.Sqrt(sum / float64(n))
}

func displayLevel(level float64) float64 {
	return math.Min(1, level*8)
}
