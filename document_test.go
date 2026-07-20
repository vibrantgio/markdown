package markdown_test

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/font/gofont"
	gioinput "gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/markdown"
	golden "github.com/vibrantgio/markdown/internal/golden"
	"github.com/vibrantgio/prism/tokens"
)

func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
}

// themed wraps a document in a Background-filled widget so goldens capture
// the document on its token background.
func themed(d *markdown.Document, shaper *text.Shaper, style markdown.Style, c tokens.ColorTokens) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, c.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return d.Layout(gtx, shaper, style)
		})
	}
}

// ---- Golden-image tests ----

// TestCorpusDocumentGolden records or diffs the corpus document — every G6.2
// construct — in light and dark token themes.
func TestCorpusDocumentGolden(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse(corpus(t))
	size := image.Pt(560, 1500)
	cases := []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"corpus-light", tokens.DefaultLight},
		{"corpus-dark", tokens.DefaultDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypeScale)
			d := markdown.NewDocument(blocks)
			golden.Render(t, tc.name, size, themed(d, shaper, style, tc.colors))
		})
	}
}

// TestTableDocumentGolden records or diffs a GFM table — emphasised header
// row on its surface, token borders, and left/centre/right column alignment —
// in light and dark token themes.
func TestTableDocumentGolden(t *testing.T) {
	shaper := defaultShaper(t)
	src := "| Package | Role | Stars |\n" +
		"|:--------|:----:|------:|\n" +
		"| `prism` | primitives | 1200 |\n" +
		"| **markdown** | document rendering | 87 |\n" +
		"| cadence | patterns | 5 |\n"
	blocks := markdown.Parse([]byte(src))
	size := image.Pt(560, 180)
	cases := []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"table-light", tokens.DefaultLight},
		{"table-dark", tokens.DefaultDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypeScale)
			d := markdown.NewDocument(blocks)
			golden.Render(t, tc.name, size, themed(d, shaper, style, tc.colors))
		})
	}
}

// TestScrolledDocumentGolden records or diffs the corpus scrolled to the task
// list, proving the prism/list viewport renders later blocks.
func TestScrolledDocumentGolden(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse(corpus(t))
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypeScale)
	d := markdown.NewDocumentAt(blocks, 9)
	golden.Render(t, "corpus-scrolled", image.Pt(560, 420),
		themed(d, shaper, style, tokens.DefaultLight))
}

// ---- Layout tests ----

func measureDoc(shaper *text.Shaper, style markdown.Style, blocks []markdown.Block, size image.Point) layout.Dimensions {
	var ops op.Ops
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: size},
		Ops:         &ops,
	}
	return markdown.NewDocument(blocks).Layout(gtx, shaper, style)
}

// TestLayoutColumnNaturalHeight verifies LayoutColumn takes exactly its
// content's height — no viewport filling, no internal scrolling: more blocks
// lay out strictly taller, and the height is independent of the vertical
// constraint.
func TestLayoutColumnNaturalHeight(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypeScale)
	one := markdown.Parse([]byte("alpha\n"))
	three := markdown.Parse([]byte("alpha\n\nbravo\n\n```\ncode\n```\n"))

	column := func(blocks []markdown.Block, size image.Point) layout.Dimensions {
		var ops op.Ops
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: size},
			Ops:         &ops,
		}
		return markdown.NewDocument(blocks).LayoutColumn(gtx, shaper, style)
	}

	oneH := column(one, image.Pt(560, 10_000)).Size.Y
	threeH := column(three, image.Pt(560, 10_000)).Size.Y
	if oneH == 0 || threeH <= oneH {
		t.Errorf("column heights one=%d three=%d; want 0 < one < three (natural content height)", oneH, threeH)
	}
	if short := column(three, image.Pt(560, 40)).Size.Y; short != threeH {
		t.Errorf("column height %d under a short constraint != unconstrained height %d; LayoutColumn must not fit a viewport", short, threeH)
	}
}

// TestCodeBlockOverflowScrolls verifies a code block never wraps or exceeds
// its constraint: an over-wide line keeps the block inside the narrow width,
// and the height matches the wide layout (same line count — the overflow
// scrolls horizontally instead of wrapping).
func TestCodeBlockOverflowScrolls(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypeScale)
	blocks := markdown.Parse([]byte("```\nthe first line is much much much much much wider than the narrow viewport\nshort line\n```\n"))

	wide := measureDoc(shaper, style, blocks, image.Pt(2000, 1000))
	narrow := measureDoc(shaper, style, blocks, image.Pt(240, 1000))

	if narrow.Size.X > 240 {
		t.Errorf("narrow code block width %d exceeds constraint 240", narrow.Size.X)
	}
	if narrow.Size.Y != wide.Size.Y {
		t.Errorf("narrow code block height %d != wide height %d; over-wide code must scroll, not wrap", narrow.Size.Y, wide.Size.Y)
	}
}

