package golden_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/markdown/internal/golden"
)

// cornflowerRect is the fixed reference widget: a solid cornflower-blue
// rectangle filling the entire constraint. Using a named constant colour makes
// the stored golden self-documenting.
func cornflowerRect(gtx layout.Context) layout.Dimensions {
	paint.FillShape(gtx.Ops, color.NRGBA{R: 100, G: 149, B: 237, A: 255},
		clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// TestStable verifies that the golden harness produces a bit-identical PNG
// for the fixed reference widget on every run.
//
// The stored golden (testdata/golden/stable.png) is committed to the
// repository. If this test fails with a mismatch it means the rendering
// pipeline changed; if it fails with "not found", run:
//
//	go test -golden.update ./prism/internal/golden/
func TestStable(t *testing.T) {
	golden.Render(t, "stable", image.Pt(64, 64), cornflowerRect)
}

// TestOnepixelChangeDetected verifies that a one-pixel difference between two
// rendered images is flagged by PixelDiff. This test does not touch the
// file system; it validates the comparison primitive directly.
func TestOnepixelChangeDetected(t *testing.T) {
	size := image.Pt(64, 64)

	imgA := golden.Capture(t, size, cornflowerRect)

	// A widget identical to cornflowerRect but with a single white pixel
	// painted over position (0, 0).
	modified := func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, color.NRGBA{R: 100, G: 149, B: 237, A: 255},
			clip.Rect{Max: gtx.Constraints.Max}.Op())
		paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 255},
			clip.Rect{Max: image.Pt(1, 1)}.Op())
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	imgB := golden.Capture(t, size, modified)

	n := golden.PixelDiff(imgA, imgB)
	if n == 0 {
		t.Fatal("PixelDiff reported 0 differences; expected at least 1")
	}
	t.Logf("PixelDiff correctly detected %d differing pixel(s)", n)
}
