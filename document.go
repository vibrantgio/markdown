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
	// text holds per-heading/paragraph richtext link state, keyed by the
	// block's pointer identity.
	text map[Block]*richtext.State
	// code holds per-code-block horizontal scroll state.
	code map[*CodeBlock]*layout.List
}

// NewDocument returns a Document over blocks, scrolled to the top.
func NewDocument(blocks []Block) *Document {
	return &Document{
		blocks: blocks,
		list:   list.NewState(),
		text:   make(map[Block]*richtext.State),
		code:   make(map[*CodeBlock]*layout.List),
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

// textState returns the persistent richtext state for a heading or paragraph.
func (d *Document) textState(b Block) *richtext.State {
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
	spans := []richtext.SpanStyle{{Content: cb.Code, Typeface: style.Mono}}

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
