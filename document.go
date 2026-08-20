package markdown

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	// fixed is part of Gio's text API surface (text.Parameters.PxPerEm and
	// text.Glyph metrics are fixed.Int26_6); it is already required by
	// gioui.org itself and introduces no new third-party dependency.
	"golang.org/x/image/math/fixed"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/richtext"
	"github.com/vibrantgio/components/scrollarea"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/theme/tokens"
)

// Document lays out a parsed block tree. Allocate once per document instance
// with [NewDocument] and reuse on every frame: it holds the scroll position
// and the per-block interaction state (link focus/hover, code block scroll)
// across frames.
//
// Top-level blocks are rows of a components/list, so only the blocks in the
// viewport are laid out: O(visible), not O(len(blocks)).
type Document struct {
	blocks []Block
	list   *list.State
	// text holds per-paragraph richtext link state, keyed by the pointer
	// identity of the heading, paragraph, table cell, or image it backs.
	text map[any]*richtext.State
	// code holds per-code-block horizontal scroll state.
	code map[*CodeBlock]*scrollarea.State
	// tables holds per-table horizontal scroll state, used when even the
	// min-content column widths overflow the constraint.
	tables map[*Table]*layout.List
	// images holds per-image-block provider results, so the provider and
	// texture upload run once per block, not per frame.
	images map[*Image]imageState
	// place records where each top-level block sits among its siblings, which
	// is what spaces it; see [blockPlacement].
	place map[Block]blockPlacement
	// line is one line of body text in pixels, taken from the Style the last
	// layout was given. It is the overlap a page move keeps; see move.go.
	line int
}

// imageState is a cached provider result: the widget serving a vector
// image, the uploaded raster texture, or a recorded failure that pins the
// alt-text fallback.
type imageState struct {
	widget layout.Widget
	src    paint.ImageOp
	ok     bool
}

// NewDocument returns a Document over blocks, scrolled to the top.
func NewDocument(blocks []Block) *Document {
	return &Document{
		blocks: blocks,
		list:   list.NewState(),
		text:   make(map[any]*richtext.State),
		code:   make(map[*CodeBlock]*scrollarea.State),
		tables: make(map[*Table]*layout.List),
		images: make(map[*Image]imageState),
		place:  placements(blocks),
	}
}

// NewDocumentAt returns a Document whose initial first-visible block index is
// first. Intended for golden-image testing; production code uses NewDocument
// and lets pointer events drive scrolling.
func NewDocumentAt(blocks []Block, first int) *Document {
	d := NewDocument(blocks)
	d.list = list.NewStateAt(first)
	return d
}

// Blocks returns the document's top-level blocks.
func (d *Document) Blocks() []Block { return d.blocks }

// LayoutColumn lays out every block in a natural-height vertical column with
// no internal scrolling: the document takes exactly the height its content
// needs. Intended for embedding a document inside a context that scrolls
// already — a chat message row, a card — where [Layout]'s own viewport would
// fight the outer one. It is O(len(blocks)) per frame, so it suits the short
// documents such contexts hold.
//
// [Style.EndSpace] is not spent here: the space below an embedded document's
// end belongs to whoever is scrolling it.
func (d *Document) LayoutColumn(gtx layout.Context, shaper *text.Shaper, style Style) layout.Dimensions {
	return d.column(gtx, shaper, style, d.blocks)
}

// Layout lays out the document's visible blocks in a vertical scrolling list.
//
// The shaper is the application's, typically the theme typography's cached
// [tokens.Typography.Shaper]; see the package documentation for what its
// collection must hold for Style.Mono to resolve.
func (d *Document) Layout(gtx layout.Context, shaper *text.Shaper, style Style) layout.Dimensions {
	d.recordLine(gtx, style)
	return list.Layout(gtx, d.list, d.blocks, d.row(shaper, style, style.StartSpace, style.EndSpace))
}

