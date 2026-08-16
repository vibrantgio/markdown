package obsidian

import (
	"regexp"
	"strings"

	"github.com/vibrantgio/markdown"
)

// idTail matches a trailing block-id tail: a space, a caret, then the id.
var idTail = regexp.MustCompile(` \^([A-Za-z0-9-]+)$`)

// idOnly matches a text that is nothing but a block id — the own-line form
// written below tables, quotes and fences.
var idOnly = regexp.MustCompile(`^\^([A-Za-z0-9-]+)$`)

// BlockAnchors strips block-id tails from blocks and returns the transformed
// blocks alongside a map from each id to the index of the top-level block
// carrying it — an index straight into the returned slice, usable with
// [markdown.NewDocumentAt].
//
// A tail is a trailing " ^id" on a paragraph or a list item, with the id
// spelled from letters, digits and dashes. It is removed from the display
// spans; an id on a list item anchors the item's enclosing top-level list. A
// paragraph that consists of nothing but "^id" — the form written on its own
// line below a table, quote or fence — is dropped entirely and anchors the
// block preceding it. Ids inside code spans and code blocks are never
// recognised.
func BlockAnchors(blocks []markdown.Block) ([]markdown.Block, map[string]int) {
	anchors := make(map[string]int)
	var out []markdown.Block
	for _, b := range blocks {
		if p, isPara := b.(*markdown.Paragraph); isPara {
			if id, own := ownLineID(p); own {
				at := len(out) - 1
				if at < 0 {
					at = 0
				}
				anchors[id] = at
				continue
			}
		}
		idx := len(out)
		stripped, ids := anchorBlock(b)
		out = append(out, stripped)
		for _, id := range ids {
			anchors[id] = idx
		}
	}
	return out, anchors
}

// ownLineID reports whether the paragraph is nothing but a block id.
func ownLineID(p *markdown.Paragraph) (string, bool) {
	if len(p.Spans) != 1 {
		return "", false
	}
	s := p.Spans[0]
	if s.Code || s.URL != "" || s.Bold || s.Italic || s.Strikethrough {
		return "", false
	}
	m := idOnly.FindStringSubmatch(s.Text)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// anchorBlock rebuilds one block with its id tails stripped, returning the
// ids found anywhere inside it.
func anchorBlock(b markdown.Block) (markdown.Block, []string) {
	switch b := b.(type) {
	case *markdown.Paragraph:
		spans, id, found := stripTail(b.Spans)
		if !found {
			return b, nil
		}
		return &markdown.Paragraph{Spans: spans}, []string{id}
	case *markdown.List:
		items := make([]*markdown.ListItem, len(b.Items))
		var ids []string
		for i, it := range b.Items {
			blocks, itemIDs := anchorChildren(it.Blocks)
			items[i] = &markdown.ListItem{Task: it.Task, Checked: it.Checked, Blocks: blocks}
			ids = append(ids, itemIDs...)
		}
		return &markdown.List{Ordered: b.Ordered, Start: b.Start, Items: items}, ids
	case *markdown.Blockquote:
		blocks, ids := anchorChildren(b.Blocks)
		return &markdown.Blockquote{Blocks: blocks}, ids
	default:
		return b, nil
	}
}

// anchorChildren applies anchorBlock across nested blocks, dropping own-line
// id paragraphs the same way the top level does.
func anchorChildren(blocks []markdown.Block) ([]markdown.Block, []string) {
	var out []markdown.Block
	var ids []string
	for _, b := range blocks {
		if p, isPara := b.(*markdown.Paragraph); isPara {
			if id, own := ownLineID(p); own {
				ids = append(ids, id)
				continue
			}
		}
		stripped, found := anchorBlock(b)
		out = append(out, stripped)
		ids = append(ids, found...)
	}
	return out, ids
}

// stripTail removes a trailing id tail from the last eligible span. found is
// false when no tail is present.
func stripTail(spans []markdown.Span) ([]markdown.Span, string, bool) {
	if len(spans) == 0 {
		return spans, "", false
	}
	last := spans[len(spans)-1]
	if last.Code || last.URL != "" {
		return spans, "", false
	}
	m := idTail.FindStringSubmatchIndex(last.Text)
	if m == nil {
		return spans, "", false
	}
	id := last.Text[m[2]:m[3]]
	out := make([]markdown.Span, len(spans))
	copy(out, spans)
	last.Text = strings.TrimRight(last.Text[:m[0]], " ")
	if last.Text == "" {
		out = out[:len(out)-1]
	} else {
		out[len(out)-1] = last
	}
	return out, id, true
}
