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
// tokens give, at that bar with the reader's "always" setting spent on it,
// and with no bar style at all.
//
// With "always" the thumb is there: the bar stands for as long as the code
// overflows the column. With either of the other two settings it is the
// platform's overlay bar, which a resting frame holds nothing of — so that
// rendering is pixel-identical to the one with no bar style at all, which is
// what "no pixel moves while the bar fades" means. The bar lands in the
// fence's bottom padding, over no code, so the rows the standing bar does
// move span no more than the bar's own width.
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
	if n := golden.PixelDiff(standing, none); n == 0 {
		t.Fatal("with scroll bars always, a resting code block drew no thumb")
	}

	first, last := -1, -1
	for y := range size.Y {
		for x := range size.X {
			if standing.RGBAAt(x, y) != none.RGBAAt(x, y) {
				if first < 0 {
					first = y
				}
				last = y
				break
			}
		}
	}
	if barPx := int(always.CodeScrollbar.Width()); last-first+1 > barPx {
		t.Errorf("the standing bar moved rows %d..%d, %d rows, more than the bar's own %d px; it reached into the code", first, last, last-first+1, barPx)
	}
}
