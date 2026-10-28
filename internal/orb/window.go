package orb

import (
	"bytes"
	"errors"
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"

	"kairo/internal/config"
	"kairo/internal/voice"
)

const (
	fps          = 24
	windowWidth  = 400
	windowHeight = 540
	pillLine     = 44
)

var (
	errClosed  = errors.New("closed")
	pillColor  = color.NRGBA{R: 246, G: 245, B: 250, A: 255}
	captionInk = color.NRGBA{R: 62, G: 58, B: 86, A: 255}
	modeInk    = map[string]color.NRGBA{
		"listening": {R: 30, G: 150, B: 220, A: 255},
		"thinking":  {R: 140, G: 90, B: 220, A: 255},
		"speaking":  {R: 40, G: 160, B: 110, A: 255},
		"awake":     {R: 90, G: 90, B: 130, A: 255},
	}
)

type orbWindow struct {
	font          *text.GoTextFaceSource
	dir           string
	frames        map[string][]*ebiten.Image
	start         time.Time
	state         voice.State
	lastRead      time.Time
	sleepingSince time.Time
	level         float64
	placed        bool
	dragging      bool
	grabX, grabY  int
	lastClick     time.Time
	lastAlive     time.Time
}

func Run() error {
	dir := config.Path("orb")
	if !Ready(dir) {
		if err := Generate(dir); err != nil {
			return err
		}
	}
	font, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return err
	}
	ebiten.SetWindowTitle("Kairo")
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowDecorated(false)
	ebiten.SetTPS(60)
	w := &orbWindow{font: font, dir: dir, frames: map[string][]*ebiten.Image{}, start: time.Now()}
	err = ebiten.RunGameWithOptions(w, &ebiten.RunGameOptions{
		ScreenTransparent: true,
		InitUnfocused:     true,
		X11ClassName:      "kairo-orb",
		X11InstanceName:   "kairo-orb",
	})
	if errors.Is(err, errClosed) {
		return nil
	}
	return err
}

func (w *orbWindow) Update() error {
	if !w.placed && time.Since(w.start) > 500*time.Millisecond {
		if x, y, ok := loadPosition(); ok {
			ebiten.SetWindowPosition(x, y)
		}
		w.placed = true
	}
	if w.closeRequested() {
		signalDaemon(syscall.SIGUSR2)
		return errClosed
	}
	w.drag()
	now := time.Now()
	if now.Sub(w.lastAlive) > time.Second {
		if !daemonAlive() {
			return errClosed
		}
		w.lastAlive = now
	}
	if now.Sub(w.lastRead) > 80*time.Millisecond {
		w.state = voice.ReadState()
		w.lastRead = now
	}
	if w.state.Mode == "sleeping" || w.state.Mode == "" {
		if w.sleepingSince.IsZero() {
			w.sleepingSince = now
		} else if now.Sub(w.sleepingSince) > 1500*time.Millisecond {
			return errClosed
		}
	} else {
		w.sleepingSince = time.Time{}
	}
	w.level = w.level*0.75 + w.state.Level*0.25
	return nil
}

func (w *orbWindow) Draw(screen *ebiten.Image) {
	if !w.placed {
		return
	}
	mode := w.state.Mode
	if _, ok := palettes[mode]; !ok {
		mode = "awake"
	}
	frames := w.get(mode)
	frame := frames[int(time.Since(w.start).Seconds()*fps)%len(frames)]

	bounds := screen.Bounds()
	fit := float64(bounds.Dx()) / Width
	scale := 1.0
	if mode == "listening" || mode == "speaking" {
		scale += 0.08 * w.level
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-Width/2, -CenterY)
	op.GeoM.Scale(fit*scale, fit*scale)
	op.GeoM.Translate(float64(bounds.Dx())/2, CenterY*fit+float64(bounds.Dx())*0.04)
	screen.DrawImage(frame, op)
	w.drawPill(screen, fit)
}

