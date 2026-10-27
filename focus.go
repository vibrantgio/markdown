package markdown

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// focusTag is a non-zero-size type so its address is a unique event tag. A
// zero-size field could share an address with a neighbouring field.
type focusTag struct{ _ byte }

// Focus returns the document's keyboard focus tag. A caller gives the
// keyboard to the document with key.FocusCmd{Tag: d.Focus()}. The tag is
// stable for the lifetime of the Document.
//
// [Document.Layout] and [Document.LayoutScrollbar] register the tag.
// [Document.LayoutColumn] does not: an embedded document is part of the
// embedder's scrolling context, and the embedder keeps the keyboard. The tag
// is the first Tab stop of the document, before its task boxes and links.
func (d *Document) Focus() event.Tag { return &d.focus }

// Focused reports whether the document has the keyboard.
func (d *Document) Focused(gtx layout.Context) bool { return gtx.Focused(&d.focus) }

// focusArea registers the focus tag over the whole viewport. It runs before
// the blocks lay out: the router takes Tab order from the order of the
// registrations, so the document comes before its task boxes and links. The
// area is pass-through, so the links and task boxes still get pointer events.
func (d *Document) focusArea(gtx layout.Context) {
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: &d.focus}); !ok {
			break
		}
	}
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &d.focus)
	area.Pop()
	pass.Pop()
}
