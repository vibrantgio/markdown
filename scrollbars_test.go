package markdown_test

// The code block's own horizontal bar, read at the two settings the platform
// answers with. The bar is [markdown.Style.CodeScrollbar], which the caller
// hands in already resolved; an application spends the platform's "Show
// scroll bars" setting on it before the document is drawn, and what that
// spending comes to is a bar standing on a resting frame or no bar at all,
// which is what these pixels are.

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/layout"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// overflowingFence is a fence whose one line runs well past any column the
// test lays it out in, so the block scrolls and has a bar to draw.
const overflowingFence = "```go\n" +
	"func main() { println(\"a line far too long for the column it is set in, and then some more of it\") }\n" +
	"```\n"

// TestTheCodeBlockSpendsTheScrollBarSetting lays an overflowing fence out on
// a frame nothing has scrolled and nothing is pointing at, at the bar the
// tokens give, at that bar with the reader's "always" setting applied, and
// with no bar style at all.
//
// With either setting but "always" the bar is the platform's overlay bar,
// which a resting frame holds nothing of, and it takes no lane: that
// rendering is pixel-identical to the one with no bar style at all, the code's
// padding reaching the block's bottom edge.
//
// With "always" the bar stands for as long as the code overflows. The block
// is the region that scrolls, so the bar is drawn against the block's own
// bottom edge, inside its rim, from the block's leading edge rather than
// inset by the padding, and it takes a lane of its own: the block is the
// bar's width taller, and every row above the lane is the row the barless
// block draws, so the code and its padding stop short of the lane.
func TestTheCodeBlockSpendsTheScrollBarSetting(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	size := image.Pt(420, 120)
	blocks := markdown.Parse([]byte(overflowingFence))

	resting := func(st markdown.Style) *image.RGBA {
		doc := markdown.NewDocument(blocks)
		w := themed(doc, shaper, st, tokens.PlatformLight)
		return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Now = time.Unix(1700000000, 0)
			return w(gtx)
		})
	}

	always := style
	always.CodeScrollbar = always.CodeScrollbar.Always()
	barless := style
	barless.CodeScrollbar = scrollbar.Style{}

	standing := resting(always)
	fading := resting(style)
	none := resting(barless)

	if n := golden.PixelDiff(fading, none); n != 0 {
		t.Errorf("a resting code block drew %d pixels the same block without a bar style did not; want none", n)
	}

	page := color.RGBAModel.Convert(tokens.PlatformLight.TextBackground).(color.RGBA)
	// bottom is the last row of the block's box down the image's middle
	// column, its rim included.
	bottom := func(img *image.RGBA) int {
		last := -1
		for y := range size.Y {
			if img.RGBAAt(size.X/2, y) != page {
				last = y
			}
		}
		return last
	}
	barW := int(always.CodeScrollbar.Width())
	pad := int(always.CodeScrollbar.TrackPadding)
	rim := 0
	if style.CodeBorder.A > 0 {
		rim = 1
	}

	if got, want := bottom(standing)-bottom(none), barW; got != want {
		t.Fatalf("with scroll bars always the block grew by %d px; want the bar's lane, %d px", got, want)
	}
	laneTop := bottom(standing) - rim + 1 - barW
	blockLead, blockTrail := size.X, -1
	for x := range size.X {
		if standing.RGBAAt(x, laneTop) != page {
			blockLead = min(blockLead, x)
			blockTrail = x
		}
	}
	// The barless block's rounded bottom corners are drawn above the lane,
	// where the standing block continues straight down, so the columns the
	// corners take are left out of the comparison.
	corner := int(tokens.Radius.Base)
	for y := range laneTop {
		for x := blockLead + corner; x <= blockTrail-corner; x++ {
			if a, b := standing.RGBAAt(x, y), none.RGBAAt(x, y); channelGap(a, b) > glyphGap {
				t.Fatalf("with scroll bars always the pixel at (%d,%d), above the lane at row %d, is %v against the barless block's %v; the bar or the lane reached into the code", x, y, laneTop, a, b)
			}
		}
	}

	// A glyph's edge can take the thumb's colour, so a thumb pixel is one the
	// barless block does not draw the same.
	thumb := color.RGBAModel.Convert(always.CodeScrollbar.ThumbColor).(color.RGBA)
	first, last, lead := -1, -1, size.X
	for y := range size.Y {
		for x := range size.X {
			if c := standing.RGBAAt(x, y); c == thumb && c != none.RGBAAt(x, y) {
				if first < 0 {
					first = y
				}
				last = y
				lead = min(lead, x)
			}
		}
	}
	if first < 0 {
		t.Fatal("with scroll bars always, a resting code block drew no thumb")
	}
	if first < laneTop {
		t.Errorf("the standing thumb starts at row %d, above the lane at row %d", first, laneTop)
	}
	if want := bottom(standing) - rim - pad; last != want {
		t.Errorf("the standing thumb ends at row %d; want %d, the track's padding above the block's bottom edge inside its rim", last, want)
	}
	// The thumb's leading corners are rounded, so its first opaque column
	// is a pixel or two past the track's start.
	if from := lead - blockLead; from < rim+pad || from > rim+pad+int(always.CodeScrollbar.ThumbCornerRadius) {
		t.Errorf("the standing thumb starts %d px from the block's leading edge; want the rim and the track's padding, %d px, so the bar is not inset by the block's padding", from, rim+pad)
	}
}

// glyphGap is the channel gap the two renders may show on a glyph's edge
// pixels. The two renders composite the same glyph one level apart in a
// channel, measured on Roboto Mono at 13 px; a bar or a lane is a gap of
// tens of levels.
const glyphGap = 2

// channelGap is the largest difference between a and b in one channel.
func channelGap(a, b color.RGBA) int {
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	return max(d(a.R, b.R), d(a.G, b.G), d(a.B, b.B), d(a.A, b.A))
}
