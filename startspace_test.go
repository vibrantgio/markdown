package markdown_test

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// The space a scrolling document keeps above its first block.
//
// These are endspace_test.go's questions at the other end of the viewport, and
// they are asked the same way: in pixels, because what is under test is where
// the ink starts relative to the viewport's leading edge, and no position
// value answers that.

// startSpaceUnderTest is the inset the tests here ask for: a round number well
// clear of anything a block opens with by itself, so a gap that came from the
// document's own spacing cannot pass for the inset.
const startSpaceUnderTest = unit.Dp(40)

// startShot lays a document out at the size given, moved to wherever move puts
// it, and returns the number of pixel rows between the top edge and the first
// ink — the blank the reader sees over the document's first line.
func startShot(t *testing.T, src string, size image.Point, start unit.Dp, move func(*markdown.Document)) int {
	t.Helper()
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	style.StartSpace = start
	doc := markdown.NewDocument(markdown.Parse([]byte(src)))

	var ops op.Ops
	frame := func() {
		ops.Reset()
		doc.Layout(layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Ops:         &ops,
		}, shaper, style)
	}
	frame()
	if move != nil {
		move(doc)
	}
	frame()

	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, tokens.DefaultLight.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return doc.Layout(gtx, shaper, style)
	})
	return blankAbove(img)
}

// blankAbove returns the number of rows at the head of img carrying no ink.
func blankAbove(img *image.RGBA) int {
	ground := tokens.DefaultLight.Background
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R != ground.R || c.G != ground.G || c.B != ground.B {
				return y - b.Min.Y
			}
		}
	}
	return b.Dy()
}

// TestADocumentRestsClearOfTheViewportsStart is the point of the inset: at the
// top of the document, the first line stands off the leading edge rather than
// sitting on it. Without the inset it sits on it, which is the state this
// compares against — one number apart, and the number is the one asked for.
func TestADocumentRestsClearOfTheViewportsStart(t *testing.T) {
	size := image.Pt(480, 400)

	flush := startShot(t, longDoc(30), size, 0, nil)
	rested := startShot(t, longDoc(30), size, startSpaceUnderTest, nil)

	if got, want := rested-flush, int(startSpaceUnderTest); got != want {
		t.Errorf("the document rested %d px further from the leading edge than a flush one, want %d", got, want)
	}
	if rested < int(startSpaceUnderTest) {
		t.Errorf("the first line rests %d px below the leading edge, less than the %d px asked for", rested, int(startSpaceUnderTest))
	}
}

// TestOnlyTheStartPaysForTheSpace: the inset is a resting position, not a
// margin. Part way down a document the viewport's first row carries whatever
// the scroll left there, and a line half off the leading edge is the viewport
// cutting it. A document that reserved the space on every frame would leave a
// strip of empty ground over that half-cut line, which reads as a clipping
// fault rather than as scrolling — and it would put that strip under whatever
// chrome the viewport begins against.
func TestOnlyTheStartPaysForTheSpace(t *testing.T) {
	size := image.Pt(480, 400)
	page := func(d *markdown.Document) { d.PageDown() }

	flush := startShot(t, longDoc(30), size, 0, page)
	inset := startShot(t, longDoc(30), size, startSpaceUnderTest, page)

	if flush != inset {
		t.Errorf("part way down, the ink starts %d px below the leading edge with the inset and %d px without it", inset, flush)
	}
}

// TestTheStartAgreesWithTheRestingPosition: the resting position the inset
// creates is the one every way of reaching the start lands on, and it is a
// place the document stays and can leave again.
func TestTheStartAgreesWithTheRestingPosition(t *testing.T) {
	size := image.Pt(480, 400)

	paged := newReader(t, longDoc(30), size)
	paged.style.StartSpace = startSpaceUnderTest
	paged.seatAt(20)
	paged.frame()
	for i := 0; i < 200 && !paged.atStart(); i++ {
		paged.doc.PageUp()
		paged.frame()
	}
	if !paged.atStart() {
		t.Fatalf("paging up 200 times never reached the start: %+v", paged.pos())
	}

	jumped := newReader(t, longDoc(30), size)
	jumped.style.StartSpace = startSpaceUnderTest
	jumped.seatAt(20)
	jumped.frame()
	jumped.doc.ScrollToStart()
	jumped.frame()
	if !jumped.atStart() {
		t.Fatalf("ScrollToStart left the document at %+v", jumped.pos())
	}
	if a, b := paged.pos(), jumped.pos(); a.First != b.First || a.Offset != b.Offset {
		t.Fatalf("ScrollToStart landed at %+v, paging to the start at %+v", b, a)
	}

	before := jumped.pos()
	jumped.frame()
	if got := jumped.pos(); got.First != before.First || got.Offset != before.Offset {
		t.Fatalf("the resting position drifted from %+v to %+v on the next frame", before, got)
	}

	// And the reader can leave it again: a start the document could not be
	// paged out of would be a trap at the head of every long note.
	for i := 0; i < 200 && !jumped.atEnd(); i++ {
		jumped.doc.PageDown()
		jumped.frame()
	}
	if !jumped.atEnd() {
		t.Fatalf("paging down from the resting position never reached the end: %+v", jumped.pos())
	}
}

// TestASeatedDocumentStartsFlush: the space belongs to the document's first
// block, not to whichever block the viewport happens to lead with. A landing
// that seats a heading at the top — a followed link's anchor, an outline
// entry — puts that heading against the leading edge, exactly as it did before
// the inset existed.
func TestASeatedDocumentStartsFlush(t *testing.T) {
	size := image.Pt(480, 400)
	seat := func(d *markdown.Document) { d.ScrollToBlock(9) }

	flush := startShot(t, longDoc(30), size, 0, seat)
	inset := startShot(t, longDoc(30), size, startSpaceUnderTest, seat)

	if flush != inset {
		t.Errorf("seated at a block, the ink starts %d px below the leading edge with the inset and %d px without it", inset, flush)
	}
}

// TestAnEmbeddedColumnKeepsItsHeightAboveToo: [Document.LayoutColumn] takes
// exactly the height its blocks need, at both ends. The space above an
// embedded document's first block is the embedder's to spend, so the inset
// must not appear in the column's height.
func TestAnEmbeddedColumnKeepsItsHeightAboveToo(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte(longDoc(4)))
	measure := func(start unit.Dp) layout.Dimensions {
		style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
		style.StartSpace = start
		var ops op.Ops
		return markdown.NewDocument(blocks).LayoutColumn(layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(480, 1<<20)},
			Ops:         &ops,
		}, shaper, style)
	}
	if a, b := measure(0), measure(startSpaceUnderTest); a.Size != b.Size {
		t.Errorf("an embedded column measured %v with the inset and %v without it", b.Size, a.Size)
	}
}