// LayoutScrollbar lays out the document exactly like [Layout] and additionally
// draws bar along its trailing edge, reporting where the viewport sits on the
// document and how much of it is showing. It draws nothing when the whole
// document fits, and dragging it scrolls the document.
//
// anchor decides whether the bar reserves a gutter beside the prose
// ([list.Occupy]) or floats over it ([list.Overlay]). A reading column wants
// Occupy: the gutter costs a few dp of measure once, where an overlay bar
// lands on the ends of the lines it is drawn over.
//
// The bar is the design system's — build it with [scrollbar.FromTokens] from
// the same colour tokens the [Style] came from — so a document's scrollbar is
// the same object as a list's. A document laid out with [Layout] has no
// scrollbar at all, which is what an embedder inside its own scrolling
// viewport wants.
func (d *Document) LayoutScrollbar(gtx layout.Context, shaper *text.Shaper, style Style, bar scrollbar.Style, anchor list.Anchor) layout.Dimensions {
	d.recordLine(gtx, style)
	return list.LayoutScrollbar(gtx, d.list, bar, anchor, d.blocks, d.row(shaper, style, style.StartSpace, style.EndSpace))
}

// row returns the per-block row function both list entry points lay out. start
// is added above the document's first block and end below its last, each at
// that one place and nowhere else, which is what makes them resting positions
// rather than margins every frame pays for; see [Style.StartSpace] and
// [Style.EndSpace]. They ride in the row itself so the list measures them as
// content: the scroll bounds, the page moves and the scrollbar's geometry then
// all agree about where the document begins and ends without being told.
//
// A document of one block takes both, being its own first and last.
func (d *Document) row(shaper *text.Shaper, style Style, start, end unit.Dp) func(layout.Context, Block) layout.Dimensions {
	var first, last Block
	if n := len(d.blocks); n > 0 {
		if start > 0 {
			first = d.blocks[0]
		}
		if end > 0 {
			last = d.blocks[n-1]
		}
	}
	return func(gtx layout.Context, b Block) layout.Dimensions {
		top, bottom := blockSpace(style, b, d.placeOf(b))
		if b == first {
			top += start
		}
		if b == last {
			bottom += end
		}
		return layout.Inset{Top: top, Bottom: bottom, Right: style.Gutter}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return d.block(gtx, shaper, style, b)
		})
	}
}

// blockPlacement is where a block sits among its siblings, which is what damps
// the space around it. A heading opening the run has nothing above to separate
// itself from; two headings in a row are one announcement rather than two
// sections, so the pair closes up from both sides — the shaped lines alone
// leave a heading's descent and the next heading's ascent between them, which
// is already as much blank as a section break carries. A paragraph with a list
// directly under it is announcing that list, and closes up towards it.
//
// The zero value is the ordinary block, spaced by the block gap alone.
type blockPlacement struct {
	// first marks the block that opens the run.
	first bool
	// under marks a heading directly below another heading.
	under bool
	// over marks a heading directly above another heading.
	over bool
	// announcing marks a paragraph directly above a list.
	announcing bool
}

// placements records where every block among blocks sits, keeping only the ones
// with somewhere to be. A document's blocks are fixed for its life, so this is
// computed once at construction: the row function sees one block at a time and
// cannot tell what surrounds it.
func placements(blocks []Block) map[Block]blockPlacement {
	m := make(map[Block]blockPlacement)
	for i, b := range blocks {
		if p := placement(blocks, i); p != (blockPlacement{}) {
			m[b] = p
		}
	}
	return m
}

// placement classifies the block at index i of blocks.
func placement(blocks []Block, i int) blockPlacement {
	p := blockPlacement{first: i == 0}
	if i > 0 {
		_, p.under = blocks[i-1].(*Heading)
	}
	if i < len(blocks)-1 {
		_, p.over = blocks[i+1].(*Heading)
		if _, ok := blocks[i].(*Paragraph); ok {
			_, p.announcing = blocks[i+1].(*List)
		}
	}
	return p
}

// placeOf returns the recorded placement of a top-level block; the zero value
// for the blocks that have no placement worth recording.
func (d *Document) placeOf(b Block) blockPlacement {
	return d.place[b]
}

// blockSpace returns the vertical space a block puts above and below itself.
// An ordinary block closes with BlockGap and reaches nothing above it, so the
// space between two of them is one gap. A heading closes with its own,
// tighter space instead, and reaches above the gap by however much its space
// above exceeds one — opening none at all where it is the run's first block
// or sits directly under another heading, and closing with half its space
// where the next block is a heading. A paragraph announcing the list under it
// closes with the seam instead of the gap.
//
// The seam is spent by the paragraph rather than claimed by the list, which is
// what keeps it a plain space: the block above has not closed yet when the
// decision is made, where a list claiming the seam would have to claim a
// negative one against a gap already spent.
func blockSpace(style Style, b Block, p blockPlacement) (top, bottom unit.Dp) {
	h, ok := b.(*Heading)
	if !ok {
		if p.announcing {
			return 0, style.listSpace()
		}
		return 0, style.BlockGap
	}
	above, below := style.headingSpace(h.Level)
	top = above - style.BlockGap
	if p.first || p.under {
		top = 0
	}
	if p.over {
		below /= 2
	}
	return top, below
}

