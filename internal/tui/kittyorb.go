package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"sync"

	"golang.org/x/image/draw"

	"kairo/internal/config"
	"kairo/internal/orb"
)

// kittyOrb shows the real orb frames as images (kitty graphics protocol,
// supported by Ghostty and kitty), placed with Unicode placeholders so they
// move with the text like any other cell.
type kittyOrb struct {
	mu    sync.Mutex
	ready map[string][]uint32 // image ids per mode, once sent
}

const (
	orbCols  = 24
	orbRows  = 12
	orbTop   = 30  // the orb and its glow fill the frame below this line
	orbPx    = 240 // sent image width; the terminal fits it into the cells
	frameGap = 2   // send every 2nd frame: 24 per mode, played at 12 fps
	idBase   = 0x4b0000
)

// rowcolumn diacritics from the kitty spec: the n-th one encodes row/column n.
var diacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F, 0x0346, 0x034A,
	0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357, 0x035B, 0x0363, 0x0364, 0x0365,
	0x0366, 0x0367, 0x0368, 0x0369,
}

func graphicsTerminal() bool {
	term := os.Getenv("TERM")
	return term == "xterm-ghostty" || term == "xterm-kitty" || os.Getenv("TERM_PROGRAM") == "ghostty"
}

func newKittyOrb() *kittyOrb {
	o := &kittyOrb{ready: map[string][]uint32{}}
	go o.send()
	return o
}

// send prepares and transmits every frame once, calm mode first. Each image
// goes out in a single write so it cannot interleave with screen updates.
func (o *kittyOrb) send() {
	dir := config.Path("orb")
	if !orb.Ready(dir) {
		if orb.Generate(dir) != nil {
			return
		}
	}
	id := uint32(idBase)
	for _, mode := range []string{"awake", "listening", "thinking", "speaking"} {
		var ids []uint32
		for i := 0; i < orb.Frames; i += frameGap {
			img, err := orb.LoadFrame(dir, mode, i)
			if err != nil {
				return
			}
			id++
			if _, err := os.Stdout.Write(transmit(id, orbPNG(img))); err != nil {
				return
			}
			ids = append(ids, id)
		}
		o.mu.Lock()
		o.ready[mode] = ids
		o.mu.Unlock()
	}
}

func orbPNG(img image.Image) []byte {
	src := image.Rect(0, orbTop, orb.Width, orb.Height)
	out := image.NewNRGBA(image.Rect(0, 0, orbPx, orbPx*src.Dy()/src.Dx()))
	draw.CatmullRom.Scale(out, out.Bounds(), img, src, draw.Src, nil)
	var buf bytes.Buffer
	png.Encode(&buf, out)
	return buf.Bytes()
}

// transmit sends a PNG and creates a virtual placement for placeholders.
func transmit(id uint32, data []byte) []byte {
	payload := base64.StdEncoding.EncodeToString(data)
	var b bytes.Buffer
	for first := true; len(payload) > 0; first = false {
		chunk := payload[:min(4096, len(payload))]
		payload = payload[len(chunk):]
		more := 0
		if len(payload) > 0 {
			more = 1
		}
		if first {
			fmt.Fprintf(&b, "\x1b_Gf=100,a=t,t=d,i=%d,q=2,m=%d;%s\x1b\\", id, more, chunk)
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
	}
	fmt.Fprintf(&b, "\x1b_Ga=p,U=1,i=%d,c=%d,r=%d,q=2\x1b\\", id, orbCols, orbRows)
	return b.Bytes()
}

// Frame returns the placeholder cells for one frame, or nil until the first
// mode has been sent. Modes still sending fall back to the calm one.
func (o *kittyOrb) Frame(mode string, frame int) []string {
	o.mu.Lock()
	ids := o.ready[mode]
	if ids == nil {
		ids = o.ready["awake"]
	}
	o.mu.Unlock()
	if ids == nil {
		return nil
	}
	id := ids[(frame/frameGap)%len(ids)]
	color := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", id>>16&0xff, id>>8&0xff, id&0xff)
	lines := make([]string, orbRows)
	for r := range lines {
		var b strings.Builder
		b.WriteString(color)
		for c := 0; c < orbCols; c++ {
			b.WriteRune(0x10EEEE)
			b.WriteRune(diacritics[r])
			b.WriteRune(diacritics[c])
		}
		b.WriteString("\x1b[39m")
		lines[r] = b.String()
	}
	return lines
}

// clearImages frees the frames in the terminal when the window closes.
func clearImages() {
	os.Stdout.WriteString("\x1b_Ga=d,d=A,q=2\x1b\\")
}
