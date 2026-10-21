// hover.go reports the address the reader is on to the document's caller,
// and where that address is: one address and one box for the whole document,
// told no more than once a frame.
//
// The reader is on a link by pointing at it or by giving it the keyboard,
// and a paragraph says so either way; the pointer wins while both stand. A
// document is many paragraphs, and each of them reports for itself. Crossing
// from a link in one block straight onto a link in another is therefore two
// reports on one frame — a departure and an arrival — in whichever order
// those blocks happen to lay out. So the document collects the frame's
// reports and tells its caller once, after every block has laid out, an
// address reported anywhere in the document winning over a departure
// reported elsewhere in it.
//
// The box is carried rather than reported: [paragraph.Style] is the
// components contract and its hover callback says the address alone, so a
// caller reads the rectangle back off the document with [Document.LinkBox]
// instead of being handed it by a second callback this package cannot add to
// that style. A paragraph reports the box in its own coordinates; carrying
// it out to the plane the document was laid out into is
// [Document.hoverInto]'s job, called by every layout path that offsets what
// it has drawn.

package markdown

import (
	"image"

	"gioui.org/layout"

	"github.com/vibrantgio/components/paragraph"
)

// hoverState is the document's link-address state.
type hoverState struct {
	// url is the address the caller was last told.
	url string
	// frame is the address this frame's paragraphs reported, and seen
	// whether any of them reported at all: a frame on which no paragraph
	// changed says nothing, which is not the same as one reporting no
	// address.
	frame string
	seen  bool

	// at is the offset from the plane the current top-level block is being
	// laid out into to whatever is being drawn now. Every path that offsets
	// what it has drawn adds its offset to it for as long as its subtree is
	// laying out; see [Document.hoverInto].
	at image.Point
	// box is where this frame's layout drew the link the reader is on and
	// boxed whether it drew one at all. The box is in the coordinates of the
	// row or column the block was laid out into, which row then carries the
	// rest of the way. pointed says a pointer is what put it there, which is
	// what keeps a focused link in one block from taking the box off the
	// link the pointer is on in another, and seq counts the writes so a
	// layout path can tell whether the block it just laid out is the one
	// that answered.
	box     image.Rectangle
	boxed   bool
	pointed bool
	seq     int
	// row is the top-level block whose row holds the box, on the paths that
	// lay the blocks out as list rows; nil on the paths that stack them
	// themselves, where the box needs no carrying. The list decides where a
	// row sits, so [Document.rowTops] is what places it.
	row Block
}

// hoverFrame opens one frame's collection. Every layout path calls it before
// it draws.
func (d *Document) hoverFrame() {
	d.hover.frame, d.hover.seen = "", false
	d.hover.box, d.hover.boxed, d.hover.row = image.Rectangle{}, false, nil
	d.hover.pointed = false
	d.hover.at = image.Point{}
}

// hoverInto enters a subtree drawn at off and returns the function that
// leaves it again. It is [Document.shift]'s counterpart for the one box the
// hover has: every layout path that offsets what it has drawn calls it, so a
// box a paragraph reports in its own coordinates comes out in the plane the
// document laid its blocks into.
func (d *Document) hoverInto(off image.Point) func() {
	d.hover.at = d.hover.at.Add(off)
	return func() { d.hover.at = d.hover.at.Sub(off) }
}

// hoverBox records where the link the reader is on landed, when the
// paragraph just laid out is the one holding it. One pointer is on one link
// and one link holds the keyboard, so the last paragraph to answer is the
// only one that answers — and a paragraph the pointer is on answers over one
// that only holds the keyboard, whichever of them laid out first.
func (d *Document) hoverBox(gtx layout.Context, state *paragraph.State) {
	if r, ok := state.HoveredLinkBox(); ok {
		d.setBox(r, true)
		return
	}
	if d.hover.pointed {
		return
	}
	if r, ok := state.FocusedLinkBox(gtx); ok {
		d.setBox(r, false)
	}
}

// setBox carries one paragraph's box out to the plane the current block is
// being laid out into and counts the write.
func (d *Document) setBox(r image.Rectangle, pointed bool) {
	d.hover.box, d.hover.boxed, d.hover.pointed = r.Add(d.hover.at), true, pointed
	d.hover.seq++
}

// LinkBox returns where the last layout drew the link the reader is on, in
// the plane the document was laid out into — the viewport for
// [Document.Layout] and [Document.LayoutScrollbar], the whole column for
// [Document.LayoutColumn] — and false when the reader is on no link.
//
// It is the rectangle an annotation standing beside a link is placed
// against: [Style].Text.OnLinkHover says which address the reader is on and
// this says where that link is. A link the pointer is on reports the run
// under the pointer and a link holding the keyboard the block around all of
// its runs, as [paragraph.State.HoveredLinkBox] and
// [paragraph.State.FocusedLinkBox] do.
//
// Read it after the layout for this frame's geometry. It is empty before the
// first frame and for a link in a block the viewport does not reach, the way
// [Match].Rect is: the position is a result of wrapping and of where the
// reader is, and neither is known until the document lays out.
func (d *Document) LinkBox() (image.Rectangle, bool) {
	if !d.hover.boxed {
		return image.Rectangle{}, false
	}
	box := d.hover.box
	if d.hover.row != nil {
		top, ok := d.rowTops()[d.hover.row]
		if !ok {
			return image.Rectangle{}, false
		}
		box = box.Add(image.Pt(0, top))
	}
	return box, true
}

// hovered puts the document's collector on a paragraph style in place of the
// caller's own callback. Every path laying out live prose passes its style
// through it, so what a block reports reaches the document and not the
// caller.
func (d *Document) hovered(ps paragraph.Style) paragraph.Style {
	ps.OnLinkHover = func(_ layout.Context, url string) {
		d.hover.seen = true
		if url != "" || d.hover.frame == "" {
			d.hover.frame = url
		}
	}
	return ps
}

// hoverReport tells the caller the frame's address once every block has laid
// out. Every layout path calls it after it draws.
func (d *Document) hoverReport(gtx layout.Context, style Style) {
	if !d.hover.seen || d.hover.frame == d.hover.url {
		return
	}
	d.hover.url = d.hover.frame
	if style.Text.OnLinkHover != nil {
		style.Text.OnLinkHover(gtx, d.hover.url)
	}
}