// block dispatches one block to its widget.
func (d *Document) block(gtx layout.Context, shaper *text.Shaper, style Style, b Block) layout.Dimensions {
	switch b := b.(type) {
	case *Heading:
		h := style.heading(b.Level)
		return richtext.Layout(gtx, d.textState(b), shaper, h, style.spanStyles(b.Spans, font.Bold, h.Size))
	case *Paragraph:
		return richtext.Layout(gtx, d.textState(b), shaper, style.Text, style.spanStyles(b.Spans, font.Normal, style.Text.Size))
	case *List:
		return d.listBlock(gtx, shaper, style, b)
	case *Blockquote:
		return d.blockquote(gtx, shaper, style, b)
	case *CodeBlock:
		return d.codeBlock(gtx, shaper, style, b)
	case *Rule:
		return rule(gtx, style)
	case *Table:
		return d.table(gtx, shaper, style, b)
	case *Image:
		return d.image(gtx, shaper, style, b)
	}
	return layout.Dimensions{}
}

// column stacks blocks vertically, spacing each pair the way the list rows
// are spaced — the ordinary gap between ordinary blocks, a heading's own
// asymmetric space around a heading — and returns the union size. Nothing is
// added above the first block or below the last: an embedded column takes
// exactly its content's height.
func (d *Document) column(gtx layout.Context, shaper *text.Shaper, style Style, blocks []Block) layout.Dimensions {
	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	var size image.Point
	closing := 0 // the space the previous block closes with
	for i, b := range blocks {
		top, bottom := blockSpace(style, b, placement(blocks, i))
		if i > 0 {
			size.Y += closing + gtx.Dp(top)
		}
		tr := op.Offset(image.Pt(0, size.Y)).Push(gtx.Ops)
		dims := d.block(cgtx, shaper, style, b)
		tr.Pop()
		size.Y += dims.Size.Y
		size.X = max(size.X, dims.Size.X)
		closing = gtx.Dp(bottom)
	}
	return layout.Dimensions{Size: size}
}

// textState returns the persistent richtext state for a heading, paragraph,
// table cell, or image fallback, keyed by pointer identity.
func (d *Document) textState(b any) *richtext.State {
	s, ok := d.text[b]
	if !ok {
		s = richtext.NewState()
		d.text[b] = s
	}
	return s
}

// codeState returns the persistent horizontal scroll state for a code block.
func (d *Document) codeState(b *CodeBlock) *scrollarea.State {
	s, ok := d.code[b]
	if !ok {
		s = scrollarea.NewState()
		d.code[b] = s
	}
	return s
}

// tableState returns the persistent horizontal scroll state for a table.
func (d *Document) tableState(b *Table) *layout.List {
	l, ok := d.tables[b]
	if !ok {
		l = &layout.List{Axis: layout.Horizontal}
		d.tables[b] = l
	}
	return l
}

// quoteBarWidth is the width of the bar leading a blockquote.
const quoteBarWidth = unit.Dp(3)

// codeEdge is how wide the hairline around a fence is drawn when
// [Style].CodeBorder asks for one. It is the width every other line in a
// document is drawn at — a thematic break, a table's rules — because an edge
// that has to be seen is not the same thing as an edge that has to be
// noticed: the block is already a block, and the line only has to say where
// it ends.
const codeEdge = unit.Dp(1)

