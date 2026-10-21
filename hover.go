// hover.go reports the address under the pointer to the document's caller:
// one address for the whole document, told no more than once a frame.
//
// A document is many paragraphs, and each of them reports the link under the
// pointer for itself. Crossing from a link in one block straight onto a link
// in another is therefore two reports on one frame — a departure and an
// arrival — in whichever order those blocks happen to lay out. So the
// document collects the frame's reports and tells its caller once, after
// every block has laid out, an address reported anywhere in the document
// winning over a departure reported elsewhere in it.

package markdown

import (
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
}

// hoverFrame opens one frame's collection. Every layout path calls it before
// it draws.
func (d *Document) hoverFrame() {
	d.hover.frame, d.hover.seen = "", false
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
