package markdown

import (
	"fmt"
	"image"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/prism/list"
	"github.com/vibrantgio/prism/richtext"
	"github.com/vibrantgio/prism/tokens"
)

// Document lays out a parsed block tree. Allocate once per document instance
// with [NewDocument] and reuse on every frame: it holds the scroll position
// and the per-block interaction state (link focus/hover, code block scroll)
// across frames.
//
// Top-level blocks are rows of a prism/list, so only the blocks in the
// viewport are laid out: O(visible), not O(len(blocks)).
type Document struct {
	blocks []Block
	list   *list.State
	// text holds per-paragraph richtext link state, keyed by the pointer
	// identity of the heading, paragraph, table cell, or image it backs.
	text map[any]*richtext.State
	// code holds per-code-block horizontal scroll state.
	code map[*CodeBlock]*layout.List
	// images holds per-image-block provider results, so the provider and
	// texture upload run once per block, not per frame.
	images map[*Image]imageState
}

// imageState is a cached ImageProvider result: the uploaded texture, or a
// recorded failure that pins the alt-text fallback.
type imageState struct {
	src paint.ImageOp
	ok  bool
}

// NewDocument returns a Document over blocks, scrolled to the top.
func NewDocument(blocks []Block) *Document {
	return &Document{
		blocks: blocks,
		list:   list.NewState(),
		text:   make(map[any]*richtext.State),
		code:   make(map[*CodeBlock]*layout.List),
		images: make(map[*Image]imageState),
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
func (d *Document) LayoutColumn(gtx layout.Context, shaper *text.Shaper, style Style) layout.Dimensions {
	return d.column(gtx, shaper, style, d.blocks)
}

// Layout lays out the document's visible blocks in a vertical scrolling list.
func (d *Document) Layout(gtx layout.Context, shaper *text.Shaper, style Style) layout.Dimensions {
	return list.Layout(gtx, d.list, d.blocks, func(gtx layout.Context, b Block) layout.Dimensions {
		return layout.Inset{Bottom: style.BlockGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return d.block(gtx, shaper, style, b)
		})
	})
}

// block dispatches one block to its widget.
func (d *Document) block(gtx layout.Context, shaper *text.Shaper, style Style, b Block) layout.Dimensions {
	switch b := b.(type) {
	case *Heading:
		return richtext.Layout(gtx, d.textState(b), shaper, style.heading(b.Level), style.spanStyles(b.Spans, font.Bold))
	case *Paragraph:
		return richtext.Layout(gtx, d.textState(b), shaper, style.Text, style.spanStyles(b.Spans, font.Normal))
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

// column stacks blocks vertically with BlockGap between them, returning the
// union size.
func (d *Document) column(gtx layout.Context, shaper *text.Shaper, style Style, blocks []Block) layout.Dimensions {
	gap := gtx.Dp(style.BlockGap)
	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	var size image.Point
	for i, b := range blocks {
		tr := op.Offset(image.Pt(0, size.Y)).Push(gtx.Ops)
		dims := d.block(cgtx, shaper, style, b)
		tr.Pop()
		size.Y += dims.Size.Y
		if i < len(blocks)-1 {
			size.Y += gap
		}
		size.X = max(size.X, dims.Size.X)
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
func (d *Document) codeState(b *CodeBlock) *layout.List {
	l, ok := d.code[b]
	if !ok {
		l = &layout.List{Axis: layout.Horizontal}
		d.code[b] = l
	}
	return l
}

// quoteBarWidth is the width of the bar leading a blockquote.
const quoteBarWidth = unit.Dp(3)

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
// spanning the full width, with tabs already expanded at parse time and
// horizontal overflow scrolling instead of wrapping.
func (d *Document) codeBlock(gtx layout.Context, shaper *text.Shaper, style Style, cb *CodeBlock) layout.Dimensions {
	pad := gtx.Dp(unit.Dp(tokens.Spacing.S3))
	radius := gtx.Dp(unit.Dp(tokens.Radius.Base))
	codeStyle := richtext.Style{Color: style.CodeColor, Size: style.CodeSize}
	spans := style.codeSpans(cb)

	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	cgtx.Constraints.Max.X = max(cgtx.Constraints.Max.X-2*pad, 0)
	macro := op.Record(gtx.Ops)
	// The horizontal list gives the code unbounded width — hard newlines
	// still break lines, over-wide lines scroll instead of wrapping — and
	// clips it to the viewport.
	content := d.codeState(cb).Layout(cgtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return richtext.Render(shaper, codeStyle, spans, richtext.Idle())(gtx)
	})
	call := macro.Stop()

	total := image.Pt(gtx.Constraints.Max.X, content.Size.Y+2*pad)
	paint.FillShape(gtx.Ops, style.CodeBackground,
		clip.UniformRRect(image.Rectangle{Max: total}, radius).Op(gtx.Ops))
	tr := op.Offset(image.Pt(pad, pad)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	tr.Pop()

	return layout.Dimensions{Size: total}
}

// rule renders a thematic break: a full-width 1 dp line with BlockGap
// padding above and below.
func rule(gtx layout.Context, style Style) layout.Dimensions {
	w := gtx.Constraints.Max.X
	th := max(gtx.Dp(1), 1)
	pad := gtx.Dp(style.BlockGap)
	paint.FillShape(gtx.Ops, style.RuleColor, clip.Rect{
		Min: image.Pt(0, pad),
		Max: image.Pt(w, pad+th),
	}.Op())
	return layout.Dimensions{Size: image.Pt(w, 2*pad+th)}
}

// cellSpans returns a table cell's richtext spans; header cells are
// emphasised with the bold run weight.
func cellSpans(style Style, cell *TableCell, header bool) []richtext.SpanStyle {
	w := font.Normal
	if header {
		w = font.Bold
	}
	return style.spanStyles(cell.Spans, w)
}

// table renders a GFM table as a grid: the emphasised header row on the
// TableHeaderBackground surface above the body rows, ruled by 1 dp
// TableBorder lines. Each column takes its widest cell's natural width,
// shrunk proportionally when the grid would overflow the constraint, and
// cell content honours the column alignment.
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
	widths := make([]int, cols)
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
			widths[ci] = max(widths[ci], dims.Size.X)
		}
	}
	total := 0
	for _, w := range widths {
		total += w
	}
	if total > avail && total > 0 {
		for i, w := range widths {
			widths[i] = w * avail / total
		}
		total = 0
		for _, w := range widths {
			total += w
		}
	}
	tableW := total + (cols+1)*border + cols*2*pad

	// Layout pass: rows top to bottom, one horizontal rule above each and one
	// below the last; vertical rules span the finished height at the end.
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
			tr := op.Offset(image.Pt(x+pad+dx, y+pad)).Push(gtx.Ops)
			calls[ci].Add(gtx.Ops)
			tr.Pop()
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
		if style.Images != nil {
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
	return widget.Image{
		Src:      st.src,
		Fit:      widget.ScaleDown,
		Position: layout.NW,
	}.Layout(gtx)
}

// listBlock renders a list's items as marker-plus-content rows; nested lists
// recurse through the item content column, indenting one marker column per
// level.
func (d *Document) listBlock(gtx layout.Context, shaper *text.Shaper, style Style, l *List) layout.Dimensions {
	gap := gtx.Dp(unit.Dp(tokens.Spacing.S1))
	var size image.Point
	for i, item := range l.Items {
		tr := op.Offset(image.Pt(0, size.Y)).Push(gtx.Ops)
		dims := d.listItem(gtx, shaper, style, l, item, i)
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
// in a fixed Indent-wide column, then the content blocks.
func (d *Document) listItem(gtx layout.Context, shaper *text.Shaper, style Style, l *List, item *ListItem, i int) layout.Dimensions {
	markerW := gtx.Dp(style.Indent)

	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	cgtx.Constraints.Max.X = max(cgtx.Constraints.Max.X-markerW, 0)
	macro := op.Record(gtx.Ops)
	content := d.column(cgtx, shaper, style, item.Blocks)
	call := macro.Stop()

	// lineH approximates the first text line's height, anchoring the marker
	// vertically.
	lineH := gtx.Sp(style.Text.Size)
	switch {
	case item.Task:
		drawCheckbox(gtx, style, item.Checked, lineH)
	case l.Ordered:
		marker := fmt.Sprintf("%d.", l.Start+i)
		mgtx := gtx
		mgtx.Constraints.Min = image.Point{}
		mgtx.Constraints.Max.X = markerW
		richtext.Render(shaper, style.Text, []richtext.SpanStyle{{Content: marker}}, richtext.Idle())(mgtx)
	default:
		drawBullet(gtx, style, lineH)
	}

	tr := op.Offset(image.Pt(markerW, 0)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	tr.Pop()

	h := max(content.Size.Y, lineH)
	return layout.Dimensions{Size: image.Pt(markerW+content.Size.X, h)}
}

// drawBullet paints an unordered item's marker: a small filled disc centred
// on the first text line.
func drawBullet(gtx layout.Context, style Style, lineH int) {
	r := gtx.Dp(unit.Dp(2.5))
	cx, cy := gtx.Dp(unit.Dp(4)), lineH*11/20
	paint.FillShape(gtx.Ops, style.Text.Color, clip.Ellipse{
		Min: image.Pt(cx-r, cy-r),
		Max: image.Pt(cx+r, cy+r),
	}.Op(gtx.Ops))
}

// drawCheckbox paints a task item's checkbox, centred on the first text line:
// a rounded outline when unchecked, a filled box with a check mark when
// checked. The checkbox is display-only — GFM task state belongs to the
// document, not the reader.
func drawCheckbox(gtx layout.Context, style Style, checked bool, lineH int) {
	sz := gtx.Dp(14)
	top := max((lineH-sz)*3/4, 0)
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
