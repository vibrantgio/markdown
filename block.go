// Package markdown renders markdown documents with prism primitives
// (DESIGN §Markdown).
//
// [Parse] walks a goldmark AST (with extension.GFM) into a block model — a
// tree of [Block] values whose inline content is expressed as styled [Span]
// runs. [Document] lays the top-level blocks through prism/list, so long
// documents stay O(visible), and renders each block with prism widgets:
// type-scale headings, richtext paragraphs, nested lists with task-list
// checkboxes, inset blockquotes with a leading token-coloured bar, rules,
// monospace code blocks on a surface background with tab expansion and
// horizontal overflow scrolling, GFM tables as bordered grids, and images
// through a caller-supplied [ImageProvider]. Code blocks are optionally
// syntax-highlighted through the [Highlighter] hook on [Style]; the
// markdown/highlight subpackage provides a chroma-backed implementation so
// this package never depends on chroma.
//
// This module carries the goldmark dependency so prism does not have to;
// gioui.org/x/markdown was evaluated (2026-07-20) and rejected as a
// dependency — see the README.
package markdown

// Span is one styled inline run within a heading or paragraph. The zero
// value's flags describe plain body text; rendering styles (colour, size,
// typeface) are resolved against a [Style] at layout time.
type Span struct {
	// Text is the run's content. A trailing "\n" is a hard line break.
	Text string
	// Bold marks **strong emphasis**.
	Bold bool
	// Italic marks *emphasis*.
	Italic bool
	// Code marks an `inline code` span, rendered in the monospace typeface.
	Code bool
	// Strikethrough marks GFM ~~deleted~~ text.
	Strikethrough bool
	// URL, when non-empty, marks the run as a hyperlink.
	URL string
}

// Block is one block-level element of a parsed document. The concrete types
// are *[Heading], *[Paragraph], *[List], *[Blockquote], *[CodeBlock],
// *[Rule], *[Table], and *[Image].
type Block interface{ isBlock() }

// Heading is an ATX or setext heading.
type Heading struct {
	// Level is the heading level, 1 through 6.
	Level int
	// Spans is the heading's inline content.
	Spans []Span
}

// Paragraph is a run of wrapped inline text.
type Paragraph struct {
	// Spans is the paragraph's inline content.
	Spans []Span
}

// List is an ordered or unordered list.
type List struct {
	// Ordered distinguishes numbered lists from bulleted ones.
	Ordered bool
	// Start is the first item's number in an ordered list; 0 when unordered.
	Start int
	// Items are the list's items in document order.
	Items []*ListItem
}

// ListItem is one item of a [List]. Nested lists appear as *[List] blocks
// within Blocks.
type ListItem struct {
	// Task marks a GFM task-list item rendered with a checkbox.
	Task bool
	// Checked is the checkbox state of a task item.
	Checked bool
	// Blocks is the item's content.
	Blocks []Block
}

// Blockquote is a quoted group of blocks, rendered as an inset column with a
// leading token-coloured bar.
type Blockquote struct {
	// Blocks is the quoted content; nested quotes appear as *[Blockquote].
	Blocks []Block
}

// CodeBlock is a fenced or indented code block.
type CodeBlock struct {
	// Language is the fence's info string ("" for indented blocks or bare
	// fences).
	Language string
	// Code is the literal content without the trailing newline. Tabs are
	// expanded to 4-column tab stops at parse time.
	Code string
}

// Rule is a thematic break.
type Rule struct{}

// Alignment is a table column's horizontal cell alignment, from the GFM
// delimiter row. The zero value aligns left.
type Alignment int

const (
	// AlignLeft aligns cell content to the left (the GFM default).
	AlignLeft Alignment = iota
	// AlignCenter centres cell content.
	AlignCenter
	// AlignRight aligns cell content to the right.
	AlignRight
)

// Table is a GFM table: an emphasised header row over zero or more body rows,
// rendered as a grid with token-coloured borders.
type Table struct {
	// Alignments is the per-column alignment; its length is the column count.
	Alignments []Alignment
	// Header is the header row. Rows are normalised to the column count:
	// missing cells are empty, extra cells are dropped (per GFM).
	Header []*TableCell
	// Rows are the body rows, each normalised like Header.
	Rows [][]*TableCell
}

// TableCell is one cell of a [Table].
type TableCell struct {
	// Spans is the cell's inline content.
	Spans []Span
}

// Image is a block-level image: a paragraph whose sole inline content is one
// image. Pixels come from the caller's [ImageProvider]; without one (or when
// it fails) the alt text renders as a paragraph instead. Images mixed into
// surrounding text fall back to their alt text at parse time.
type Image struct {
	// URL is the image destination as written in the source. The library
	// never fetches it — resolution is the provider's business.
	URL string
	// Alt is the image's alternate text, the fallback rendering.
	Alt string
}

func (*Heading) isBlock()    {}
func (*Paragraph) isBlock()  {}
func (*List) isBlock()       {}
func (*Blockquote) isBlock() {}
func (*CodeBlock) isBlock()  {}
func (*Rule) isBlock()       {}
func (*Table) isBlock()      {}
func (*Image) isBlock()      {}
