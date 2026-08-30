package markdown_test

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// A marker beside a list item is centred on the first text line, and the
// measurement that says so is taken off the pixels, the way the reference
// reading surface it was matched against was measured: the marker's ink band
// against the text's.
//
// markerProbe says everything in capitals with flat terminals — no ascender
// above them, no descender below, no round letter overshooting either edge —
// so the text's ink band is exactly the cap band the marker claims to be
// centred on, with nothing to interpret away. Each item is one line, so every
// band in the image belongs to one row.
const markerProbe = "- [ ] FLAT INK AT THE LINE\n" +
	"- [x] FLAT INK AT THE LINE\n" +
	"- FLAT INK AT THE LINE\n"

// markerColumn is the width of the probe's marker column: Indent at the
// probe's scale, which is Spacing.S6. The marker lives left of it and the
// text right of it.
const markerColumn = 24

// inkBands returns the vertical extent of every run of rows carrying ink
// within the column [x0, x1), in order, as half-open [top, bottom) intervals.
// Ink is the same luminance departure from the background that the rhythm
// scan uses.
func inkBands(img *image.RGBA, x0, x1 int) [][2]int {
	b := img.Bounds()
	lum := func(x, y int) float64 {
		c := img.RGBAAt(x, y)
		return 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
	}
	bg := lum(b.Max.X-1, b.Min.Y)
	var out [][2]int
	top := -1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		ink := false
		for x := x0; x < x1 && x < b.Max.X && !ink; x++ {
			if d := lum(x, y) - bg; d > 24 || d < -24 {
				ink = true
			}
		}
		switch {
		case ink && top < 0:
			top = y
		case !ink && top >= 0:
			out = append(out, [2]int{top, y})
			top = -1
		}
	}
	if top >= 0 {
		out = append(out, [2]int{top, b.Max.Y})
	}
	return out
}

// center is a band's middle, in half pixels of resolution because a band of
// an even height has its middle between two rows.
func center(band [2]int) float64 { return float64(band[0]+band[1]) / 2 }

// TestAListMarkerCentresOnItsFirstTextLine holds every marker to one rule:
// its ink sits centred on the cap band of the line beside it — the strip from
// the tops of the capitals down to the baseline, which is the band a reader
// sees a line of text occupy and the band the reference centres its own
// checkbox on.
//
// The rule is a claim about the shaped line, so it is measured against the
// shaped line: the marker anchor is derived from the shaper's baseline and
// cap height, and if either ever moves — a face change, a metrics change in
// the shaper, an anchor computed from the text size again — this fails rather
// than drifting quietly. A whole pixel of drift is a whole pixel of the error
// this test exists to prevent, so the tolerance is the half pixel that band
// centres are quantised to.
func TestAListMarkerCentresOnItsFirstTextLine(t *testing.T) {
	shaper := defaultShaper(t)
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography)
			d := markdown.NewDocument(markdown.Parse([]byte(markerProbe)))
			img := golden.Capture(t, image.Pt(320, 120), func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, tc.colors.Background,
					clip.Rect{Max: gtx.Constraints.Max}.Op())
				return d.LayoutColumn(gtx, shaper, style)
			})

			markers := inkBands(img, 0, markerColumn)
			lines := inkBands(img, markerColumn, img.Bounds().Max.X)
			if len(markers) != 3 || len(lines) != 3 {
				t.Fatalf("scanned %d marker bands and %d text bands, want 3 of each (one per item): %v / %v; the probe or the scan has drifted", len(markers), len(lines), markers, lines)
			}
			for i, kind := range []string{"unchecked checkbox", "checked checkbox", "bullet"} {
				off := center(markers[i]) - center(lines[i])
				if off < -0.5 || off > 0.5 {
					t.Errorf("the %s's ink centres %+.1f px from its text line's (marker %v, text %v); a marker hangs from the shaped line's cap band, not above it", kind, off, markers[i], lines[i])
				}
			}
		})
	}
}

// TestInlineCodeLeavesTheLineTheBodysHeight is the guard on the seam this
// package keeps having to repair: a line of prose is as tall as the body face
// asks for, and a word quoted into it in another face does not get to say
// otherwise.
//
// Set at the paragraph's own size the monospace face asks for more ascent
// than the body face does, and since every segment on a line shares one
// baseline, that ascent pushes the whole line's baseline down — out from
// under every marker hung beside it from the body line's geometry. Sizing
// inline code below the prose is what keeps the two agreeing, and this
// measures the agreement rather than the size: any future change that lets a
// code span reach past the body face's ascent fails here.
//
// The one-word line is the control. Without it a probe that wrapped in one
// case and not the other could report two equal heights that are equally
// wrong.
func TestInlineCodeLeavesTheLineTheBodysHeight(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	oneWord := columnHeight(t, shaper, style, "moves\n")
	plain := columnHeight(t, shaper, style, "moves the file contents over\n")
	code := columnHeight(t, shaper, style, "`git mv` the file contents over\n")
	if plain != oneWord {
		t.Fatalf("the plain probe measures %d px against a single line's %d; it wrapped, so the comparison below says nothing", plain, oneWord)
	}
	if code != plain {
		t.Errorf("a line holding inline code measures %d px against a plain line's %d; inline code is sized to sit inside the body line, not to stretch it", code, plain)
	}
}

// codeMarkerProbe is [markerProbe]'s task row with a code span opening it —
// the shape that would ride a checkbox high beside a row beginning in
// backticks. Its plain twin runs directly above it, so one
// render carries both and the two offsets are measured under identical
// conditions.
//
// Everything is capitals here for the same reason [markerProbe] is: the ink
// band a row measures is then exactly its cap band. The code span's capitals
// are shorter than the body's and sit inside them, so both rows' bands are
// the body cap band and the comparison is of marker positions alone.
const codeMarkerProbe = "- [x] FLAT INK AT THE LINE\n" +
	"- [x] `GIT MV` FLAT INK AT THE LINE\n"

// TestAMarkerHangsLevelBesideACodeOpeningRow measures what the owner sees: a
// task row opening with inline code carries its checkbox at the same height
// as a row of plain prose. It is the pixel half of
// [TestInlineCodeLeavesTheLineTheBodysHeight] — that one pins the line box,
// this one pins where the marker lands beside it — and it fails if a code
// span ever drags a row's baseline out from under the marker column again.
func TestAMarkerHangsLevelBesideACodeOpeningRow(t *testing.T) {
	shaper := defaultShaper(t)
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography)
			d := markdown.NewDocument(markdown.Parse([]byte(codeMarkerProbe)))
			img := golden.Capture(t, image.Pt(360, 80), func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, tc.colors.Background,
					clip.Rect{Max: gtx.Constraints.Max}.Op())
				return d.LayoutColumn(gtx, shaper, style)
			})

			markers := inkBands(img, 0, markerColumn)
			lines := inkBands(img, markerColumn, img.Bounds().Max.X)
			if len(markers) != 2 || len(lines) != 2 {
				t.Fatalf("scanned %d marker bands and %d text bands, want 2 of each (one per row): %v / %v; the probe or the scan has drifted", len(markers), len(lines), markers, lines)
			}
			plain := center(markers[0]) - center(lines[0])
			code := center(markers[1]) - center(lines[1])
			if d := code - plain; d < -0.5 || d > 0.5 {
				t.Errorf("the checkbox on the code-opening row sits %+.1f px from its text against the plain row's %+.1f — a gap of %+.1f px; a row that opens in code is a row of the body", code, plain, d)
			}
		})
	}
}
