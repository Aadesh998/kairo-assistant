package orb

import (
	"image"
	"image/color"
	"math"
)

const (
	Width   = 400
	Height  = 450
	Frames  = 48
	CenterY = Height * 150 / 360
	k       = Width / 320.0
)

var Modes = []string{"awake", "listening", "thinking", "speaking"}

type rgb [3]float64

type palette struct {
	blobs [4]rgb
	speed float64
	pulse float64
	glow  float64
}

var palettes = map[string]palette{
	"awake":     {blobs: [4]rgb{{150, 120, 245}, {95, 175, 250}, {120, 225, 245}, {185, 150, 250}}, speed: 1, pulse: 0.02, glow: 0.30},
	"listening": {blobs: [4]rgb{{140, 115, 248}, {80, 170, 252}, {105, 225, 248}, {170, 140, 252}}, speed: 1, pulse: 0.05, glow: 0.40},
	"thinking":  {blobs: [4]rgb{{175, 110, 245}, {120, 95, 240}, {225, 140, 230}, {110, 200, 250}}, speed: 2, pulse: 0.03, glow: 0.34},
	"speaking":  {blobs: [4]rgb{{105, 155, 255}, {85, 210, 250}, {150, 120, 250}, {130, 235, 240}}, speed: 1, pulse: 0.10, glow: 0.48},
}

var (
	white     = rgb{255, 255, 255}
	glass     = rgb{241, 240, 252}
	base      = rgb{234, 231, 250}
	glowColor = rgb{150, 122, 240}
	streak    = rgb{220, 250, 255}
)

type blob struct {
	x, y, size float64
	color      rgb
}

func Render(mode string, frame int) *image.NRGBA {
	p, ok := palettes[mode]
	if !ok {
		p = palettes["awake"]
	}
	t := float64(frame) / Frames
	cx, cy := Width/2.0, float64(CenterY)
	r := 112.0 * k * (1 + p.pulse*math.Sin(2*math.Pi*t*2))
	blobs := makeBlobs(p, t, cx, cy, r)
	img := image.NewNRGBA(image.Rect(0, 0, Width, Height))
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			c, a := pixel(px, py, cx, cy, r, t, p, blobs)
			img.SetNRGBA(x, y, color.NRGBA{uint8(c[0]), uint8(c[1]), uint8(c[2]), uint8(a * 255)})
		}
	}
	return img
}

func makeBlobs(p palette, t, cx, cy, r float64) []blob {
	var list []blob
	for i := 0; i < 4; i++ {
		fi := float64(i)
		dir := 1.0
		if i%2 == 1 {
			dir = -1
		}
		angle := fi*math.Pi/2 + 0.4 + dir*2*math.Pi*p.speed*t
		orbit := r * (0.32 + 0.10*math.Sin(2*math.Pi*(t+fi/4)))
		list = append(list, blob{
			x:     cx + orbit*math.Cos(angle),
			y:     cy + orbit*0.8*math.Sin(angle),
			size:  r * (0.30 + 0.06*math.Sin(2*math.Pi*(2*t+fi/3))),
			color: p.blobs[i],
		})
	}
	return list
}

func pixel(px, py, cx, cy, r, t float64, p palette, blobs []blob) (rgb, float64) {
	gx := (px - cx) / (r * 0.8)
	gy := (py - (cy + r*0.95)) / (r * 0.55)
	glowA := p.glow * 1.3 * math.Exp(-(gx*gx+gy*gy)*1.4)

	dx, dy := px-cx, py-cy
	dist := math.Hypot(dx, dy)
	sphereA := clamp(r - dist + 0.5)
	if sphereA <= 0 {
		return glowColor, glowA
	}

	c := base
	for _, b := range blobs {
		d2 := (px-b.x)*(px-b.x) + (py-b.y)*(py-b.y)
		c = mix(c, b.color, clamp(math.Exp(-d2/(2*b.size*b.size))*0.85))
	}
	wave := 0.5 + 0.5*math.Sin((dx*0.7+dy)*0.035/k+2*math.Pi*p.speed*t)
	c = mix(c, white, 0.10*wave)

	inner := r * 0.84
	c = mix(c, glass, smoothstep(inner-7, inner+5, dist)*0.78)
	c = mix(c, white, math.Exp(-math.Pow((r-dist)/2.2, 2))*0.6)

	light := (dx + dy) / (2 * r)
	c = mix(c, white, clamp(-light*0.22))
	c = mix(c, rgb{120, 110, 200}, clamp(light*0.10))

	if dist < inner {
		sx, sy := dx+0.15*r, dy-0.05*r
		ring := math.Hypot(sx, sy)
		angle := math.Atan2(sy, sx) * 180 / math.Pi
		center := -50 + 10*math.Sin(2*math.Pi*t)
		mask := math.Exp(-math.Pow((angle-center)/24, 2))
		band := math.Exp(-math.Pow((ring-0.50*r)/(0.10*r), 2))
		c = mix(c, rgb{95, 185, 250}, clamp(band*mask*0.75))
		arc := math.Exp(-math.Pow((ring-0.62*r)/(0.03*r), 2))
		c = mix(c, streak, clamp(arc*mask*1.2))
	}

	a := sphereA + glowA*(1-sphereA)
	out := mix(glowColor, c, sphereA/math.Max(a, 1e-6))
	return out, a
}

func mix(a, b rgb, t float64) rgb {
	return rgb{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

func clamp(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func smoothstep(e0, e1, x float64) float64 {
	t := clamp((x - e0) / (e1 - e0))
	return t * t * (3 - 2*t)
}
