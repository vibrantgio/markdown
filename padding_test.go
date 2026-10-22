package markdown_test

import (
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// paddingDp is the padding the padding tests keep on both sides of the text,
// set as the style's Gutter.
const paddingDp = 24

// paddedStyle is the token style at measure m with the padding set.
func paddedStyle(c tokens.PlatformColors, m unit.Dp) markdown.Style {
	style := markdown.FromTokens(c, tokens.DefaultTypography, color.NRGBA{})
	style.Measure = m
	style.Gutter = paddingDp
	return style
}

// barred lays the document out like themed does, with a standing bar that
// occupies its own lane at the trailing edge.
func barred(d *markdown.Document, style markdown.Style, c tokens.PlatformColors) layout.Widget {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bar := scrollbar.FromTokens(c, c.TextBackground).Always()
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, c.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(measurePage).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return d.LayoutScrollbar(gtx, shaper, style, bar, list.Occupy)
		})
	}
}

// TestAWideViewportKeepsItsPixels holds the wide case to the images recorded
// before the padding was kept below the measure: a viewport with room for the
// measure and the padding on both sides centres the measure as it always has.
// The edge case is the widest measure that still fits, where centring leaves
// exactly the padding on each side.
func TestAWideViewportKeepsItsPixels(t *testing.T) {
	shaper := defaultShaper(t)
	c := tokens.PlatformLight
	blocks := markdown.Parse([]byte(measureSource))
	edge := unit.Dp(measureRegion - 2*paddingDp)
	golden.Render(t, "measure-padded-wide", measureWideSize,
		themed(markdown.NewDocument(blocks), shaper, paddedStyle(c, measureWidth), c))
	golden.Render(t, "measure-padded-edge", measureWideSize,
		themed(markdown.NewDocument(blocks), shaper, paddedStyle(c, edge), c))
	golden.Render(t, "measure-padded-wide-bar", measureWideSize,
		barred(markdown.NewDocument(blocks), paddedStyle(c, measureWidth), c))
}

// TestANarrowViewportKeepsThePaddingOnBothSides lays a document whose first
// block is a code block — its fill spans the block's whole width — into
// regions narrower than the measure plus the padding on both sides. The block
// keeps the padding at its leading edge, and its width is the region less
// twice the padding, less the bar's lane first when a bar occupies one. The
// band just below the edge case, where the measure would fit beside one
// padding but not two, is the one that used to give up the leading padding.
func TestANarrowViewportKeepsThePaddingOnBothSides(t *testing.T) {
	c := tokens.PlatformLight
	blocks := markdown.Parse([]byte("```\ncode\n```\n"))
	lane := int(scrollbar.FromTokens(c, c.TextBackground).Width())
	shaper := defaultShaper(t)
	for _, tc := range []struct {
		name    string
		measure unit.Dp
		bar     bool
	}{
		{"one padding short", unit.Dp(measureRegion - paddingDp - 2), false},
		{"far below", 2 * unit.Dp(measureRegion), false},
		{"one padding short with a bar", unit.Dp(measureRegion - lane - paddingDp - 2), true},
		{"far below with a bar", 2 * unit.Dp(measureRegion), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			style := paddedStyle(c, tc.measure)
			w := themed(markdown.NewDocument(blocks), shaper, style, c)
			want := measureRegion - 2*paddingDp
			if tc.bar {
				w = barred(markdown.NewDocument(blocks), style, c)
				want -= lane
			}
			img := golden.Capture(t, measureWideSize, w)
			lead, trail, ok := pageExtent(img, c.TextBackground)
			if !ok {
				t.Fatal("the code block drew no fill")
			}
			if got := lead - measurePage; got != paddingDp {
				t.Errorf("the first block leads %d px into the region, want the padding %d", got, paddingDp)
			}
			if got := trail - lead + 1; got != want {
				t.Errorf("the first block is %d px wide, want the region less twice the padding: %d", got, want)
			}
		})
	}
}
