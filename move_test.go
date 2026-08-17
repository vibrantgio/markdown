package markdown_test

import (
	"image"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// reader is the fixture for the document-movement tests: a document laid out
// repeatedly into a fixed viewport, with the scroll position readable between
// frames. The tests here are transitions — where the viewport was, what it was
// asked to do, where it ended up — not pictures.
type reader struct {
	doc    *markdown.Document
	blocks []markdown.Block
	shaper *text.Shaper
	style  markdown.Style
	size   image.Point
	ops    op.Ops
}

func newReader(t *testing.T, src string, size image.Point) *reader {
	t.Helper()
	blocks := markdown.Parse([]byte(src))
	return &reader{
		doc:    markdown.NewDocument(blocks),
		blocks: blocks,
		shaper: defaultShaper(t),
		style:  markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography),
		size:   size,
	}
}

// seatAt re-seats the document at a block index, the way a followed link's
// anchor landing does, so a test can start part way down.
func (r *reader) seatAt(first int) {
	r.doc = markdown.NewDocumentAt(r.blocks, first)
}

func (r *reader) frame() {
	r.ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(r.size),
		Ops:         &r.ops,
	}
	r.doc.Layout(gtx, r.shaper, r.style)
}

func (r *reader) pos() layout.Position { return r.doc.Position() }

// atEnd reports whether the viewport's trailing edge rests on the document's.
func (r *reader) atEnd() bool {
	p := r.pos()
	return p.First+p.Count == len(r.blocks) && p.OffsetLast >= 0
}

func (r *reader) atStart() bool {
	p := r.pos()
	return p.First == 0 && p.Offset == 0
}

// tallDoc is a document whose first block alone is many viewports tall: a
// fenced block of n lines. While the viewport stays inside that block,
// Position.Offset is the absolute scroll in pixels, which is what lets the
// page-distance test below be exact.
func tallDoc(n int) string {
	var b strings.Builder
	b.WriteString("```\n")
	for i := 0; i < n; i++ {
		b.WriteString("a line of code that goes on for a while\n")
	}
	b.WriteString("```\n\nA closing paragraph.\n")
	return b.String()
}

// longDoc is many ordinary blocks — headings, paragraphs, a list — so paging
// crosses block boundaries rather than staying inside one.
func longDoc(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("## Section\n\nSome prose in a paragraph that wraps at the\nreading measure and then keeps going for another line or two.\n\n- one\n- two\n- three\n\n")
	}
	return b.String()
}

// TestPageIsAViewportLessAnOverlap pins the distance one page move covers.
// The document's first block is taller than every page taken here, so the
// leading block never changes and Position.Offset is the absolute scroll.
func TestPageIsAViewportLessAnOverlap(t *testing.T) {
	size := image.Pt(480, 400)
	r := newReader(t, tallDoc(400), size)
	r.frame()
	if !r.atStart() {
		t.Fatalf("a fresh document is at %+v, want the top", r.pos())
	}

	r.doc.PageDown()
	r.frame()
	p := r.pos()
	if p.First != 0 {
		t.Fatalf("the first page down left block %d leading; the fixture's first block should be taller than that", p.First)
	}
	page := p.Offset
	if page >= size.Y {
		t.Fatalf("a page moved %d px in a %d px viewport: there is no overlap, so the reader loses the line they were on", page, size.Y)
	}
	if page < size.Y*3/4 {
		t.Fatalf("a page moved only %d px in a %d px viewport: the overlap is not small", page, size.Y)
	}

	// Every subsequent page covers the same distance.
	for i := 2; i <= 4; i++ {
		r.doc.PageDown()
		r.frame()
		if got, want := r.pos().Offset, page*i; got != want {
			t.Fatalf("after %d pages down: offset %d, want %d", i, got, want)
		}
	}
	// And up retraces it exactly.
	for i := 3; i >= 0; i-- {
		r.doc.PageUp()
		r.frame()
		if got, want := r.pos().Offset, page*i; got != want {
			t.Fatalf("paging back up to page %d: offset %d, want %d", i, got, want)
		}
	}
}

