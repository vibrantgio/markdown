package markdown_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// markSource is three paragraphs of unequal measure. The middle one is the
// shortest, so a wash sized to the block it marks cannot reach as far right
// as the document's widest line does.
const markSource = `A first paragraph, set wide enough to give the document its measure.

Short.

A third paragraph, also wide, so the marked one is not the widest.
`

// markWash is the colour the marking is probed by: the reserved highlighter
// resolved for the light scheme, which is the colour the arrival marking is
// drawn in and is not any of this style's own.
var markWash = tokens.DefaultLight.Highlight

// washBounds returns the bounding box of the pixels painted exactly in
// markWash, and how many there are. Glyph ink over the wash is antialiased
// against it and does not answer, which is what makes the count a measure of
// the field rather than of the text on it.
func washBounds(img *image.RGBA) (image.Rectangle, int) {
	box := image.Rectangle{Min: image.Pt(1<<30, 1<<30), Max: image.Pt(-1, -1)}
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R != markWash.R || c.G != markWash.G || c.B != markWash.B {
				continue
			}
			n++
			box.Min.X = min(box.Min.X, x)
			box.Min.Y = min(box.Min.Y, y)
			box.Max.X = max(box.Max.X, x+1)
			box.Max.Y = max(box.Max.Y, y+1)
		}
	}
	return box, n
}

// inkBounds returns the bounding box of everything drawn over the document's
// background.
func inkBounds(img *image.RGBA, bg color.NRGBA) image.Rectangle {
	box := image.Rectangle{Min: image.Pt(1<<30, 1<<30), Max: image.Pt(-1, -1)}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R == bg.R && c.G == bg.G && c.B == bg.B {
				continue
			}
			box.Min.X = min(box.Min.X, x)
			box.Min.Y = min(box.Min.Y, y)
			box.Max.X = max(box.Max.X, x+1)
			box.Max.Y = max(box.Max.Y, y+1)
		}
	}
	return box
}

// TestHighlightMarksOneBlockAndNothingElse asserts the three properties the
// marking is for: the wash is painted, it is sized to the block it names and
// not to the column, and it changes no pixel outside that block — so a
// document with the marking cleared is the document that was never marked.
func TestHighlightMarksOneBlockAndNothingElse(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte(markSource))
	colors := tokens.DefaultLight
	style := markdown.FromTokens(colors, tokens.DefaultTypography)
	size := image.Pt(560, 200)

	shot := func(mark func(*markdown.Document)) *image.RGBA {
		d := markdown.NewDocument(blocks)
		mark(d)
		return golden.Capture(t, size, themed(d, shaper, style, colors))
	}

	plain := shot(func(*markdown.Document) {})
	marked := shot(func(d *markdown.Document) { d.Highlight(1, markWash) })
	cleared := shot(func(d *markdown.Document) {
		d.Highlight(1, markWash)
		d.ClearHighlight()
	})

	box, n := washBounds(marked)
	if n == 0 {
		t.Fatal("Highlight painted no wash")
	}
	if _, n := washBounds(plain); n != 0 {
		t.Errorf("an unmarked document painted %d wash pixels", n)
	}
	if diff := golden.PixelDiff(plain, cleared); diff != 0 {
		t.Errorf("ClearHighlight left %d pixels changed; the marking is frame state", diff)
	}

	ink := inkBounds(plain, colors.Background)
	if box.Max.X >= ink.Max.X {
		t.Errorf("the wash reaches x=%d, the document's widest line reaches x=%d; "+
			"the marking is sized to the column, not to the block", box.Max.X, ink.Max.X)
	}
	if box.Min.X != ink.Min.X {
		t.Errorf("the wash starts at x=%d, the content at x=%d; the marking must open on the block's own edge",
			box.Min.X, ink.Min.X)
	}

	// Every pixel the marking changed lies inside the wash's own box: the
	// blocks above and below it are untouched.
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			if marked.RGBAAt(x, y) == plain.RGBAAt(x, y) {
				continue
			}
			if !image.Pt(x, y).In(box) {
				t.Fatalf("marking block 1 changed the pixel at (%d,%d), outside the marked block's box %v", x, y, box)
			}
		}
	}
}

// TestHighlightOutsideTheDocumentMarksNothing asserts the two refusals the
// caller relies on when its own state is stale: an index no block has, and a
// wash with no alpha in it.
func TestHighlightOutsideTheDocumentMarksNothing(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte(markSource))
	colors := tokens.DefaultLight
	style := markdown.FromTokens(colors, tokens.DefaultTypography)
	size := image.Pt(560, 200)

	shot := func(mark func(*markdown.Document)) *image.RGBA {
		d := markdown.NewDocument(blocks)
		mark(d)
		return golden.Capture(t, size, themed(d, shaper, style, colors))
	}

	plain := shot(func(*markdown.Document) {})
	cases := []struct {
		name string
		mark func(*markdown.Document)
	}{
		{"past the last block", func(d *markdown.Document) { d.Highlight(len(blocks), markWash) }},
		{"negative", func(d *markdown.Document) { d.Highlight(-1, markWash) }},
		{"transparent wash", func(d *markdown.Document) {
			d.Highlight(1, color.NRGBA{R: markWash.R, G: markWash.G, B: markWash.B})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := golden.PixelDiff(plain, shot(tc.mark)); diff != 0 {
				t.Errorf("%d pixels changed; nothing should have been marked", diff)
			}
		})
	}
}
