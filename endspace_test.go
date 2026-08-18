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

// The space a scrolling document keeps below its last block.
//
// The measurements below are pixels, because the question is where the ink
// stops relative to the viewport's trailing edge and no position value
// answers that. Every shot fills the ground with the token background first,
// so "the last row that is not the ground" is the last row that carries ink.

// endSpaceUnderTest is the inset the tests here ask for. Any value clear of
// the ordinary block gap would do; this one is a round number well above it,
// so a gap that merely came from the last block's own closing space cannot
// pass for the inset.
const endSpaceUnderTest = unit.Dp(40)

// endShot lays a document out at the size given, moved to wherever move puts
// it, and returns the number of pixel rows between the last ink and the
// bottom edge — the blank the reader sees under the document's last line.
//
// The document lays out three times: once to give the move a viewport to
// measure against, once to carry the move out, and once inside the capture.
// That is what the application does too, a key arriving on the frame before
// the one that answers it.
func endShot(t *testing.T, src string, size image.Point, end unit.Dp, move func(*markdown.Document)) int {
	t.Helper()
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	style.EndSpace = end
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
	return blankBelow(img)
}

// blankBelow returns the number of rows at the foot of img carrying no ink.
func blankBelow(img *image.RGBA) int {
	ground := tokens.DefaultLight.Background
	b := img.Bounds()
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R != ground.R || c.G != ground.G || c.B != ground.B {
				return b.Max.Y - 1 - y
			}
		}
	}
	return b.Dy()
}

// TestADocumentRestsClearOfTheViewportsEnd is the whole point of the inset:
// scrolled as far as it goes, the document's last line stands off the
// trailing edge rather than sitting on it. Without the inset it sits on it,
// which is the state this compares against — one number apart, and the number
// is the one asked for.
func TestADocumentRestsClearOfTheViewportsEnd(t *testing.T) {
	size := image.Pt(480, 400)
	toEnd := (*markdown.Document).ScrollToEnd

	flush := endShot(t, longDoc(30), size, 0, toEnd)
	rested := endShot(t, longDoc(30), size, endSpaceUnderTest, toEnd)

	if got, want := rested-flush, int(endSpaceUnderTest); got != want {
		t.Errorf("the document rested %d px further from the edge than a flush one, want %d", got, want)
	}
	if rested < int(endSpaceUnderTest) {
		t.Errorf("the last line rests %d px above the trailing edge, less than the %d px asked for", rested, int(endSpaceUnderTest))
	}
}

// TestOnlyTheEndPaysForTheSpace: the inset is a resting position, not a
// margin. Part way down a long document every row of the viewport may carry
// text, and a line half off the trailing edge is the viewport cutting it —
// exactly as it is without the inset. A document that reserved the space on
// every frame would leave a strip of empty ground under a half-cut line,
// which reads as a clipping fault rather than as scrolling.
func TestOnlyTheEndPaysForTheSpace(t *testing.T) {
	size := image.Pt(480, 400)
	page := func(d *markdown.Document) { d.PageDown() }

	flush := endShot(t, longDoc(30), size, 0, page)
	inset := endShot(t, longDoc(30), size, endSpaceUnderTest, page)

	if flush != inset {
		t.Errorf("part way down, the ink stops %d px above the edge with the inset and %d px without it", inset, flush)
	}
}

// TestTheEndsAgreeWithTheRestingPosition: the resting position the inset
// creates is the one every way of reaching the end lands on. Paging down to
// the end and jumping to it must agree, or a key and a held page key stop the
// reader in two different places.
func TestTheEndsAgreeWithTheRestingPosition(t *testing.T) {
	size := image.Pt(480, 400)

	paged := newReader(t, longDoc(30), size)
	paged.style.EndSpace = endSpaceUnderTest
	paged.frame()
	for i := 0; i < 200 && !paged.atEnd(); i++ {
		paged.doc.PageDown()
		paged.frame()
	}
	if !paged.atEnd() {
		t.Fatalf("paging down 200 times never reached the end: %+v", paged.pos())
	}

	jumped := newReader(t, longDoc(30), size)
	jumped.style.EndSpace = endSpaceUnderTest
	jumped.frame()
	jumped.doc.ScrollToEnd()
	jumped.frame()
	if !jumped.atEnd() {
		t.Fatalf("ScrollToEnd left the document at %+v", jumped.pos())
	}
	if a, b := paged.pos(), jumped.pos(); a.First != b.First || a.Offset != b.Offset {
		t.Fatalf("ScrollToEnd landed at %+v, paging to the end at %+v", b, a)
	}

	// And the end is a place the document stays: a second frame must not
	// drift, or the indicator would creep after the reader stopped.
	before := jumped.pos()
	jumped.frame()
	if got := jumped.pos(); got.First != before.First || got.Offset != before.Offset {
		t.Fatalf("the resting position drifted from %+v to %+v on the next frame", before, got)
	}

	// And the reader can leave it again. A resting position that could not be
	// paged back out of would be a trap at the foot of every long document.
	for i := 0; i < 200 && !jumped.atStart(); i++ {
		jumped.doc.PageUp()
		jumped.frame()
	}
	if !jumped.atStart() {
		t.Fatalf("paging back up from the resting position never reached the start: %+v", jumped.pos())
	}
}

// TestADocumentThatFitsRestsWhereItAlwaysDid: asking for the inset must not
// start a document scrolling that has nowhere to scroll. The space is at the
// end of a document the reader can reach the end of; a note shorter than the
// viewport is already showing its end.
func TestADocumentThatFitsRestsWhereItAlwaysDid(t *testing.T) {
	r := newReader(t, "# Short\n\nTwo lines, and no more than that.\n", image.Pt(480, 400))
	r.style.EndSpace = endSpaceUnderTest
	r.frame()
	before := r.pos()
	r.doc.ScrollToEnd()
	r.frame()
	if got := r.pos(); got.First != before.First || got.Offset != before.Offset {
		t.Fatalf("a document that fits moved from %+v to %+v", before, got)
	}
}

// TestAnEmbeddedColumnKeepsItsContentHeight: [Document.LayoutColumn] is for a
// document inside somebody else's scrolling viewport, and it takes exactly the
// height its blocks need. The space below such a document's end is the
// embedder's to spend, so the inset must not appear in the column's height.
func TestAnEmbeddedColumnKeepsItsContentHeight(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte(longDoc(4)))
	measure := func(end unit.Dp) layout.Dimensions {
		style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
		style.EndSpace = end
		var ops op.Ops
		return markdown.NewDocument(blocks).LayoutColumn(layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(480, 1<<20)},
			Ops:         &ops,
		}, shaper, style)
	}
	if a, b := measure(0), measure(endSpaceUnderTest); a.Size != b.Size {
		t.Errorf("an embedded column measured %v with the inset and %v without it", b.Size, a.Size)
	}
}