// blockquote renders a quoted group as an inset column with a leading
// token-coloured bar spanning the content height. Text inside the quote uses
// the muted QuoteColor; nested quotes recurse.
func (d *Document) blockquote(gtx layout.Context, shaper *text.Shaper, style Style, q *Blockquote) layout.Dimensions {
	qs := style
	qs.Text.Color = style.QuoteColor
	inset := gtx.Dp(unit.Dp(tokens.Spacing.S3))

	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	cgtx.Constraints.Max.X -= inset
	macro := op.Record(gtx.Ops)
	content := d.column(cgtx, shaper, qs, q.Blocks)
	call := macro.Stop()

	paint.FillShape(gtx.Ops, style.QuoteBar, clip.Rect{
		Max: image.Pt(gtx.Dp(quoteBarWidth), content.Size.Y),
	}.Op())
	tr := op.Offset(image.Pt(inset, 0)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	tr.Pop()

	return layout.Dimensions{Size: image.Pt(inset+content.Size.X, content.Size.Y)}
}

// codeBlock renders monospace code on a rounded surface-coloured background
// spanning the full width, with tabs already expanded at parse time.
//
// A code block's own line breaks are the code, so a line too wide for the
// column is never reflowed and never cut away: the block is a horizontal
// scroll area (components/scrollarea), and the part that does not fit is
// scrolled to. The fence's padding is inside the scroll area rather than
// around it, which puts the block's whole box on the horizontal axis: the
// leading padding scrolls away with the first column of code, the dissolve
// that marks a cut edge runs the full height of the fence, and the bar the
// area draws while it scrolls lands in the bottom padding, where it covers no
// code. A block that fits lays out exactly as it would with no scroll area at
// all — same height, same clip, no bar, no dissolve.
//
// A Style.CodeBorder with any alpha in it edges the fence: the border colour
// fills the block's whole rounded box and the ground fills a box one hairline
// smaller, concentric with it, so the rim is drawn without a stroke and
// without a seam. The ground's box is also what the content is clipped to,
// which is what keeps the dissolve at a cut edge off the rim it would
// otherwise paint over. The block's own size is the outer box either way, so
// edging one moves nothing below it.
func (d *Document) codeBlock(gtx layout.Context, shaper *text.Shaper, style Style, cb *CodeBlock) layout.Dimensions {
	pad := unit.Dp(tokens.Spacing.S3)
	radius := gtx.Dp(unit.Dp(tokens.Radius.Base))
	codeStyle := richtext.Style{Color: style.CodeColor, Size: style.CodeSize}
	spans := style.codeSpans(cb)
	area := scrollarea.Style{Fade: unit.Dp(tokens.Spacing.S4), FadeColor: style.CodeBackground}

	code := func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(pad).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return richtext.Render(shaper, codeStyle, spans, richtext.Idle())(gtx)
		})
	}
	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	// The padding is drawn inside the area but is not part of the height the
	// code has to fit in: a fence takes its content's height and pads it,
	// rather than making the content fit a viewport two paddings shorter.
	cgtx.Constraints.Max.Y += 2 * gtx.Dp(pad)
	state := d.codeState(cb)
	macro := op.Record(gtx.Ops)
	var content layout.Dimensions
	if style.CodeScrollbar.Width() > 0 {
		content = area.LayoutScrollbar(cgtx, state, style.CodeScrollbar, code)
	} else {
		content = area.Layout(cgtx, state, code)
	}
	call := macro.Stop()

	total := image.Pt(gtx.Constraints.Max.X, content.Size.Y)
	box := image.Rectangle{Max: total}
	fence := clip.UniformRRect(box, radius)
	if style.CodeBorder.A > 0 {
		edge := max(gtx.Dp(codeEdge), 1)
		paint.FillShape(gtx.Ops, style.CodeBorder, fence.Op(gtx.Ops))
		fence = clip.UniformRRect(box.Inset(edge), max(radius-edge, 0))
	}
	paint.FillShape(gtx.Ops, style.CodeBackground, fence.Op(gtx.Ops))
	if state.Overflows() {
		// The dissolve at a cut edge is opaque where it meets that edge, so
		// left unclipped it would square off the two corners it runs into.
		// Only an overflowing fence draws one — a fence that fits reaches no
		// corner, and clipping it would cost the corners' anti-aliasing a
		// pixel for nothing.
		defer fence.Push(gtx.Ops).Pop()
	}
	call.Add(gtx.Ops)

	return layout.Dimensions{Size: total}
}

// rule renders a thematic break: a full-width 1 dp line, and nothing else.
//
// It used to pad itself by a block gap on each side, from a time when the gap
// was the smallest stop that separates two widgets and a break spaced like an
// ordinary block would have read as a stray line. The reading rhythm is wide
// enough now to say "section break" on its own, and doubling it around the
// line only opened a hole: the break is a block, and the rhythm spaces it like
// every other block.
func rule(gtx layout.Context, style Style) layout.Dimensions {
	w := gtx.Constraints.Max.X
	th := max(gtx.Dp(1), 1)
	paint.FillShape(gtx.Ops, style.RuleColor, clip.Rect{Max: image.Pt(w, th)}.Op())
	return layout.Dimensions{Size: image.Pt(w, th)}
}

