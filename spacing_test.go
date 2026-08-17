package markdown_test

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// The spacing tests are transitions, not pictures: each measures what the
// layout puts between two adjacent blocks by differencing heights — the
// column holding both, less what each block takes on its own — so the numbers
// they compare are the inserted space itself, free of the shaped text's own
// height.

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

// TestStackedHeadingsCloseUp: a heading directly under another heading is the
// second half of one announcement, not the start of a second section, so it
// takes less space above it than the same heading following prose — while
// still standing apart from the heading above it.
func TestStackedHeadingsCloseUp(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)

	// A gap is what the block above closes with plus what the block below
	// opens with, so differencing against a paragraph in the lower position
	// leaves the heading's own space above and nothing else.
	stacked := gapBetween(t, shaper, style, heading(2), heading(3)) -
		gapBetween(t, shaper, style, heading(2), spacingProse)
	opened := gapBetween(t, shaper, style, spacingProse, heading(3)) -
		gapBetween(t, shaper, style, spacingProse, spacingProse)
	switch {
	case stacked >= opened:
		t.Errorf("a heading under a heading opened %d px above it, one under a paragraph %d px; the stacked pair must close up", stacked, opened)
	case stacked <= 0:
		t.Errorf("a heading under a heading opened %d px above it; the pair is still two blocks", stacked)
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
