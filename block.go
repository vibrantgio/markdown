// Package markdown renders markdown documents with prism primitives
// (DESIGN §Markdown).
//
// [Parse] walks a goldmark AST (with extension.GFM) into a block model — a
// tree of [Block] values whose inline content is expressed as styled [Span]
// runs. [Document] lays the top-level blocks through prism/list, so long
// documents stay O(visible), and renders each block with prism widgets:
// type-scale headings, richtext paragraphs, nested lists with task-list
// checkboxes, inset blockquotes with a leading token-coloured bar, rules, and
// monospace code blocks on a surface background with tab expansion and
// horizontal overflow scrolling.
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
// are *[Heading], *[Paragraph], *[List], *[Blockquote], *[CodeBlock], and
// *[Rule].
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

func (*Heading) isBlock()    {}
func (*Paragraph) isBlock()  {}
func (*List) isBlock()       {}
func (*Blockquote) isBlock() {}
func (*CodeBlock) isBlock()  {}
func (*Rule) isBlock()       {}