func (w *orbWindow) drawPill(screen *ebiten.Image, unit float64) {
	title, body := pillText(w.state)
	titleFace := &text.GoTextFace{Source: w.font, Size: 13 * unit}
	bodyFace := &text.GoTextFace{Source: w.font, Size: 15 * unit}
	lines := wrap(body, pillLine, 2)
	lineH := 21 * unit
	boxW := float64(screen.Bounds().Dx()) - 40*unit
	boxH := 20*unit + 18*unit + lineH*float64(len(lines))
	x := 20 * unit
	y := float64(screen.Bounds().Dy()) - boxH - 14*unit
	r := float32(18 * unit)
	vector.FillRect(screen, float32(x)+r, float32(y), float32(boxW)-2*r, float32(boxH), pillColor, true)
	vector.FillRect(screen, float32(x), float32(y)+r, float32(boxW), float32(boxH)-2*r, pillColor, true)
	for _, c := range [][2]float64{{x, y}, {x + boxW, y}, {x, y + boxH}, {x + boxW, y + boxH}} {
		cx := c[0] + math.Copysign(float64(r), x+boxW/2-c[0])
		cy := c[1] + math.Copysign(float64(r), y+boxH/2-c[1])
		vector.FillCircle(screen, float32(cx), float32(cy), r, pillColor, true)
	}
	ink, ok := modeInk[w.state.Mode]
	if !ok {
		ink = modeInk["awake"]
	}
	top := &text.DrawOptions{}
	top.GeoM.Translate(x+16*unit, y+10*unit)
	top.ColorScale.ScaleWithColor(ink)
	text.Draw(screen, title, titleFace, top)
	for i, line := range lines {
		op := &text.DrawOptions{}
		op.GeoM.Translate(x+16*unit, y+30*unit+lineH*float64(i))
		op.ColorScale.ScaleWithColor(captionInk)
		text.Draw(screen, line, bodyFace, op)
	}
}

func pillText(s voice.State) (string, string) {
	switch s.Mode {
	case "listening":
		if s.Text != "" {
			return "• LISTENING", s.Text
		}
		return "• LISTENING", "Go ahead, I'm listening…"
	case "thinking":
		if s.Text == "" || s.Text == "..." {
			return "• WORKING", "Understanding what you said…"
		}
		return "• WORKING", s.Text
	case "speaking":
		return "• SPEAKING", s.Text
	case "notice":
		return "• NOTIFICATION", s.Text
	}
	return "• KAIRO", "Yes? Tell me what to do."
}

func wrap(text string, width, maxLines int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if len([]rune(line))+len([]rune(word))+1 > width && line != "" {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		lines = append(lines, line)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = strings.TrimRight(lines[maxLines-1], ".,") + "…"
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

func (w *orbWindow) drag() {
	cx, cy := ebiten.CursorPosition()
	if w.onSphere(cx, cy) || w.dragging {
		ebiten.SetCursorShape(ebiten.CursorShapeMove)
	} else {
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && w.onSphere(cx, cy) {
		w.dragging = true
		w.grabX, w.grabY = cx, cy
	}
	if !w.dragging {
		return
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		w.dragging = false
		savePosition(ebiten.WindowPosition())
		return
	}
	if dx, dy := cx-w.grabX, cy-w.grabY; dx != 0 || dy != 0 {
		wx, wy := ebiten.WindowPosition()
		ebiten.SetWindowPosition(wx+dx, wy+dy)
	}
}

func (w *orbWindow) closeRequested() bool {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		return true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		double := time.Since(w.lastClick) < 400*time.Millisecond
		w.lastClick = time.Now()
		return double
	}
	return false
}

func daemonPID() int {
	data, err := os.ReadFile(config.Path("daemon.pid"))
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

func daemonAlive() bool {
	pid := daemonPID()
	return pid > 0 && syscall.Kill(pid, 0) == nil
}

func signalDaemon(sig syscall.Signal) {
	if pid := daemonPID(); pid > 0 {
		syscall.Kill(pid, sig)
	}
}

func (w *orbWindow) onSphere(x, y int) bool {
	centerX := float64(windowWidth) / 2
	centerY := CenterY*float64(windowWidth)/Width + windowWidth*0.04
	radius := 112 * k * float64(windowWidth) / Width
	return math.Hypot(float64(x)-centerX, float64(y)-centerY) <= radius
}

func loadPosition() (int, int, bool) {
	var x, y int
	data, err := os.ReadFile(config.Path("orb-position"))
	if err != nil {
		return 0, 0, false
	}
	_, err = fmt.Sscanf(string(data), "%d %d", &x, &y)
	return x, y, err == nil
}

func savePosition(x, y int) {
	os.WriteFile(config.Path("orb-position"), []byte(fmt.Sprintf("%d %d", x, y)), 0o600)
}

func (w *orbWindow) Layout(outsideWidth, outsideHeight int) (int, int) {
	s := ebiten.Monitor().DeviceScaleFactor()
	return int(float64(outsideWidth) * s), int(float64(outsideHeight) * s)
}

func (w *orbWindow) get(mode string) []*ebiten.Image {
	if list, ok := w.frames[mode]; ok {
		return list
	}
	var list []*ebiten.Image
	for i := 0; i < Frames; i++ {
		img, err := readPNG(FramePath(w.dir, mode, i))
		if err != nil {
			img = Render(mode, i)
		}
		list = append(list, ebiten.NewImageFromImage(img))
	}
	w.frames[mode] = list
	return list
}
