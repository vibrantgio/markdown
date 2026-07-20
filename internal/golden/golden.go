// Package golden provides a golden-image test harness for Gio widgets.
//
// # Usage
//
//	func TestMyWidget(t *testing.T) {
//	    golden.Render(t, "my-widget", image.Pt(200, 200), func(gtx layout.Context) layout.Dimensions {
//	        paint.FillShape(gtx.Ops, color.NRGBA{R: 255, A: 255},
//	            clip.Rect{Max: gtx.Constraints.Max}.Op())
//	        return layout.Dimensions{Size: gtx.Constraints.Max}
//	    })
//	}
//
// # File layout
//
// Golden images live in testdata/golden/<name>.png relative to the calling
// test's package directory (the directory go test uses as the working directory).
//
// # Updating goldens
//
//	go test -golden.update ./...
//
// # CI gate
//
// If a golden file does not exist and -golden.update is NOT set, the test
// fails with a message directing the developer to run -golden.update. This
// prevents silently passing tests with no stored baseline.
package golden

import (
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
)

var update = flag.Bool("golden.update", false, "overwrite golden images with current output")

// Render renders draw into a headless window of size and diffs the result
// against testdata/golden/<name>.png.
//
// If -golden.update is set, the stored image is written (or overwritten) and
// the test passes. Otherwise the stored golden must exist; if it is absent the
// test fails with instructions to run -golden.update.
//
// On a pixel mismatch the test fails and saves the actual output alongside the
// golden as testdata/golden/<name>.actual.png for side-by-side inspection.
func Render(t *testing.T, name string, size image.Point, draw layout.Widget) {
	t.Helper()
	img := Capture(t, size, draw)
	path := filepath.Join("testdata", "golden", name+".png")

	if *update {
		if err := saveImage(path, img); err != nil {
			t.Fatalf("golden: save %s: %v", path, err)
		}
		return
	}

	stored, err := loadImage(path)
	if os.IsNotExist(err) {
		t.Fatalf("golden: %s not found; run go test -golden.update to create", path)
		return
	}
	if err != nil {
		t.Fatalf("golden: load %s: %v", path, err)
		return
	}

	if n := PixelDiff(stored, img); n > 0 {
		actualPath := strings.TrimSuffix(path, ".png") + ".actual.png"
		_ = saveImage(actualPath, img)
		t.Fatalf("golden: %q: %d pixel(s) differ (actual saved to %s)", name, n, actualPath)
	}
}

// Capture renders draw into a headless window of size and returns the RGBA
// pixel data. The test is skipped if headless rendering is not available on
// the current platform.
func Capture(t *testing.T, size image.Point, draw layout.Widget) *image.RGBA {
	t.Helper()
	w, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("golden: headless rendering not supported: %v", err)
	}
	defer w.Release()

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Ops:         &ops,
	}
	draw(gtx)

	if err := w.Frame(&ops); err != nil {
		t.Fatalf("golden: Frame: %v", err)
	}

	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := w.Screenshot(img); err != nil {
		t.Fatalf("golden: Screenshot: %v", err)
	}
	return img
}

// PixelDiff counts the number of pixels that differ between a and b.
// Returns -1 if the images have different sizes.
func PixelDiff(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	bounds := a.Bounds()
	n := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			aOff := (y-bounds.Min.Y)*a.Stride + (x-bounds.Min.X)*4
			bOff := (y-bounds.Min.Y)*b.Stride + (x-bounds.Min.X)*4
			if a.Pix[aOff] != b.Pix[bOff] ||
				a.Pix[aOff+1] != b.Pix[bOff+1] ||
				a.Pix[aOff+2] != b.Pix[bOff+2] ||
				a.Pix[aOff+3] != b.Pix[bOff+3] {
				n++
			}
		}
	}
	return n
}

func saveImage(path string, img *image.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// headless.Screenshot fills *image.RGBA with straight-alpha (non-premultiplied)
	// pixel values. Wrapping as *image.NRGBA before encoding tells png.Encode to
	// store the bytes as-is, avoiding a premultiplication pass that would corrupt
	// the stored values for anti-aliased (partially-transparent) edge pixels.
	nrgba := &image.NRGBA{Pix: img.Pix, Stride: img.Stride, Rect: img.Rect}
	return png.Encode(f, nrgba)
}

// loadImage reads a PNG from path and returns it as *image.RGBA.
// The raw pixel bytes are reinterpreted directly, so straight-alpha data
// written by saveImage round-trips without any alpha conversion.
func loadImage(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	switch v := decoded.(type) {
	case *image.RGBA:
		return v, nil
	case *image.NRGBA:
		// png.Decode returns NRGBA for 8-bit RGBA PNGs.
		// The raw Pix bytes are identical to RGBA layout, so we
		// reinterpret in-place without any alpha conversion.
		return &image.RGBA{Pix: v.Pix, Stride: v.Stride, Rect: v.Rect}, nil
	default:
		bounds := decoded.Bounds()
		rgba := image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				rgba.Set(x, y, decoded.At(x, y))
			}
		}
		return rgba, nil
	}
}