// cellSpans returns a table cell's richtext spans; header cells are
// emphasised with the bold run weight.
func cellSpans(style Style, cell *TableCell, header bool) []richtext.SpanStyle {
	w := font.Normal
	if header {
		w = font.Bold
	}
	return style.spanStyles(cell.Spans, w, style.Text.Size)
}

// table renders a GFM table as a grid: the emphasised header row on the
// TableHeaderBackground surface above the body rows, ruled by 1 dp
// TableBorder lines. Each column takes its widest cell's natural width;
// when the grid would overflow the constraint, columns shrink towards —
// but never below — their min-content width (the widest single word), the
// deficit removed from the columns' slack in proportion, so prose columns
// wrap while narrow columns keep their longest word intact. When even the
// min-content widths overflow, the grid scrolls horizontally like a code
// block. Cell content honours the column alignment.
func (d *Document) table(gtx layout.Context, shaper *text.Shaper, style Style, t *Table) layout.Dimensions {
	cols := len(t.Header)
	if cols == 0 || cols != len(t.Alignments) {
		return layout.Dimensions{}
	}
	rows := make([][]*TableCell, 0, len(t.Rows)+1)
	rows = append(rows, t.Header)
	rows = append(rows, t.Rows...)

	border := max(gtx.Dp(1), 1)
	pad := gtx.Dp(unit.Dp(tokens.Spacing.S2))
	avail := max(gtx.Constraints.Max.X-(cols+1)*border-cols*2*pad, 0)

	// Measure pass: record each cell at the full content width, discard the
	// ops, and keep the widest natural width per column.
	naturals := make([]int, cols)
	mgtx := gtx
	mgtx.Constraints.Min = image.Point{}
	mgtx.Constraints.Max.X = avail
	for ri, row := range rows {
		for ci, cell := range row {
			if ci >= cols {
				break
			}
			m := op.Record(gtx.Ops)
			dims := richtext.Render(shaper, style.Text, cellSpans(style, cell, ri == 0), richtext.Idle())(mgtx)
			m.Stop()
			naturals[ci] = max(naturals[ci], dims.Size.X)
		}
	}
	widths := naturals
	total := 0
	for _, w := range naturals {
		total += w
	}
	if total > avail {
		widths = distributeWidths(naturals, minColumnWidths(gtx, shaper, style, rows, cols, naturals), avail)
		total = 0
		for _, w := range widths {
			total += w
		}
	}
	tableW := total + (cols+1)*border + cols*2*pad

	if tableW <= gtx.Constraints.Max.X {
		return d.tableGrid(gtx, shaper, style, t, rows, widths)
	}
	// Even the min-content widths overflow: scroll the grid horizontally,
	// clipped to the viewport, like an over-wide code block.
	return d.tableState(t).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return d.tableGrid(gtx, shaper, style, t, rows, widths)
	})
}

// minColumnWidths measures each column's min-content width: the widest
// single whitespace-separated word across its cells, in the cell's span
// styling. Only called when the natural widths overflow, so the fitting
// fast path pays nothing for it.
func minColumnWidths(gtx layout.Context, shaper *text.Shaper, style Style, rows [][]*TableCell, cols int, naturals []int) []int {
	mgtx := gtx
	mgtx.Constraints.Min = image.Point{}
	mgtx.Constraints.Max = image.Pt(1e6, 1e6)
	mins := make([]int, cols)
	for ri, row := range rows {
		for ci, cell := range row {
			if ci >= cols {
				break
			}
			for _, sp := range cellSpans(style, cell, ri == 0) {
				for _, word := range strings.Fields(sp.Content) {
					w := sp
					w.Content = word
					m := op.Record(gtx.Ops)
					dims := richtext.Render(shaper, style.Text, []richtext.SpanStyle{w}, richtext.Idle())(mgtx)
					m.Stop()
					mins[ci] = max(mins[ci], dims.Size.X)
				}
			}
		}
	}
	for i := range mins {
		mins[i] = min(mins[i], naturals[i])
	}
	return mins
}

