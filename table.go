// table.go keeps what a table block measured and drew, so a wide table is
// shaped once rather than three times a frame.
//
// Laying a table out reads every cell three times: once at the full content
// width for the columns' natural widths, once word by word for their
// min-content widths when the naturals overflow, and once more at the
// distributed widths to draw. Each of those shapes the cell's text and throws
// the shaping away, so a table of long cells costs its whole shaping on every
// frame it is in view — a fifteen-row table of two-thousand-character cells
// measured 218 ms a frame before this file existed. What the three passes
// produce, though, is a few integers per column and one recorded call per
// cell, and none of it changes while the width, the styling and the marking
// hold. So it is kept.

package markdown

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/paragraph"
)

// cellStyle is everything a table cell's shaping and colours are read from:
// the density and the shaper that size and shape it, the paragraph defaults
// it is set in, and the Style fields [cellSpans] takes. Nothing else reaches
// a cell — a cell's own width is the measurement being keyed, and a cell
// reads no other part of the frame — so two layouts agreeing on this value
// draw the same cell.
//
// It is derived field by field rather than taken as the Style itself because
// a Style carries callbacks, which no two values of it compare equal on.
type cellStyle struct {
	metric unit.Metric
	shaper *text.Shaper
	// The paragraph defaults, from Style.Text.
	color color.NRGBA
	link  color.NRGBA
	focus color.NRGBA
	hover color.NRGBA
	size  unit.Sp
	lineH unit.Sp
	// The inline-code treatment, from the Style's own fields.
	mono     font.Typeface
	codeSize unit.Sp
	chip     color.NRGBA
	chipEdge color.NRGBA
	// The fills a marked cell is drawn with.
	match        color.NRGBA
	currentMatch color.NRGBA
}

// cellStyleOf reads the styling a frame draws a table's cells in.
func cellStyleOf(gtx layout.Context, shaper *text.Shaper, style Style) cellStyle {
	return cellStyle{
		metric:       gtx.Metric,
		shaper:       shaper,
		color:        style.Text.Color,
		link:         style.Text.LinkColor,
		focus:        style.Text.FocusColor,
		hover:        style.Text.HoverColor,
		size:         style.Text.Size,
		lineH:        style.Text.LineHeight,
		mono:         style.Mono,
		codeSize:     style.CodeSize,
		chip:         style.CodeChip,
		chipEdge:     style.CodeChipBorder,
		match:        style.MatchFill,
		currentMatch: style.CurrentMatchFill,
	}
}

// tableMeasure is one table's measured columns.
//
// The minimum widths are kept apart from the other two because no available
// width changes them: a column's min-content width is the widest single word
// it holds, measured unconstrained, so a window being resized re-measures the
// natural widths and redistributes, and the per-word pass — the one that
// shapes every word in the table separately — is spent once for the styling.
type tableMeasure struct {
	// style is the styling all of it was measured in; a change of styling
	// discards the whole entry.
	style cellStyle
	// mins is each column's min-content width, unclamped.
	mins []int
	// avail is the content width naturals and widths were measured for, and
	// measured says they were measured at all.
	avail    int
	measured bool
	naturals []int
	widths   []int
}

// cellRecord is one table cell's drawn call, held in an op list of its own so
// it outlives the frame it was recorded on: the frame's own list is reset
// before the next one, which would leave the call pointing at nothing.
type cellRecord struct {
	ops  *op.Ops
	key  cellKey
	call op.CallOp
	size image.Point
	// marks is the marking the record was drawn with, and found the
	// rectangles the marks landed at, in the cell's own coordinates. A
	// replayed cell reports them again: they are collected by the layout
	// above, which cannot see inside a recorded call.
	marks []spanMark
	found []foundRect
	// drawn marks the record as holding a cell rather than being empty.
	drawn bool
}

// cellKey is what a recorded cell is kept under. current is the match drawn
// in the current match's own fill, and is held only for a cell that carries
// marks: for every other cell the find's position changes nothing.
type cellKey struct {
	style   cellStyle
	width   int
	header  bool
	current int
}

// tableMeasured returns the persistent column measurement for a table.
func (d *Document) tableMeasured(t *Table) *tableMeasure {
	m, ok := d.columns[t]
	if !ok {
		m = &tableMeasure{}
		d.columns[t] = m
	}
	return m
}

// cellRecorded returns the persistent drawn record for a table cell.
func (d *Document) cellRecorded(cell *TableCell) *cellRecord {
	r, ok := d.cells[cell]
	if !ok {
		r = &cellRecord{ops: new(op.Ops)}
		d.cells[cell] = r
	}
	return r
}

// holds reports whether the record was drawn for exactly this key and
// marking, and so may be replayed instead of laid out again.
func (r *cellRecord) holds(key cellKey, marks []spanMark) bool {
	if !r.drawn || r.key != key || len(r.marks) != len(marks) {
		return false
	}
	for i := range marks {
		if r.marks[i] != marks[i] {
			return false
		}
	}
	return true
}

// linked reports whether a cell's text carries a link.
func linked(cell *TableCell) bool {
	for _, sp := range cell.Spans {
		if sp.URL != "" {
			return true
		}
	}
	return false
}

// cell draws one table cell inside the width gtx allows, returning the call
// that places it and the size it took, and appending wherever a mark landed
// to the document's found rectangles in the cell's own coordinates — the
// contract the grid's own layout keeps for them.
//
// A cell whose text carries no link is recorded once per styling, width and
// marking and replayed from that record on every frame after. Shaping is the
// whole cost of a table of long cells, and nothing a cell without a link
// draws changes between frames: the width is the only part of the frame its
// wrapping reads, and the styling is the only other thing it is drawn from.
//
// A cell that does carry a link is laid out live on every frame instead. Its
// links drain that frame's events and draw the hover and focus treatments the
// pointer and the keyboard are asking for; a recorded call draws neither and
// drains nothing, and a link whose events are never drained is not a link.
func (d *Document) cell(gtx layout.Context, shaper *text.Shaper, style Style, skey cellStyle, cell *TableCell, header bool) (op.CallOp, image.Point) {
	live := func(gtx layout.Context) (op.CallOp, image.Point) {
		spans, fills := d.fills(style, cell, cellSpans(style, cell, header))
		m := op.Record(gtx.Ops)
		dims := paragraph.Layout(gtx, d.textState(cell), shaper, d.hovered(d.filled(style.Text, fills)), spans)
		return m.Stop(), dims.Size
	}
	if linked(cell) {
		return live(gtx)
	}

	marks := d.find.marks[cell]
	key := cellKey{style: skey, width: gtx.Constraints.Max.X, header: header}
	if len(marks) > 0 {
		// Which match is the current one decides the fill it is drawn in,
		// and that is all it decides: an unmarked cell is drawn the same at
		// every position of the find.
		key.current = d.find.current
	}
	r := d.cellRecorded(cell)
	if r.holds(key, marks) {
		d.find.found = append(d.find.found, r.found...)
		return r.call, r.size
	}

	// The record is drawn into an op list of its own, so the call survives
	// the frame's own list being reset before the next frame.
	r.ops.Reset()
	rgtx := gtx
	rgtx.Ops = r.ops
	from := len(d.find.found)
	r.call, r.size = live(rgtx)
	r.key = key
	r.marks = append(r.marks[:0], marks...)
	r.found = append(r.found[:0], d.find.found[from:]...)
	r.drawn = true
	return r.call, r.size
}