// TestPagingIsBoundedAtBothEnds is what a reader holding a page key down does.
// Paging past the end must land on the end and stay there, and the same going
// back up — never past, never oscillating.
func TestPagingIsBoundedAtBothEnds(t *testing.T) {
	r := newReader(t, longDoc(30), image.Pt(480, 400))
	r.frame()

	for i := 0; i < 200 && !r.atEnd(); i++ {
		r.doc.PageDown()
		r.frame()
	}
	if !r.atEnd() {
		t.Fatalf("paging down 200 times never reached the end: %+v", r.pos())
	}
	end := r.pos()
	for i := 0; i < 3; i++ {
		r.doc.PageDown()
		r.frame()
		if got := r.pos(); got.First != end.First || got.Offset != end.Offset {
			t.Fatalf("paging down past the end moved the document from %+v to %+v", end, got)
		}
	}

	for i := 0; i < 200 && !r.atStart(); i++ {
		r.doc.PageUp()
		r.frame()
	}
	if !r.atStart() {
		t.Fatalf("paging up 200 times never reached the start: %+v", r.pos())
	}
	for i := 0; i < 3; i++ {
		r.doc.PageUp()
		r.frame()
		if !r.atStart() {
			t.Fatalf("paging up past the start moved the document to %+v", r.pos())
		}
	}
}

// TestTheEndsAreTheSamePlacePagingReaches: the ends the keys jump to must be
// the ends paging arrives at, or the two ways of crossing a note disagree
// about where it stops.
func TestTheEndsAreTheSamePlacePagingReaches(t *testing.T) {
	size := image.Pt(480, 400)
	paged := newReader(t, longDoc(30), size)
	paged.frame()
	for i := 0; i < 200 && !paged.atEnd(); i++ {
		paged.doc.PageDown()
		paged.frame()
	}

	jumped := newReader(t, longDoc(30), size)
	jumped.frame()
	jumped.doc.ScrollToEnd()
	jumped.frame()
	if !jumped.atEnd() {
		t.Fatalf("ScrollToEnd left the document at %+v", jumped.pos())
	}
	if a, b := paged.pos(), jumped.pos(); a.First != b.First || a.Offset != b.Offset {
		t.Fatalf("ScrollToEnd landed at %+v, paging to the end at %+v", b, a)
	}

	jumped.doc.ScrollToStart()
	jumped.frame()
	if !jumped.atStart() {
		t.Fatalf("ScrollToStart left the document at %+v", jumped.pos())
	}
}

// TestTheEndsReachFromAnAnchorLanding starts the document seated part way down,
// as a followed link's anchor leaves it, and checks both ends are still
// reachable from there — the seating is a scroll position like any other, not
// a mode the keys have to know about.
func TestTheEndsReachFromAnAnchorLanding(t *testing.T) {
	r := newReader(t, longDoc(30), image.Pt(480, 400))
	r.seatAt(20)
	r.frame()
	if r.atStart() || r.atEnd() {
		t.Fatalf("the fixture seated at block 20 is already at an end: %+v", r.pos())
	}

	r.doc.ScrollToStart()
	r.frame()
	if !r.atStart() {
		t.Fatalf("ScrollToStart from an anchor landing left %+v", r.pos())
	}

	r.seatAt(20)
	r.frame()
	r.doc.ScrollToEnd()
	r.frame()
	if !r.atEnd() {
		t.Fatalf("ScrollToEnd from an anchor landing left %+v", r.pos())
	}
}

