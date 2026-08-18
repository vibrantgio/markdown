package markdown_test

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// Most of the spacing tests are transitions, not pictures: each measures what
// the layout puts between two adjacent blocks by differencing heights — the
// column holding both, less what each block takes on its own — so the numbers
// they compare are the inserted space itself, free of the shaped text's own
// height. They pin the proportions, which is what a rhythm is.
//
// The last one measures the other quantity: not the space the layout inserts
// but the blank the reader sees, scanned out of a rendered document the way
// the reference the rhythm was matched to was measured. That is the one that
// can be held against a number taken from another application.

const spacingProse = "Paragraph enjoying jumping typography glyphs."

// columnHeight lays src out as a natural-height column and returns its height.
func columnHeight(t *testing.T, shaper *text.Shaper, style markdown.Style, src string) int {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(560, 10_000)},
		Ops:         &ops,
	}
	return markdown.NewDocument(markdown.Parse([]byte(src))).LayoutColumn(gtx, shaper, style).Size.Y
}

// listHeight lays src out through the scrolling list path, in a viewport tall
// enough to hold the whole document, and returns the height it reports.
func listHeight(t *testing.T, shaper *text.Shaper, style markdown.Style, src string) int {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(560, 10_000)},
		Ops:         &ops,
	}
	return markdown.NewDocument(markdown.Parse([]byte(src))).Layout(gtx, shaper, style).Size.Y
}

// gapBetween is the space the layout inserts between two adjacent blocks.
func gapBetween(t *testing.T, shaper *text.Shaper, style markdown.Style, first, second string) int {
	t.Helper()
	both := columnHeight(t, shaper, style, first+"\n\n"+second+"\n")
	return both - columnHeight(t, shaper, style, first+"\n") - columnHeight(t, shaper, style, second+"\n")
}

// heading returns the source of a heading at the given level.
func heading(level int) string {
	return fmt.Sprintf("%s Heading at level %d", "######"[:level], level)
}

// TestAHeadingKeepsMoreSpaceAboveThanBelow is the proximity rule the whole
// pass is about: a heading separates from the section it closes and clings to
// the one it opens, so the space above it is wider than an ordinary block gap
// and the space below it is narrower — and above is at least twice below.
func TestAHeadingKeepsMoreSpaceAboveThanBelow(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)

	ordinary := gapBetween(t, shaper, style, spacingProse, spacingProse)
	for level := 1; level <= 6; level++ {
		above := gapBetween(t, shaper, style, spacingProse, heading(level))
		below := gapBetween(t, shaper, style, heading(level), spacingProse)
		switch {
		case above <= ordinary:
			t.Errorf("level %d: %d px above the heading, %d px between ordinary blocks; a heading must open more space than a paragraph", level, above, ordinary)
		case below >= ordinary:
			t.Errorf("level %d: %d px below the heading, %d px between ordinary blocks; a heading must sit closer to what it introduces", level, below, ordinary)
		case above < 2*below:
			t.Errorf("level %d: %d px above the heading against %d px below; want the space above at least twice the space below", level, above, below)
		}
	}
}

// TestDeeperHeadingsEarnLessSpace: the space is derived from the type scale,
// so it falls with the level — never rising as the headings get smaller, and
// strictly smaller at level six than at level one on both sides.
func TestDeeperHeadingsEarnLessSpace(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)

	var above, below [7]int
	for level := 1; level <= 6; level++ {
		above[level] = gapBetween(t, shaper, style, spacingProse, heading(level))
		below[level] = gapBetween(t, shaper, style, heading(level), spacingProse)
	}
	for level := 2; level <= 6; level++ {
		if above[level] > above[level-1] {
			t.Errorf("level %d takes %d px above, level %d takes %d px; a deeper heading must not take more", level, above[level], level-1, above[level-1])
		}
		if below[level] > below[level-1] {
			t.Errorf("level %d takes %d px below, level %d takes %d px; a deeper heading must not take more", level, below[level], level-1, below[level-1])
		}
	}
	if above[6] >= above[1] || below[6] >= below[1] {
		t.Errorf("level 6 takes %d/%d px above/below against level 1's %d/%d; the levels must differ", above[6], below[6], above[1], below[1])
	}
}

// TestTheFirstBlockTakesNoSpaceAbove: a document opening on a heading has
// nothing above it to separate from, so the heading starts at the very top of
// the scrolling column — the height the list reports is the content plus the
// heading's own trailing space and nothing else.
func TestTheFirstBlockTakesNoSpaceAbove(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)

	title := heading(1)
	below := gapBetween(t, shaper, style, title, spacingProse)
	content := columnHeight(t, shaper, style, title+"\n")
	if got, want := listHeight(t, shaper, style, title+"\n"), content+below; got != want {
		t.Errorf("a document opening on a heading lays out %d px tall, want %d (the heading plus its %d px trailing space); the leading %d px belong above nothing", got, want, below, got-want)
	}

	// The same heading with a paragraph before it does take its space above.
	if opened := gapBetween(t, shaper, style, spacingProse, title); opened <= 0 {
		t.Errorf("a heading after a paragraph opened %d px above it; the suppression is for the first block only", opened)
	}
}