// distributeWidths sizes columns for avail total content width: natural
// widths when they fit, otherwise each column shrunk towards — but never
// below — its min-content width, the deficit removed from the columns'
// slack (natural − min) in proportion. When even the minima overflow every
// column gets its minimum and the caller lets the grid overflow.
func distributeWidths(naturals, mins []int, avail int) []int {
	total, totalMin := 0, 0
	for i := range naturals {
		total += naturals[i]
		totalMin += mins[i]
	}
	widths := make([]int, len(naturals))
	switch {
	case total <= avail:
		copy(widths, naturals)
	case totalMin >= avail:
		copy(widths, mins)
	default:
		slack := total - totalMin
		extra := avail - totalMin
		rem := avail
		for i := range widths {
			widths[i] = mins[i] + (naturals[i]-mins[i])*extra/slack
			rem -= widths[i]
		}
		// Integer division under-fills; hand the remainder to columns that
		// still have slack.
		for rem > 0 {
			grown := false
			for i := range widths {
				if rem > 0 && widths[i] < naturals[i] {
					widths[i]++
					rem--
					grown = true
				}
			}
			if !grown {
				break
			}
		}
	}
	return widths
}

// tableGrid lays out the grid at the given column widths.
func (d *Document) tableGrid(gtx layout.Context, shaper *text.Shaper, style Style, t *Table, rows [][]*TableCell, widths []int) layout.Dimensions {
	cols := len(widths)
	border := max(gtx.Dp(1), 1)
	pad := gtx.Dp(unit.Dp(tokens.Spacing.S2))
	total := 0
	for _, w := range widths {
		total += w
	}
	tableW := total + (cols+1)*border + cols*2*pad

	// Rows top to bottom, one horizontal rule above each and one below the
	// last; vertical rules span the finished height at the end.
	y := 0
	hline := func() {
		paint.FillShape(gtx.Ops, style.TableBorder, clip.Rect{
			Min: image.Pt(0, y),
			Max: image.Pt(tableW, y+border),
		}.Op())
		y += border
	}
	calls := make([]op.CallOp, cols)
	sizes := make([]image.Point, cols)
	for ri, row := range rows {
		hline()
		rowH := 0
		cgtx := gtx
		cgtx.Constraints.Min = image.Point{}
		for ci, cell := range row {
			if ci >= cols {
				break
			}
			cgtx.Constraints.Max.X = widths[ci]
			m := op.Record(gtx.Ops)
			dims := richtext.Layout(cgtx, d.textState(cell), shaper, style.Text, cellSpans(style, cell, ri == 0))
			calls[ci] = m.Stop()
			sizes[ci] = dims.Size
			rowH = max(rowH, dims.Size.Y)
		}
		if ri == 0 {
			paint.FillShape(gtx.Ops, style.TableHeaderBackground, clip.Rect{
				Min: image.Pt(border, y),
				Max: image.Pt(tableW-border, y+rowH+2*pad),
			}.Op())
		}
		x := border
		for ci := range min(len(row), cols) {
			dx := 0
			switch t.Alignments[ci] {
			case AlignCenter:
				dx = max((widths[ci]-sizes[ci].X)/2, 0)
			case AlignRight:
				dx = max(widths[ci]-sizes[ci].X, 0)
			}
			// Clip to the padded cell box so degenerate content can never
			// paint across the rule into the neighbouring column.
			cl := clip.Rect{
				Min: image.Pt(x, y),
				Max: image.Pt(x+2*pad+widths[ci], y+2*pad+rowH),
			}.Push(gtx.Ops)
			tr := op.Offset(image.Pt(x+pad+dx, y+pad)).Push(gtx.Ops)
			calls[ci].Add(gtx.Ops)
			tr.Pop()
			cl.Pop()
			x += widths[ci] + 2*pad + border
		}
		y += rowH + 2*pad
	}
	hline()
	tableH := y

	x := 0
	for ci := 0; ci <= cols; ci++ {
		paint.FillShape(gtx.Ops, style.TableBorder, clip.Rect{
			Min: image.Pt(x, 0),
			Max: image.Pt(x+border, tableH),
		}.Op())
		if ci < cols {
			x += border + widths[ci] + 2*pad
		}
	}
	return layout.Dimensions{Size: image.Pt(tableW, tableH)}
}

