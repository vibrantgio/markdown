package obsidian

import (
	"regexp"
	"strings"

	"github.com/vibrantgio/markdown"
)

// wikiLink matches one wikilink occurrence: an optional embed bang, then a
// double-bracketed body free of brackets and newlines.
var wikiLink = regexp.MustCompile(`(!?)\[\[([^\[\]\n]+)\]\]`)

// WikiSpans returns blocks with every wikilink occurrence lifted into its own
// hyperlink span. It walks headings, paragraphs, list items, blockquotes and
// table cells; code blocks pass through untouched, and spans already marked
// Code or already carrying a URL are never split — code is not a link edge,
// and existing links keep their destination.
//
// A recognised occurrence becomes a [markdown.Span] whose Text is the alias
// when one is written ("[[target|alias]]") and the body as written otherwise,
// and whose URL is "wiki:" plus the raw body — "wikiembed:" plus the raw body
// for the "![[target]]" embed form. The body is carried verbatim, alias
// included; interpreting it is the caller's business. Surrounding text keeps
// its styling, and the link span inherits it.
//
// Limitations, by construction. A span is a styling run, so a wikilink whose
// body crosses a styling boundary ("[[a *b*]]" arrives as three spans) is
// not recognised. And a span is finished text: [markdown.Parse] has already
// resolved backslash escapes, so brackets written "\[\[Note\]\]" to keep a
// wikilink from forming reach this function as the plain characters
// "[[Note]]" and are recognised anyway. Escapes inside a body do reach their
// target correctly — "[[q5\_0]]" links to "q5_0", the name on disk.
func WikiSpans(blocks []markdown.Block) []markdown.Block {
	out := make([]markdown.Block, len(blocks))
	for i, b := range blocks {
		out[i] = wikiBlock(b)
	}
	return out
}

// wikiBlock rebuilds one block with its spans transformed, sharing blocks
// that carry no spans.
func wikiBlock(b markdown.Block) markdown.Block {
	switch b := b.(type) {
	case *markdown.Heading:
		return &markdown.Heading{Level: b.Level, Spans: wikiSpans(b.Spans)}
	case *markdown.Paragraph:
		return &markdown.Paragraph{Spans: wikiSpans(b.Spans)}
	case *markdown.List:
		items := make([]*markdown.ListItem, len(b.Items))
		for i, it := range b.Items {
			items[i] = &markdown.ListItem{
				Task:    it.Task,
				Checked: it.Checked,
				Blocks:  WikiSpans(it.Blocks),
			}
		}
		return &markdown.List{Ordered: b.Ordered, Start: b.Start, Items: items}
	case *markdown.Blockquote:
		return &markdown.Blockquote{Blocks: WikiSpans(b.Blocks)}
	case *markdown.Table:
		return &markdown.Table{
			Alignments: b.Alignments,
			Header:     wikiCells(b.Header),
			Rows:       wikiRows(b.Rows),
		}
	default:
		return b // CodeBlock, Rule, Image: no spans to transform.
	}
}

func wikiRows(rows [][]*markdown.TableCell) [][]*markdown.TableCell {
	out := make([][]*markdown.TableCell, len(rows))
	for i, row := range rows {
		out[i] = wikiCells(row)
	}
	return out
}

func wikiCells(cells []*markdown.TableCell) []*markdown.TableCell {
	out := make([]*markdown.TableCell, len(cells))
	for i, c := range cells {
		out[i] = &markdown.TableCell{Spans: wikiSpans(c.Spans)}
	}
	return out
}

// wikiSpans splits every eligible span on the wikilink grammar.
func wikiSpans(spans []markdown.Span) []markdown.Span {
	var out []markdown.Span
	for _, s := range spans {
		if s.Code || s.URL != "" {
			out = append(out, s)
			continue
		}
		out = appendSplit(out, s)
	}
	return out
}

// appendSplit appends s to out, cut at every recognised wikilink.
func appendSplit(out []markdown.Span, s markdown.Span) []markdown.Span {
	text := s.Text
	pos := 0
	for _, m := range wikiLink.FindAllStringSubmatchIndex(text, -1) {
		body := text[m[4]:m[5]]
		display, ok := wikiDisplay(body)
		if !ok {
			continue
		}
		scheme := "wiki:"
		if m[3] > m[2] { // the embed bang matched
			scheme = "wikiembed:"
		}
		if m[0] > pos {
			plain := s
			plain.Text = text[pos:m[0]]
			out = append(out, plain)
		}
		link := s
		link.Text = display
		link.URL = scheme + body
		out = append(out, link)
		pos = m[1]
	}
	if pos == 0 {
		return append(out, s)
	}
	if pos < len(text) {
		tail := s
		tail.Text = text[pos:]
		out = append(out, tail)
	}
	return out
}

// wikiDisplay derives the display text from a link body: the alias when one
// follows the first pipe, the body as written otherwise. A body whose target
// part is blank is not a link.
func wikiDisplay(body string) (string, bool) {
	target, alias, hasAlias := strings.Cut(body, "|")
	if strings.TrimSpace(target) == "" {
		return "", false
	}
	if hasAlias && alias != "" {
		return alias, true
	}
	if hasAlias {
		return target, true
	}
	return body, true
}
