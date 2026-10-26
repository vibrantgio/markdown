package markdown_test

// The sideways bar of a table too wide for its column, read at the setting
// that makes it stand. The bar is [markdown.Style.TableScrollbar], which the
// caller hands in already resolved.

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

// overflowingTable is a table whose longest words alone overflow a 420 px frame, so
// the table scrolls sideways.
const overflowingTable = "| first | second | third | fourth |\n" +
	"| --- | --- | --- | --- |\n" +
	"| incomprehensibilities | counterrevolutionaries | uncharacteristically | internationalisation |\n"

// fittingTable fits the same frame at its natural widths.
const fittingTable = "| a | b |\n| --- | --- |\n| 1 | 2 |\n"

// TestTheWideTableDrawsItsSidewaysBar lays a wide table and a table that fits
// out on a resting frame, once with the bar at Always and once with no bar
// style.
//
// At Always the wide table is a scroll area whose bar stands at its own
// bottom edge in a lane of its own. The table is taller by the bar's width.
// Every row above the lane is the row the barless table draws. The thumb is
// drawn inside the lane, a track padding above the table's bottom edge. The
// table that fits draws no bar and takes no lane.
func TestTheWideTableDrawsItsSidewaysBar(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	size := image.Pt(420, 160)
	always := style
	always.TableScrollbar = always.TableScrollbar.Always()
	barless := style
	barless.TableScrollbar = scrollbar.Style{}

	resting := func(src string, st markdown.Style) (*image.RGBA, int) {
		doc := markdown.NewDocument(markdown.Parse([]byte(src)))
		var h int
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Now = time.Unix(1700000000, 0)
			gtx.Constraints.Min = image.Point{}
			dims := doc.Layout(gtx, shaper, st)
			h = dims.Size.Y
			return dims
		})
		return img, h
	}

	standing, standingH := resting(overflowingTable, always)
	none, noneH := resting(overflowingTable, barless)

	barW := int(always.TableScrollbar.Width())
	pad := int(always.TableScrollbar.TrackPadding)
	if got := standingH - noneH; got != barW {
		t.Fatalf("with scroll bars always the wide table grew by %d px; want the bar's lane, %d px", got, barW)
	}

	// bottom is the last row of the image that holds a drawn pixel.
	bottom := func(img *image.RGBA) int {
		last := -1
		for y := range size.Y {
			for x := range size.X {
				if img.RGBAAt(x, y).A != 0 {
					last = y
					break
				}
			}
		}
		return last
	}
	gridBottom := bottom(none)
	if gridBottom < 0 {
		t.Fatal("the barless wide table drew nothing")
	}
	for y := 0; y <= gridBottom; y++ {
		for x := range size.X {
			if standing.RGBAAt(x, y) != none.RGBAAt(x, y) {
				t.Fatalf("with scroll bars always the pixel at (%d,%d), above the lane below row %d, differs from the barless table's; the bar or the lane reached into the grid", x, y, gridBottom)
			}
		}
	}

	// A glyph's edge can take the thumb's colour, so a thumb pixel is one the
	// barless table does not draw the same.
	thumb := color.RGBAModel.Convert(always.TableScrollbar.ThumbColor).(color.RGBA)
	first, last := -1, -1
	for y := range size.Y {
		for x := range size.X {
			if c := standing.RGBAAt(x, y); c == thumb && c != none.RGBAAt(x, y) {
				if first < 0 {
					first = y
				}
				last = y
			}
		}
	}
	if first < 0 {
		t.Fatal("with scroll bars always, a resting wide table drew no thumb")
	}
	if first <= gridBottom {
		t.Errorf("the standing thumb starts at row %d, inside the grid that ends at row %d", first, gridBottom)
	}
	if want := gridBottom + barW - pad; last != want {
		t.Errorf("the standing thumb ends at row %d; want %d, the track's padding above the table's bottom edge", last, want)
	}

	fits, fitsH := resting(fittingTable, always)
	fitsNone, fitsNoneH := resting(fittingTable, barless)
	if fitsH != fitsNoneH {
		t.Errorf("with scroll bars always a table that fits is %d px tall; want %d, no lane", fitsH, fitsNoneH)
	}
	if n := golden.PixelDiff(fits, fitsNone); n != 0 {
		t.Errorf("with scroll bars always a table that fits drew %d pixels the barless table did not; want none", n)
	}
}