// image renders a block image through the style's provider, scaled down (never
// up) to fit the width constraint. Without a provider, or when it fails, the
// alt text — or the URL when the alt is empty — renders as an italic
// paragraph. The provider result is cached per block.
func (d *Document) image(gtx layout.Context, shaper *text.Shaper, style Style, n *Image) layout.Dimensions {
	st, cached := d.images[n]
	if !cached {
		if wp, ok := style.Images.(WidgetImageProvider); ok {
			if w, err := wp.ImageWidget(n.URL); err == nil && w != nil {
				st = imageState{widget: w, ok: true}
			}
		}
		if !st.ok && style.Images != nil {
			if img, err := style.Images.Image(n.URL); err == nil && img != nil {
				st = imageState{src: paint.NewImageOp(img), ok: true}
			}
		}
		d.images[n] = st
	}
	if !st.ok {
		alt := n.Alt
		if alt == "" {
			alt = n.URL
		}
		spans := []richtext.SpanStyle{{Content: alt, Style: font.Italic}}
		return richtext.Layout(gtx, d.textState(n), shaper, style.Text, spans)
	}
	if st.widget != nil {
		return st.widget(gtx)
	}
	return widget.Image{
		Src:      st.src,
		Fit:      widget.ScaleDown,
		Position: layout.NW,
	}.Layout(gtx)
}

// listBlock renders a list's items as marker-plus-content rows; nested lists
// recurse through the item content column, indenting one marker column per
// level.
//
// A whole list is one block of the reading flow, so the reading rhythm stands
// above and below it and nowhere inside it: the items are spaced by the list's
// own compact stop, and so are the blocks within an item — a continuation
// paragraph, or the nested list a deeper level opens with. Spacing those by
// the block gap would put as much air between an item and its own sub-items as
// between two paragraphs, which reads as several lists rather than one.
func (d *Document) listBlock(gtx layout.Context, shaper *text.Shaper, style Style, l *List) layout.Dimensions {
	gap := gtx.Dp(unit.Dp(tokens.Spacing.S1))
	style = style.compact(unit.Dp(tokens.Spacing.S2))
	// One reading of the shaped body line for the whole list: every item's
	// first line is set in the same style, so every marker hangs from the
	// same anchor.
	line := firstLine(gtx, shaper, style)
	var size image.Point
	for i, item := range l.Items {
		tr := op.Offset(image.Pt(0, size.Y)).Push(gtx.Ops)
		dims := d.listItem(gtx, shaper, style, l, item, i, line)
		tr.Pop()
		size.Y += dims.Size.Y
		if i < len(l.Items)-1 {
			size.Y += gap
		}
		size.X = max(size.X, dims.Size.X)
	}
	return layout.Dimensions{Size: size}
}