// TestADocumentThatFitsDoesNotMove: with the whole note on screen there is
// nowhere to go, and every move must be a no-op — a page that scrolled the
// last line off a document that fits would be the worst kind of motion.
func TestADocumentThatFitsDoesNotMove(t *testing.T) {
	src := "# Short\n\nTwo lines, and no more than that.\n"
	for _, tc := range []struct {
		name string
		move func(d *markdown.Document)
	}{
		{"page down", (*markdown.Document).PageDown},
		{"page up", (*markdown.Document).PageUp},
		{"to the end", (*markdown.Document).ScrollToEnd},
		{"to the start", (*markdown.Document).ScrollToStart},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newReader(t, src, image.Pt(480, 400))
			r.frame()
			before := r.pos()
			tc.move(r.doc)
			r.frame()
			if got := r.pos(); got.First != before.First || got.Offset != before.Offset {
				t.Fatalf("a document that fits moved from %+v to %+v", before, got)
			}
		})
	}
}

// TestMovingBeforeTheFirstLayoutIsHarmless: a key can arrive on the frame
// before the document has ever laid out, when there is no viewport to measure
// a page against. Nothing may move, and nothing may panic.
func TestMovingBeforeTheFirstLayoutIsHarmless(t *testing.T) {
	r := newReader(t, longDoc(10), image.Pt(480, 400))
	r.doc.PageDown()
	r.doc.PageUp()
	r.frame()
	if !r.atStart() {
		t.Fatalf("paging before the first layout moved the document to %+v", r.pos())
	}
}

// TestAKeyboardMoveChangesTheScrollFractions is what wakes the scroll
// indicator: the bar reads the viewport's fractions off the same position, so
// a page move must change them exactly as a wheel scroll does.
func TestAKeyboardMoveChangesTheScrollFractions(t *testing.T) {
	r := newReader(t, longDoc(30), image.Pt(480, 400))
	r.frame()
	before := r.pos()
	r.doc.PageDown()
	r.frame()
	after := r.pos()
	switch {
	case after.Length <= 0:
		t.Fatalf("Length = %d; no proportion can be derived from it", after.Length)
	case before.First == after.First && before.Offset == after.Offset:
		t.Fatalf("a page down left the position at %+v", after)
	}
}

// TestScrollToBlockSeatsTheNamedBlock is the move an outline entry makes: the
// named block leads the viewport afterwards, whether it was below the fold or
// above it, and the document is the same one throughout — nothing here builds a
// second document at the target.
func TestScrollToBlockSeatsTheNamedBlock(t *testing.T) {
	r := newReader(t, longDoc(30), image.Pt(480, 400))
	r.frame()
	before := r.doc

	// Down the document and back up it, past the fold in both directions.
	for _, i := range []int{18, 3, 40, 9} {
		r.doc.ScrollToBlock(i)
		r.frame()
		if p := r.pos(); p.First != i || p.Offset != 0 {
			t.Fatalf("ScrollToBlock(%d): position = %+v, want First %d Offset 0", i, p, i)
		}
	}
	if r.doc != before {
		t.Error("the document was rebuilt; scrolling to a block must move the reader, not reload the note")
	}
}

// TestScrollToBlockOutOfRange covers the two indices a caller may hand over
// without checking: past the last block lands on the document's end, and a
// negative one on its start.
func TestScrollToBlockOutOfRange(t *testing.T) {
	r := newReader(t, longDoc(20), image.Pt(480, 400))
	r.frame()

	r.doc.ScrollToBlock(len(r.blocks) + 25)
	r.frame()
	if !r.atEnd() {
		t.Fatalf("ScrollToBlock past the last block left %+v, want the document's end", r.pos())
	}

	r.doc.ScrollToBlock(-3)
	r.frame()
	if !r.atStart() {
		t.Fatalf("ScrollToBlock(-3) left %+v, want the document's start", r.pos())
	}
}

// TestScrollToBlockOnADocumentThatFits is the short-note case: every block is
// already on screen, so naming one moves nothing.
func TestScrollToBlockOnADocumentThatFits(t *testing.T) {
	r := newReader(t, "# Title\n\nOne short paragraph.\n", image.Pt(480, 400))
	r.frame()
	r.doc.ScrollToBlock(len(r.blocks) - 1)
	r.frame()
	if !r.atStart() {
		t.Fatalf("position = %+v; a document shorter than the viewport must not move", r.pos())
	}
}