// TestNestedListIndents verifies each nesting level shifts content by the
// Indent column: three levels lay out strictly wider than one when width is
// unconstrained.
func TestNestedListIndents(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypeScale)

	flat := markdown.Parse([]byte("- alpha\n"))
	nested := markdown.Parse([]byte("- alpha\n  - alpha\n    - alpha\n"))

	fw := measureDoc(shaper, style, flat, image.Pt(2000, 1000)).Size.X
	nw := measureDoc(shaper, style, nested, image.Pt(2000, 1000)).Size.X
	if nw < fw+2*int(style.Indent) {
		t.Errorf("nested list width %d not at least two indents past flat width %d (indent %d)", nw, fw, int(style.Indent))
	}
}

// memProvider is an in-memory ImageProvider serving images by URL.
type memProvider map[string]image.Image

func (p memProvider) Image(url string) (image.Image, error) {
	img, ok := p[url]
	if !ok {
		return nil, fmt.Errorf("no image for %q", url)
	}
	return img, nil
}

// TestImageProvider lays out image blocks through an in-memory provider: a
// fitting image takes its natural height, an over-wide image is scaled down
// to the width constraint, and a URL the provider cannot serve falls back to
// the alt-text paragraph.
func TestImageProvider(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypeScale)
	style.Images = memProvider{
		"logo.png": image.NewNRGBA(image.Rect(0, 0, 48, 100)),
		"wide.png": image.NewNRGBA(image.Rect(0, 0, 2000, 100)),
	}

	fits := measureDoc(shaper, style, markdown.Parse([]byte("![the logo](logo.png)\n")), image.Pt(560, 400))
	if fits.Size.Y < 100 {
		t.Errorf("fitting image document height %d < image height 100", fits.Size.Y)
	}

	wide := measureDoc(shaper, style, markdown.Parse([]byte("![panorama](wide.png)\n")), image.Pt(560, 400))
	if wide.Size.Y >= 100 {
		t.Errorf("over-wide image document height %d not scaled down below 100", wide.Size.Y)
	}

	missing := measureDoc(shaper, style, markdown.Parse([]byte("![absent art](gone.png)\n")), image.Pt(560, 400))
	if missing.Size.Y == 0 {
		t.Error("missing image laid out with zero height; want alt-text fallback")
	}
	if missing.Size.Y >= fits.Size.Y {
		t.Errorf("alt-text fallback height %d not below image height %d", missing.Size.Y, fits.Size.Y)
	}
}

// TestDocumentLiveFrame drives Document.Layout through an input router for
// two frames, exercising the live richtext path (link registration and event
// draining) over the full corpus without a GPU.
func TestDocumentLiveFrame(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypeScale)
	d := markdown.NewDocument(markdown.Parse(corpus(t)))

	r := new(gioinput.Router)
	var ops op.Ops
	for range 2 {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(560, 3000)},
			Ops:         &ops,
			Source:      r.Source(),
		}
		if dims := d.Layout(gtx, shaper, style); dims.Size.Y == 0 {
			t.Fatal("document laid out with zero height")
		}
		r.Frame(&ops)
	}
}

// ---- Token defaults ----

// TestFromTokensDefaults pins the FromTokens contract: heading levels step
// down the type scale, code sits on SurfaceVariant, the quote bar is Primary
// with OnSurfaceVariant text, and rules use Outline.
func TestFromTokensDefaults(t *testing.T) {
	c, ts := tokens.DefaultLight, tokens.DefaultTypeScale
	st := markdown.FromTokens(c, ts)

	wantSizes := [6]unit.Sp{
		unit.Sp(ts.HeadlineLarge), unit.Sp(ts.HeadlineMedium), unit.Sp(ts.HeadlineSmall),
		unit.Sp(ts.TitleLarge), unit.Sp(ts.TitleMedium), unit.Sp(ts.TitleSmall),
	}
	if st.HeadingSizes != wantSizes {
		t.Errorf("HeadingSizes = %v, want %v", st.HeadingSizes, wantSizes)
	}
	if st.Text.Color != c.OnBackground || st.Text.LinkColor != c.Primary {
		t.Errorf("Text colours = %v/%v, want OnBackground/Primary", st.Text.Color, st.Text.LinkColor)
	}
	if st.CodeBackground != c.SurfaceVariant || st.CodeColor != c.OnSurfaceVariant {
		t.Errorf("code colours = %v/%v, want SurfaceVariant/OnSurfaceVariant", st.CodeBackground, st.CodeColor)
	}
	if st.QuoteBar != c.Primary || st.QuoteColor != c.OnSurfaceVariant {
		t.Errorf("quote colours = %v/%v, want Primary/OnSurfaceVariant", st.QuoteBar, st.QuoteColor)
	}
	if st.RuleColor != c.Outline {
		t.Errorf("RuleColor = %v, want Outline %v", st.RuleColor, c.Outline)
	}
	if st.CodeSize != unit.Sp(ts.BodyMedium) {
		t.Errorf("CodeSize = %v, want BodyMedium %v", st.CodeSize, ts.BodyMedium)
	}
}