// TestStackedHeadingsCloseUp: two headings in a row are one announcement, not
// two sections. The pair closes from both sides — the lower heading opens no
// space of its own and the upper one closes with less than it would over
// prose — so the space inside the pair is the tightest on the page: under an
// ordinary block gap, and well under what the same heading takes over a
// paragraph.
func TestStackedHeadingsCloseUp(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)

	pair := gapBetween(t, shaper, style, heading(2), heading(3))
	overProse := gapBetween(t, shaper, style, heading(2), spacingProse)
	underProse := gapBetween(t, shaper, style, spacingProse, heading(3))
	ordinary := gapBetween(t, shaper, style, spacingProse, spacingProse)
	switch {
	case pair >= underProse:
		t.Errorf("a heading under a heading took %d px above it, one under a paragraph %d px; the stacked pair must close up", pair, underProse)
	case pair >= overProse:
		t.Errorf("a heading over a heading closed with %d px, one over a paragraph %d px; the pair must be the tighter of the two", pair, overProse)
	case pair >= ordinary:
		t.Errorf("a heading under a heading took %d px above it against a %d px ordinary block gap; the pair must read as one announcement", pair, ordinary)
	case pair <= 0:
		t.Errorf("a heading under a heading took %d px above it; the pair is still two blocks", pair)
	}
}

// TestAStyleWithoutHeadingSpaceKeepsTheOldRhythm: the fields are the only
// thing that moves headings, so a Style built by hand — every heading space
// left zero — spaces every pair of blocks by BlockGap, exactly as a document
// laid out before headings had space of their own.
func TestAStyleWithoutHeadingSpaceKeepsTheOldRhythm(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	style.HeadingSpaceAbove = [6]unit.Dp{}
	style.HeadingSpaceBelow = [6]unit.Dp{}

	ordinary := gapBetween(t, shaper, style, spacingProse, spacingProse)
	for level := 1; level <= 6; level++ {
		above := gapBetween(t, shaper, style, spacingProse, heading(level))
		below := gapBetween(t, shaper, style, heading(level), spacingProse)
		if above != ordinary || below != ordinary {
			t.Errorf("level %d spaced %d/%d px above/below with no heading space set; want the plain %d px block gap on both sides", level, above, below, ordinary)
		}
	}
}

// TestTheEmbeddedColumnSpacesHeadingsLikeTheScrollingList: the two entry
// points are one rhythm. A document laid out as a natural-height column — a
// chat message, a card — must space its headings exactly as the scrolling
// list does, or the same note reads differently depending on where it is put.
func TestTheEmbeddedColumnSpacesHeadingsLikeTheScrollingList(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	src := heading(1) + "\n\n" + spacingProse + "\n\n" + heading(2) + "\n\n" + heading(3) + "\n\n" + spacingProse + "\n"

	// The list closes the document with the last block's trailing space; the
	// column stops at its content. Everything between the blocks is shared.
	trailing := gapBetween(t, shaper, style, spacingProse, spacingProse)
	if got, want := listHeight(t, shaper, style, src), columnHeight(t, shaper, style, src)+trailing; got != want {
		t.Errorf("the same document lays out %d px tall in the list and %d px in a column plus its %d px trailing gap; the two paths space headings differently", got, want-trailing, trailing)
	}
}

// The reading rhythm the renderer is set to, as the reader sees it: blank-run
// heights in pixels at a 16 px body, read off the reference reading surface
// the same way this test reads them off ours. The reference's own runs vary
// with the words — 49 to 52 above a level-2 heading, 22 to 25 below one — so
// these are the middles of what it measured, and rhythmTolerance is the swing
// a run takes from the glyphs that happen to face it across the gap.
const (
	referenceBlockRun     = 37
	referenceAboveHeading = 50
	referenceBelowHeading = 23
	rhythmTolerance       = 4
)

// rhythmProse walks the swing on purpose: blocks whose facing lines carry
// descenders, ascenders, both and neither, and a heading of each kind — one
// that drops below the baseline and one that does not. A rhythm measured on
// one flattering pair of lines is a rhythm measured once.
const rhythmProse = "The measurement was taken from a rendered document.\n\n" +
	"Every paragraph ends on a different set of glyphs.\n\n" +
	"Some lines drop below the baseline; jumping typography.\n\n" +
	"Others do not, so the run above them measures wider.\n\n" +
	"CAPITALS AND NUMERALS 1234 END A LINE FLAT.\n\n" +
	"quiet lowercase prose, no ascenders over an x-height run\n\n" +
	"## The section it announces\n\n" +
	"The paragraph a section heading opens with.\n\n" +
	"Another ordinary paragraph closing the section.\n\n" +
	"## Jumping typography glyphs\n\n" +
	"quiet lowercase prose after the second heading\n"