// listItem renders one item: its marker (bullet, number, or task checkbox)
// in a fixed Indent-wide column, then the content blocks. line is the
// geometry of the item's first text line, which anchors the marker.
func (d *Document) listItem(gtx layout.Context, shaper *text.Shaper, style Style, l *List, item *ListItem, i int, line lineGeometry) layout.Dimensions {
	markerW := gtx.Dp(style.Indent)

	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	cgtx.Constraints.Max.X = max(cgtx.Constraints.Max.X-markerW, 0)
	macro := op.Record(gtx.Ops)
	content := d.column(cgtx, shaper, style, item.Blocks)
	call := macro.Stop()

	switch {
	case item.Task:
		drawCheckbox(gtx, style, item.Checked, line.center)
	case l.Ordered:
		marker := fmt.Sprintf("%d.", l.Start+i)
		mgtx := gtx
		mgtx.Constraints.Min = image.Point{}
		mgtx.Constraints.Max.X = markerW
		richtext.Render(shaper, style.Text, []richtext.SpanStyle{{Content: marker}}, richtext.Idle())(mgtx)
	default:
		drawBullet(gtx, style, line.center)
	}

	tr := op.Offset(image.Pt(markerW, 0)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	tr.Pop()

	h := max(content.Size.Y, line.height)
	return layout.Dimensions{Size: image.Pt(markerW+content.Size.X, h)}
}

// lineGeometry is the vertical geometry of an item's first text line, in
// pixels below the item's top.
type lineGeometry struct {
	// height is the whole line box: the paragraph's line height, or the
	// shaped ascent plus descent where that is the taller of the two.
	height int
	// center is the middle of the cap band: the strip from the tops of the
	// capitals down to the baseline.
	center int
}

// capProbe is the string firstLine shapes to read the body face's cap height.
// A capital with flat terminals gives the band its true top, free of the
// overshoot a round letter adds.
const capProbe = "H"

// firstLine reports where an item's first text line sits, so a marker beside
// it can be hung from the shaped line rather than from an approximation of
// it. Shaping is the only honest source: a line is taller than its text size
// by the leading the face asks for, so anchoring to the size alone rides
// every marker high — most visibly the checkbox, which is nearly as tall as
// the text and so has the least room to hide the error.
//
// The anchor is the centre of the cap band, not of the line's whole ink.
// Ascenders and descenders come and go with the words, which would make a
// marker wander from line to line; the capitals' band is where a line's
// weight sits whatever it says. A line that opens with taller ink — inline
// code, say — stretches below this band, and the marker stays with the body
// line it belongs to.
//
// The probe is shaped in the paragraph's own face and size, matching what
// the paragraph is laid out with, so the two agree at any scale.
//
// The line box the paragraph sets its lines in is part of that agreement. A
// paragraph whose style names a line height taller than its shaped metrics
// splits the surplus around the ink, half above and half below, so its first
// baseline sits half a leading lower than the metrics alone would put it —
// and a marker anchored to the metrics alone would ride exactly that far
// high. The same split is applied here, on the same rounding, so the anchor
// tracks the line whatever box it is set in.
func firstLine(gtx layout.Context, shaper *text.Shaper, style Style) lineGeometry {
	px := gtx.Sp(style.Text.Size)
	shaper.LayoutString(text.Parameters{
		PxPerEm: fixed.I(px),
		Locale:  gtx.Locale,
	}, capProbe)
	g, ok := shaper.NextGlyph()
	// Drain the run so the shaper is not left mid-iteration for its next
	// caller.
	for more := ok; more; {
		_, more = shaper.NextGlyph()
	}
	if !ok {
		// No face could shape the probe: fall back to the text size, which is
		// at least the right order of magnitude.
		return lineGeometry{height: px, center: px / 2}
	}
	// Glyph bounds are relative to the dot, y down, so the cap band reaches
	// -Bounds.Min.Y above the baseline; the baseline itself is the glyph's
	// document y, the same quantity a paragraph's first line is drawn at.
	baseline := int(g.Y)
	capBand := -g.Bounds.Min.Y
	// The shaped line's own box, which is what the paragraph's leading is
	// measured against: the line's ascent and descent, both rounded outwards,
	// exactly as the paragraph measures them.
	natural := g.Ascent.Ceil() + g.Descent.Ceil()
	height := natural
	above := 0
	if box := gtx.Sp(style.Text.LineHeight); box > natural {
		above = (box - natural) / 2
		height = box
	}
	return lineGeometry{
		height: height,
		center: (fixed.I(above+baseline) - capBand/2).Round(),
	}
}

// drawBullet paints an unordered item's marker: a small filled disc centred
// on the first text line.
func drawBullet(gtx layout.Context, style Style, center int) {
	r := gtx.Dp(unit.Dp(2.5))
	cx := gtx.Dp(unit.Dp(4))
	paint.FillShape(gtx.Ops, style.Text.Color, clip.Ellipse{
		Min: image.Pt(cx-r, center-r),
		Max: image.Pt(cx+r, center+r),
	}.Op(gtx.Ops))
}

// drawCheckbox paints a task item's checkbox, centred on the first text line:
// a rounded outline when unchecked, a filled box with a check mark when
// checked. The checkbox is display-only — GFM task state belongs to the
// document, not the reader.
func drawCheckbox(gtx layout.Context, style Style, checked bool, center int) {
	sz := gtx.Dp(14)
	top := max(center-sz/2, 0)
	tr := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	defer tr.Pop()
	box := clip.UniformRRect(image.Rectangle{Max: image.Pt(sz, sz)}, gtx.Dp(unit.Dp(tokens.Radius.Sm)))
	if !checked {
		paint.FillShape(gtx.Ops, style.CheckboxColor, clip.Stroke{
			Path:  box.Path(gtx.Ops),
			Width: float32(max(gtx.Dp(1), 1)) * 1.5,
		}.Op())
		return
	}
	paint.FillShape(gtx.Ops, style.CheckboxColor, box.Op(gtx.Ops))
	s := float32(sz)
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(0.24*s, 0.52*s))
	p.LineTo(f32.Pt(0.44*s, 0.72*s))
	p.LineTo(f32.Pt(0.78*s, 0.28*s))
	paint.FillShape(gtx.Ops, style.CheckmarkColor, clip.Stroke{
		Path:  p.End(),
		Width: float32(max(gtx.Dp(2), 2)),
	}.Op())
}
