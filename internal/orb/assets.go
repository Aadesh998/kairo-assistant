package orb

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
)

func FramePath(dir, mode string, frame int) string {
	return filepath.Join(dir, mode, fmt.Sprintf("%02d.png", frame))
}

func Generate(dir string) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(Modes)*Frames)
	for _, mode := range Modes {
		if err := os.MkdirAll(filepath.Join(dir, mode), 0o755); err != nil {
			return err
		}
		for i := 0; i < Frames; i++ {
			wg.Add(1)
			go func(mode string, i int) {
				defer wg.Done()
				if err := writePNG(FramePath(dir, mode, i), Render(mode, i)); err != nil {
					errs <- err
				}
			}(mode, i)
		}
	}
	wg.Wait()
	close(errs)
	return <-errs
}

func Ready(dir string) bool {
	for _, mode := range Modes {
		if _, err := os.Stat(FramePath(dir, mode, Frames-1)); err != nil {
			return false
		}
	}
	return true
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func LoadFrame(dir, mode string, frame int) (image.Image, error) {
	return readPNG(FramePath(dir, mode, frame))
}