// blankRuns returns the height of every run of rows carrying no ink, in
// order, ignoring the runs that open and close the image. A row counts as ink
// when any pixel in it differs from the background by more than a small
// luminance threshold, which is how the reference captures were scanned:
// low enough to catch a thin stroke, high enough that the tint behind a code
// block does not read as a wall of ink.
func blankRuns(img *image.RGBA) []int {
	b := img.Bounds()
	lum := func(x, y int) float64 {
		c := img.RGBAAt(x, y)
		return 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
	}
	bg := lum(b.Min.X, b.Min.Y)
	blank := make([]bool, b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		empty := true
		for x := b.Min.X; x < b.Max.X && empty; x++ {
			if d := lum(x, y) - bg; d > 24 || d < -24 {
				empty = false
			}
		}
		blank[y-b.Min.Y] = empty
	}
	var out []int
	for y := 0; y < len(blank); {
		if !blank[y] {
			y++
			continue
		}
		s := y
		for y < len(blank) && blank[y] {
			y++
		}
		if s > 0 && y < len(blank) {
			out = append(out, y-s)
		}
	}
	return out
}

// TestTheRenderedRhythmMatchesTheReference is the measurement the rhythm was
// set from, run forwards: render a document, scan its blank runs, and hold
// the three transitions a reader notices — between two ordinary blocks, above
// a heading, below one — against what the same three measure on the reference
// reading surface at the same body size.
//
// The blocks of rhythmProse are one-line paragraphs, so every interior blank
// run is a transition between two blocks and the runs come out in document
// order: five ordinary gaps, then above and below the first heading, then one
// more ordinary gap, then above and below the second.
func TestTheRenderedRhythmMatchesTheReference(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultDark, tokens.DefaultTypography)
	d := markdown.NewDocument(markdown.Parse([]byte(rhythmProse)))
	img := golden.Capture(t, image.Pt(560, 900), func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, tokens.DefaultDark.Background,
			clip.Rect{Max: gtx.Constraints.Max}.Op())
		return d.LayoutColumn(gtx, shaper, style)
	})
	runs := blankRuns(img)
	if len(runs) != 10 {
		t.Fatalf("scanned %d blank runs, want 10 (one per block transition): %v; the probe or the scan has drifted", len(runs), runs)
	}
	ordinary := []int{runs[0], runs[1], runs[2], runs[3], runs[4], runs[7]}
	above := []int{runs[5], runs[8]}
	below := []int{runs[6], runs[9]}

	// Every run of a kind is the same authored space seen through different
	// glyphs, so the average is that space with the swing taken out of it and
	// the individual runs are what the swing does to it.
	mean := func(runs []int) int {
		sum := 0
		for _, r := range runs {
			sum += r
		}
		return sum / len(runs)
	}
	if m := mean(ordinary); m < referenceBlockRun-2 || m > referenceBlockRun+2 {
		t.Errorf("ordinary blocks show the reader %d px of blank on average (%v), want %d ± 2 — the reference's openness between blocks", m, ordinary, referenceBlockRun)
	}
	for _, r := range above {
		if r < referenceAboveHeading-rhythmTolerance || r > referenceAboveHeading+rhythmTolerance {
			t.Errorf("a level-2 heading opened %d px of blank above it (%v), want %d ± %d", r, above, referenceAboveHeading, rhythmTolerance)
		}
	}
	for _, r := range below {
		if r < referenceBelowHeading-rhythmTolerance || r > referenceBelowHeading+rhythmTolerance {
			t.Errorf("a level-2 heading left %d px of blank below it (%v), want %d ± %d", r, below, referenceBelowHeading, rhythmTolerance)
		}
	}
	// The proportions the reference holds, on the pixels rather than on the
	// style: a heading shows about twice as much blank above it as below, and
	// half again what an ordinary pair of blocks shows. On the averages, not on
	// each run — a heading with no descender leaves the blank below it several
	// pixels wider while the blank above it is unchanged, which moves the ratio
	// on that one heading without moving the rhythm.
	if a, b := mean(above), mean(below); a < 2*b-2 {
		t.Errorf("headings show %d px above and %d px below on average (%v, %v); the space above must read as about twice the space below", a, b, above, below)
	}
	if a, o := mean(above), mean(ordinary); a < o*5/4 {
		t.Errorf("headings show %d px above on average against %d px between ordinary blocks (%v, %v); a heading must open distinctly more than a block gap", a, o, above, ordinary)
	}
}
