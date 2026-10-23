package markdown_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// TestScrolledCodeReachesTheRim asserts that a fence's side padding is inside
// its scroll area. At rest the band between the rim and the code is fill
// only. Scrolled, the padding has moved off with the start of the code, and
// glyph pixels are drawn in that band, up to the rim.
func TestScrolledCodeReachesTheRim(t *testing.T) {
	shaper := defaultShaper(t)
	c := tokens.PlatformLight
	blocks := markdown.Parse([]byte(codeOverflowSource))
	style := markdown.FromTokens(c, tokens.DefaultTypography, color.NRGBA{})
	fill := style.CodeBackground
	pad := int(tokens.Spacing.S3)

	rest := golden.Capture(t, codeOverflowSize, themed(markdown.NewDocument(blocks), shaper, style, c))
	w := themed(markdown.NewDocument(blocks), shaper, style, c)
	driveDocument(w, codeOverflowSize, pointer.Event{
		Kind:     pointer.Scroll,
		Position: f32.Pt(200, 70),
		Scroll:   f32.Pt(400, 0),
		Source:   pointer.Mouse,
	})
	scrolled := golden.Capture(t, codeOverflowSize, w)

	row := fenceRow(rest, fill)
	if row < 0 {
		t.Fatal("no fence in the capture")
	}
	// The rim is the first fill pixel on the fence's middle row.
	rim := -1
	for x := 0; x < codeOverflowSize.X && rim < 0; x++ {
		if rest.RGBAAt(x, row) == rgba(fill) {
			rim = x
		}
	}
	if rim < 0 {
		t.Fatal("the fence's middle row holds no fill")
	}

	isFill := func(img *image.RGBA, x, y int) bool { return img.RGBAAt(x, y) == rgba(fill) }
	restGlyphs, glyphs, nearest := 0, 0, pad
	for y := 0; y < codeOverflowSize.Y; y++ {
		// Rows of the wide fence from its top to a padding below its middle
		// row. The bar is drawn lower, in the bottom padding.
		if !isFill(rest, rim, y) {
			continue
		}
		if y >= row+pad {
			break
		}
		for x := rim; x < rim+pad; x++ {
			if !isFill(rest, x, y) {
				restGlyphs++
			}
			if !isFill(scrolled, x, y) {
				glyphs++
				nearest = min(nearest, x-rim)
			}
		}
	}
	if restGlyphs != 0 {
		t.Errorf("at rest %d pixels between the rim and the code are not fill; the padding is drawn over", restGlyphs)
	}
	if glyphs == 0 {
		t.Fatal("scrolled, the band between the rim and the padding holds no glyph pixel; the padding does not move with the code")
	}
	t.Logf("%d glyph pixels in the %d px band, the nearest %d px from the rim", glyphs, pad, nearest)
	if nearest > 1 {
		t.Errorf("scrolled, the nearest glyph pixel is %d px from the rim; the code is clipped short of the rim", nearest)
	}
}

func rgba(c color.NRGBA) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}
