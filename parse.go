package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmext "github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	gmtext "github.com/yuin/goldmark/text"
)

// tabStop is the column width tabs expand to in code blocks.
const tabStop = 4

// Parse parses GitHub-flavored markdown into the block model. Markdown has no
// invalid inputs, so every source yields a document; constructs outside the
// supported set (raw HTML, and until G6.3 tables and images) are dropped.
func Parse(source []byte) []Block {
	p := goldmark.New(goldmark.WithExtensions(gmext.GFM)).Parser()
	doc := p.Parse(gmtext.NewReader(source))
	return convertBlocks(source, doc)
}

// convertBlocks maps the block-level children of parent onto the block model.
func convertBlocks(src []byte, parent ast.Node) []Block {
	var out []Block
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Heading:
			out = append(out, &Heading{Level: n.Level, Spans: inlines(src, n)})
		case *ast.Paragraph:
			if spans := inlines(src, n); len(spans) > 0 {
				out = append(out, &Paragraph{Spans: spans})
			}
		case *ast.TextBlock:
			// The paragraph form goldmark uses inside tight list items.
			if spans := inlines(src, n); len(spans) > 0 {
				out = append(out, &Paragraph{Spans: spans})
			}
		case *ast.ThematicBreak:
			out = append(out, &Rule{})
		case *ast.FencedCodeBlock:
			out = append(out, &CodeBlock{
				Language: string(n.Language(src)),
				Code:     blockCode(src, n),
			})
		case *ast.CodeBlock:
			out = append(out, &CodeBlock{Code: blockCode(src, n)})
		case *ast.Blockquote:
			out = append(out, &Blockquote{Blocks: convertBlocks(src, n)})
		case *ast.List:
			out = append(out, convertList(src, n))
		}
	}
	return out
}

// convertList maps a goldmark list and its items, recording ordering, the
// ordered start number, and GFM task-list checkbox state.
func convertList(src []byte, n *ast.List) *List {
	l := &List{Ordered: n.IsOrdered(), Start: n.Start}
	if !l.Ordered {
		l.Start = 0
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		li, ok := c.(*ast.ListItem)
		if !ok {
			continue
		}
		item := &ListItem{}
		// A GFM task item carries a TaskCheckBox as the first inline node of
		// its first paragraph.
		if fc := li.FirstChild(); fc != nil {
			switch fc.(type) {
			case *ast.TextBlock, *ast.Paragraph:
				if cb, ok := fc.FirstChild().(*east.TaskCheckBox); ok {
					item.Task = true
					item.Checked = cb.IsChecked
				}
			}
		}
		item.Blocks = convertBlocks(src, li)
		if item.Task {
			trimTaskSpace(item.Blocks)
		}
		l.Items = append(l.Items, item)
	}
	return l
}

// trimTaskSpace drops the space the "[x] " syntax leaves in front of a task
// item's first text run.
func trimTaskSpace(blocks []Block) {
	if len(blocks) == 0 {
		return
	}
	if p, ok := blocks[0].(*Paragraph); ok && len(p.Spans) > 0 {
		p.Spans[0].Text = strings.TrimPrefix(p.Spans[0].Text, " ")
	}
}

// blockCode joins a code block's source lines, expands tabs, and strips the
// trailing newline.
func blockCode(src []byte, n ast.Node) string {
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(src))
	}
	return expandTabs(strings.TrimSuffix(b.String(), "\n"))
}

// expandTabs replaces tabs with spaces up to the next tabStop-column stop,
// per line.
func expandTabs(s string) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		switch r {
		case '\t':
			n := tabStop - col%tabStop
			for range n {
				b.WriteByte(' ')
			}
			col += n
		case '\n':
			b.WriteRune(r)
			col = 0
		default:
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// inlines maps a block node's inline children onto styled spans, inheriting
// emphasis, code, strikethrough, and link state down the inline tree.
// Adjacent runs with identical styling are merged.
func inlines(src []byte, parent ast.Node) []Span {
	var out []Span
	add := func(s Span) {
		if s.Text == "" {
			return
		}
		if n := len(out); n > 0 && sameStyle(out[n-1], s) {
			out[n-1].Text += s.Text
			return
		}
		out = append(out, s)
	}
	var walk func(n ast.Node, s Span)
	walk = func(n ast.Node, s Span) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch c := c.(type) {
			case *ast.Text:
				sp := s
				sp.Text = string(c.Segment.Value(src))
				switch {
				case c.HardLineBreak():
					sp.Text += "\n"
				case c.SoftLineBreak():
					sp.Text += " "
				}
				add(sp)
			case *ast.String:
				sp := s
				sp.Text = string(c.Value)
				add(sp)
			case *ast.CodeSpan:
				sp := s
				sp.Code = true
				walk(c, sp)
			case *ast.Emphasis:
				sp := s
				if c.Level >= 2 {
					sp.Bold = true
				} else {
					sp.Italic = true
				}
				walk(c, sp)
			case *east.Strikethrough:
				sp := s
				sp.Strikethrough = true
				walk(c, sp)
			case *ast.Link:
				sp := s
				sp.URL = string(c.Destination)
				walk(c, sp)
			case *ast.AutoLink:
				sp := s
				sp.URL = string(c.URL(src))
				sp.Text = string(c.Label(src))
				add(sp)
			case *east.TaskCheckBox:
				// Rendered as a real checkbox by the list-item widget.
			case *ast.Image, *ast.RawHTML:
				// Dropped until G6.3 (images) / by design (raw HTML).
			default:
				walk(c, s)
			}
		}
	}
	walk(parent, Span{})
	return out
}

// sameStyle reports whether two spans carry identical styling, making their
// text runs mergeable.
func sameStyle(a, b Span) bool {
	return a.Bold == b.Bold && a.Italic == b.Italic && a.Code == b.Code &&
		a.Strikethrough == b.Strikethrough && a.URL == b.URL
}
