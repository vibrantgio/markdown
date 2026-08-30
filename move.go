package markdown

import (
	"gioui.org/layout"
)

// Moving a document from outside its pointer events: a page in either
// direction, and the two ends.
//
// The moves take no layout.Context because a key arrives on the frame that
// handles it, before the document lays out again: each records what it was
// asked for and the next layout carries it out, which is also what bounds it
// at the document's ends.

// pageOverlapDivisor caps the overlap a page move keeps at this fraction of the
// viewport. On an ordinary reading column a line of text is far below the cap;
// it exists so a viewport only a line or two tall still advances by most of
// itself rather than barely moving.
const pageOverlapDivisor = 3

// recordLine stores one line of body text in pixels, which is the overlap a
// page move keeps. The factor is the ordinary ratio of a line's height to its
// text size — the exact shaped height varies by face and by line, and a page's
// overlap does not need to be exact, only to be about a line.
func (d *Document) recordLine(gtx layout.Context, style Style) {
	d.line = gtx.Sp(style.Text.Size) * 3 / 2
}

// Position returns the scroll position the document's most recent layout
// resolved: which block leads the viewport, how far into it the leading edge
// sits, how many blocks are laid out, and the estimated total height.
func (d *Document) Position() layout.Position { return d.list.Position() }

// PageDown moves the viewport one page toward the end of the document, and
// PageUp one page back. A page is the viewport less a line of overlap, so the
// line the reader was on stays on screen and the eye has somewhere to land.
//
// Both are bounded by the document: the last page stops at the end rather than
// scrolling content off it, and a document shorter than the viewport does not
// move at all. Before the document's first layout there is no viewport to
// measure a page against and neither moves anything.
func (d *Document) PageDown() { d.list.ScrollPixels(d.page()) }

// PageUp moves the viewport one page toward the start of the document. See
// [Document.PageDown].
func (d *Document) PageUp() { d.list.ScrollPixels(-d.page()) }

// ScrollToStart puts the document's first block at the top of the viewport.
func (d *Document) ScrollToStart() { d.list.ScrollToStart() }

// ScrollToEnd puts the document's last line at the bottom of the viewport, so
// the viewport shows the end of the text and no empty space past it.
func (d *Document) ScrollToEnd() { d.list.ScrollToEnd(len(d.blocks)) }

// ScrollToBlock puts the top-level block at index i at the top of the viewport.
// It is how an outline entry, or anything else naming a place in the document,
// takes the reader there: the same document keeps reading, seated somewhere
// else, so what the reader had open, scrolled and interacted with survives the
// move.
//
// The index is the one [Document.Blocks] is indexed by, which is also what
// [NewDocumentAt] seats a fresh document at. Out of range is not an error: a
// negative index goes to the start, and one past the last block goes as far as
// the document does.
func (d *Document) ScrollToBlock(i int) {
	if n := len(d.blocks); i >= n {
		d.ScrollToEnd()
		return
	}
	d.list.ScrollTo(i)
}

// page is the pixel distance one page move covers: the viewport the document
// last laid out in, less a line of overlap.
func (d *Document) page() int {
	v := d.list.Viewport()
	if v <= 0 {
		return 0
	}
	overlap := d.line
	if most := v / pageOverlapDivisor; overlap > most {
		overlap = most
	}
	if overlap < 0 {
		overlap = 0
	}
	return v - overlap
}
