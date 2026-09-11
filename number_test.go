package markdown_test

import (
	"image"
	stdcolor "image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/paragraph"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// numberProbeText is what every item in this file's probes says. One word, in
// capitals with flat terminals: the widest space between two runs of drawn
// pixels in a row is then the space between the item's number and its content,
// with no space between words to rival it.
const numberProbeText = "FLAT"

// drawnRuns returns the horizontal extent of every run of columns carrying
// drawn pixels, in order, as half-open [left, right) intervals. A column
// carries pixels at the same luminance departure from the background that
// [drawnBands] reads down the rows.
func drawnRuns(img *image.RGBA, y0, y1 int) [][2]int {
	b := img.Bounds()
	lum := func(x, y int) float64 {
		c := img.RGBAAt(x, y)
		return 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
	}
	bg := lum(b.Max.X-1, b.Min.Y)
	var out [][2]int
	left := -1
	for x := b.Min.X; x < b.Max.X; x++ {
		drawn := false
		for y := max(y0, b.Min.Y); y < y1 && y < b.Max.Y && !drawn; y++ {
			if d := lum(x, y) - bg; d > 24 || d < -24 {
				drawn = true
			}
		}
		switch {
		case drawn && left < 0:
			left = x
		case !drawn && left >= 0:
			out = append(out, [2]int{left, x})
			left = -1
		}
	}
	if left >= 0 {
		out = append(out, [2]int{left, b.Max.X})
	}
	return out
}

// contentEdge is where the item's content begins in a row of one-word items:
// the first run of drawn pixels after the widest space, which is the space the
// number keeps between itself and the word.
func contentEdge(runs [][2]int) int {
	widest, at := -1, -1
	for i := 1; i < len(runs); i++ {
		if gap := runs[i][0] - runs[i-1][1]; gap > widest {
			widest, at = gap, i
		}
	}
	if at < 0 {
		return -1
	}
	return runs[at][0]
}

// renderList captures a document over a filled background, so the scans above
// read one document's pixels against a colour they know.
func renderList(t *testing.T, size image.Point, colors tokens.PlatformColors, style markdown.Style, src string) *image.RGBA {
	t.Helper()
	shaper := defaultShaper(t)
	d := markdown.NewDocument(markdown.Parse([]byte(src)))
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, colors.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return d.LayoutColumn(gtx, shaper, style)
	})
}

// TestALongNumberPaintsWholeBesideItsItem is the owner's defect, measured off
// the pixels: a list numbered from 291 sets each number in full — every digit
// and the period after it — and its content begins clear of it.
//
// The number is compared against the same number set on its own in the same
// style: run for run, at the same x. A number given a column too narrow for it
// loses what does not fit — the period first — and the digits that remain
// paint on over the words, which is one run short here, or one run landing
// where the reference has none.
func TestALongNumberPaintsWholeBesideItsItem(t *testing.T) {
	shaper := defaultShaper(t)
	for _, tc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"light", tokens.PlatformLight},
		{"dark", tokens.PlatformDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography, stdcolor.NRGBA{})
			size := image.Pt(360, 40)
			alone := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, tc.colors.TextBackground,
					clip.Rect{Max: gtx.Constraints.Max}.Op())
				return paragraph.Render(shaper, style.Text,
					[]paragraph.SpanStyle{{Content: "291."}}, paragraph.Idle())(gtx)
			})
			want := drawnRuns(alone, 0, size.Y)
			if len(want) < 3 {
				t.Fatalf("the number 291. set on its own scans as %d runs of pixels, too few for the digits and the period after them: %v", len(want), want)
			}

			img := renderList(t, size, tc.colors, style, "291. "+numberProbeText+"\n")
			got := drawnRuns(img, 0, size.Y)
			if len(got) <= len(want) {
				t.Fatalf("the item scans as %d runs of pixels against the %d its number alone draws: %v against %v; the item's number and its word both draw, so there are more", len(got), len(want), got, want)
			}
			for i, w := range want {
				if got[i] != w {
					t.Errorf("run %d of the item's pixels spans %v where the number set on its own spans %v; the number stands in a column of its own width, so it paints whole and unmoved", i, got[i], w)
				}
			}
			if gap := got[len(want)][0] - want[len(want)-1][1]; gap < 4 {
				t.Errorf("the item's word begins %d px after its number's last pixels; the content stands clear of the number, not against it", gap)
			}
		})
	}
}

// TestEveryItemOfAListSharesItsNumberColumn holds the column to the list
// rather than to the item: numbers of three and of four digits stand in one
// column, so the reader follows a single content edge down the list.
func TestEveryItemOfAListSharesItsNumberColumn(t *testing.T) {
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, stdcolor.NRGBA{})
	src := "998. " + numberProbeText + "\n999. " + numberProbeText + "\n1000. " + numberProbeText + "\n"
	img := renderList(t, image.Pt(360, 100), tokens.PlatformLight, style, src)
	bands := drawnBands(img, 0, img.Bounds().Max.X)
	if len(bands) != 3 {
		t.Fatalf("scanned %d rows of drawn pixels, want the 3 items of the probe: %v", len(bands), bands)
	}
	edges := make([]int, len(bands))
	for i, band := range bands {
		edges[i] = contentEdge(drawnRuns(img, band[0], band[1]))
	}
	for i, e := range edges {
		if e < 0 {
			t.Fatalf("item %d scans as one run of pixels, so the probe or the scan has drifted", i)
		}
		if e != edges[0] {
			t.Errorf("item %d sets its content at x=%d against the first item's x=%d; every item of one list shares its number column", i, e, edges[0])
		}
	}
}

// TestASingleDigitListKeepsTheIndentColumn pins the other side of the rule:
// the column grows only when a number needs it, so a list numbered from 1 sets
// its content exactly where a bulleted list does — at [markdown.Style.Indent].
func TestASingleDigitListKeepsTheIndentColumn(t *testing.T) {
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, stdcolor.NRGBA{})
	size := image.Pt(360, 40)
	numbered := renderList(t, size, tokens.PlatformLight, style, "1. "+numberProbeText+"\n")
	bulleted := renderList(t, size, tokens.PlatformLight, style, "- "+numberProbeText+"\n")
	got := contentEdge(drawnRuns(numbered, 0, size.Y))
	want := contentEdge(drawnRuns(bulleted, 0, size.Y))
	if got < 0 || want < 0 {
		t.Fatalf("a probe scans as one run of pixels (numbered %d, bulleted %d); the probe or the scan has drifted", got, want)
	}
	if got != want {
		t.Errorf("a list numbered from 1 sets its content at x=%d against a bulleted list's x=%d; a number that fits the indent leaves the column alone", got, want)
	}
}
