// Package markdown renders markdown documents as Gio widgets built from components
// primitives. It is tier 4 of the Vibrant Gio stack, alongside patterns, and
// nothing in the design system depends on it: it is a leaf an application
// reaches for when it has documents to put on screen.
//
// [Parse] walks a goldmark AST (with extension.GFM) into a block model — a
// tree of [Block] values whose inline content is expressed as styled [Span]
// runs. [Document] lays the top-level blocks through components/list, so long
// documents stay O(visible), and renders each block with components widgets:
// type-scale headings, richtext paragraphs, nested lists with task-list
// checkboxes, inset blockquotes with a leading token-coloured bar, rules,
// monospace code blocks on a surface background with tab expansion and
// horizontal overflow scrolling, GFM tables as bordered grids, and images
// through a caller-supplied [ImageProvider].
//
// Two heavy dependencies are deliberately kept out of this package's import
// graph, each reachable only by importing the subpackage that wants it: code
// blocks are syntax-highlighted through the [Highlighter] hook on [Style],
// which markdown/highlight implements with chroma, and vector images are served
// through [WidgetImageProvider], which markdown/svgimage implements with
// vibrantgio/svg. Only goldmark stops here rather than one level further out,
// so components never sees it; gioui.org/x/markdown was evaluated (2026-07-20) and
// rejected as a dependency — see the README.
//
// # The monospace font comes from the theme
//
// [FromTokens] resolves Style.Mono and Style.CodeSize from the Code role of
// the tokens.Typography it is handed — "Roboto Mono" for the default, a face
// the default collection (tokens.DefaultTypography.Faces) carries in all four
// weight/style combinations code shapes in — regular, bold, italic, and bold
// italic. Out of the box, code blocks and inline code render monospaced.
//
// The caveat survives for custom shapers: Gio matches typeface names
// literally, with no CSS generic-family fallback and no error or warning on a
// miss — a collection without a "Roboto Mono" face silently shapes code in
// the proportional body face instead. An application building its own shaper
// either includes vibrantgio/font/robotomono's faces in the collection or
// points Style.Mono at a monospace family the collection does hold.
package markdown

// Span is one styled inline run within a heading or paragraph. The zero
// value's flags describe plain body text; rendering styles (colour, size,
// typeface) are resolved against a [Style] at layout time.
type Span struct {
	// Text is the run's content, ready to draw: markup is gone and backslash
	// escapes are resolved, so "\_" arrives as "_" — except inside a code
	// span, where a backslash is literal. A trailing "\n" is a hard line
	// break.
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
	// URL is the image destination as written in the source, with its
	// backslash escapes resolved. The library never fetches it — resolution
	// is the provider's business.
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
