package markdown_test

import (
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
