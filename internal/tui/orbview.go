package tui

import (
	"fmt"
	"image"
	"strings"
	"sync"

	"kairo/internal/config"
	"kairo/internal/orb"
)

// miniOrb draws the real orb animation in the terminal with half-block
// characters (two pixels per cell), blended onto the chat window background.
type miniOrb struct {
	mu     sync.Mutex
	src    map[string][][]float64 // per mode, per frame: srcN×srcN premultiplied RGBA
	cols   int
	frames map[string][]string
}

const (
	srcN = 64
	crop = 372 // square around the orb, including its glow
)

var orbBG = [3]float64{0x13, 0x12, 0x2a}

func newMiniOrb() *miniOrb {
	o := &miniOrb{src: map[string][][]float64{}, frames: map[string][]string{}}
	go o.load()
	return o
}

// load decodes the cached orb frames once, mode by mode, in the background,
// so the window opens at once and the orb fades in as frames arrive.
func (o *miniOrb) load() {
	dir := config.Path("orb")
	if !orb.Ready(dir) {
		if orb.Generate(dir) != nil {
			return
		}
	}
	for _, mode := range []string{"awake", "listening", "thinking", "speaking"} {
		var frames [][]float64
		for i := 0; i < orb.Frames; i++ {
			img, err := orb.LoadFrame(dir, mode, i)
			if err != nil {
				return
			}
			frames = append(frames, shrink(img))
		}
		o.mu.Lock()
		o.src[mode] = frames
		o.frames[mode] = nil
		o.mu.Unlock()
	}
}

// shrink box-averages the square around the orb down to srcN×srcN.
func shrink(img image.Image) []float64 {
	x0, y0 := orb.Width/2-crop/2, orb.CenterY-crop/2
	out := make([]float64, srcN*srcN*4)
	for sy := 0; sy < srcN; sy++ {
		for sx := 0; sx < srcN; sx++ {
			var acc [4]float64
			n := 0.0
			for y := y0 + sy*crop/srcN; y < y0+(sy+1)*crop/srcN; y += 2 {
				for x := x0 + sx*crop/srcN; x < x0+(sx+1)*crop/srcN; x += 2 {
					r, g, b, a := img.At(x, y).RGBA() // premultiplied, 0..65535
					acc[0] += float64(r) / 257
					acc[1] += float64(g) / 257
					acc[2] += float64(b) / 257
					acc[3] += float64(a) / 65535
					n++
				}
			}
			for c := range acc {
				out[(sy*srcN+sx)*4+c] = acc[c] / max(n, 1)
			}
		}
	}
	return out
}

// Frame returns one animation frame as `cols` cells wide and cols/2 rows
// tall, or nil while the mode is still loading.
func (o *miniOrb) Frame(mode string, i, cols int) []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if cols != o.cols {
		o.cols = cols
		o.frames = map[string][]string{}
	}
	src := o.src[mode]
	if src == nil {
		if src = o.src["awake"]; src == nil {
			return nil
		}
		mode = "awake"
	}
	if o.frames[mode] == nil {
		o.frames[mode] = make([]string, len(src))
	}
	i %= len(src)
	if o.frames[mode][i] == "" {
		o.frames[mode][i] = cells(src[i], cols)
	}
	return strings.Split(o.frames[mode][i], "\n")
}

func cells(src []float64, cols int) string {
	n := cols // pixels per side: cols wide, cols tall (two per row)
	pixel := func(px, py int) ([3]float64, float64) {
		var acc [4]float64
		x0, x1 := px*srcN/n, max(px*srcN/n+1, (px+1)*srcN/n)
		y0, y1 := py*srcN/n, max(py*srcN/n+1, (py+1)*srcN/n)
		count := 0.0
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				for c := 0; c < 4; c++ {
					acc[c] += src[(y*srcN+x)*4+c]
				}
				count++
			}
		}
		a := acc[3] / count
		var rgb [3]float64
		for c := 0; c < 3; c++ {
			rgb[c] = acc[c]/count + orbBG[c]*(1-a)
		}
		return rgb, a
	}
	var b strings.Builder
	for row := 0; row < n/2; row++ {
		for col := 0; col < n; col++ {
			top, ta := pixel(col, row*2)
			bottom, ba := pixel(col, row*2+1)
			switch {
			case ta < 0.03 && ba < 0.03:
				b.WriteString(" ")
			case ba < 0.03:
				fmt.Fprintf(&b, "\x1b[38;2;%s;49m▀", rgbCode(top))
			case ta < 0.03:
				fmt.Fprintf(&b, "\x1b[38;2;%s;49m▄", rgbCode(bottom))
			default:
				fmt.Fprintf(&b, "\x1b[38;2;%s;48;2;%sm▀", rgbCode(top), rgbCode(bottom))
			}
		}
		b.WriteString("\x1b[0m")
		if row < n/2-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func rgbCode(c [3]float64) string {
	return fmt.Sprintf("%d;%d;%d", clamp(c[0]), clamp(c[1]), clamp(c[2]))
}

func clamp(v float64) int {
	return int(min(255, max(0, v)))
}
