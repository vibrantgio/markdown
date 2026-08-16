// Package obsidian recognises the Obsidian flavour of markdown on top of the
// parent package's public block model, without touching the parser: YAML-style
// frontmatter is split off before [github.com/vibrantgio/markdown.Parse],
// wikilink and embed syntax is lifted into hyperlink spans after it, and
// trailing block-id tails are stripped from display and exposed as anchors.
//
// The package does recognition only — no vault semantics. [SplitFrontMatter]
// cuts the leading properties block and hands it back as data;
// [WikiSpans] turns [[target]], [[target|alias]] and ![[target]] occurrences
// into [github.com/vibrantgio/markdown.Span] runs whose URL carries the raw
// link body under the "wiki:" scheme ("wikiembed:" for embeds), leaving
// resolution entirely to the caller; [BlockAnchors] strips trailing " ^id"
// tails from paragraphs and list items and maps each id to its top-level
// block index, ready for
// [github.com/vibrantgio/markdown.NewDocumentAt].
//
// One honest limitation, pinned by a test: a [github.com/vibrantgio/markdown.Span]
// is a styling run, so a wikilink whose body crosses a styling boundary
// ("[[a *b*]]") is not recognised. Obsidian targets do not carry markdown
// styling, so this excludes approximately nothing in practice.
package obsidian

import "strings"

// FrontMatter is the leading properties block of a note, as split off by
// [SplitFrontMatter].
type FrontMatter struct {
	// Present reports whether the source began with a frontmatter block at
	// all; it distinguishes an empty block from no block.
	Present bool
	// Raw is the block's inner text, exactly as written, without the
	// delimiter lines. Everything the trivial field split cannot read is
	// still here.
	Raw string
	// Fields are the pairs a trivial line split yields: "key: scalar" lines
	// and "key:" lines followed by "- item" block lists. Anything more
	// structured (nested maps, multi-line strings) is deliberately not
	// parsed — no YAML machinery is involved — and remains in Raw only.
	Fields []Field
}

// Field is one frontmatter pair the trivial split could read.
type Field struct {
	// Key is the property name, the text before the first colon.
	Key string
	// Value is the scalar value, whitespace-trimmed, exactly as written
	// otherwise (quotes and brackets are not interpreted). Empty for a list
	// field and for an empty property.
	Value string
	// Items holds the "- item" entries of a block list, nil for a scalar.
	Items []string
}

// SplitFrontMatter cuts a leading frontmatter block off src and returns it
// alongside the remaining body. A block starts with a first line that is
// exactly "---" and ends at the next line that is exactly "---" or "...";
// the body is everything after the terminator line. When src does not start
// with "---", or the block is never terminated, the frontmatter is absent
// and the body is src, byte-identical. A "---" later in the document is a
// thematic break, not frontmatter, and is left alone.
func SplitFrontMatter(src []byte) (FrontMatter, []byte) {
	text := string(src)
	first, rest, hasMore := cutLine(text)
	if trimCR(first) != "---" || !hasMore {
		return FrontMatter{}, src
	}
	inner := rest
	for pos := 0; pos <= len(rest); {
		line, after, more := cutLine(rest[pos:])
		t := trimCR(line)
		if t == "---" || t == "..." {
			raw := inner[:pos]
			body := ""
			if more {
				body = after
			}
			return FrontMatter{
				Present: true,
				Raw:     raw,
				Fields:  parseFields(raw),
			}, []byte(body)
		}
		if !more {
			break
		}
		pos += len(line) + 1
	}
	return FrontMatter{}, src
}

// cutLine splits text at its first newline. hasMore reports whether a
// newline was found; after is the text following it.
func cutLine(text string) (line, after string, hasMore bool) {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i], text[i+1:], true
	}
	return text, "", false
}

// trimCR drops a trailing carriage return so CRLF sources delimit like LF
// ones.
func trimCR(line string) string {
	return strings.TrimSuffix(line, "\r")
}

// parseFields runs the trivial line split over the block's inner text.
func parseFields(raw string) []Field {
	lines := strings.Split(raw, "\n")
	var out []Field
	for i := 0; i < len(lines); i++ {
		line := trimCR(lines[i])
		if strings.TrimSpace(line) == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			continue // indented lines belong to structure the split does not read
		}
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		value := strings.TrimSpace(line[colon+1:])
		if value != "" {
			out = append(out, Field{Key: key, Value: value})
			continue
		}
		// Empty value: a block list, an empty property, or the head of
		// structure the split does not read.
		items, next, ok := collectItems(lines, i+1)
		if !ok {
			i = next - 1
			continue // nested structure: the key stays raw only
		}
		out = append(out, Field{Key: key, Items: items})
		i = next - 1
	}
	return out
}

// collectItems gathers the "- item" lines following an empty-value key.
// next is the index of the first line not consumed. ok is false when the
// key is followed by indented lines that are not list items — structure the
// trivial split refuses to guess at.
func collectItems(lines []string, start int) (items []string, next int, ok bool) {
	i := start
	for ; i < len(lines); i++ {
		line := trimCR(lines[i])
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(trimmed, "- ") && trimmed != "-" {
			break // next top-level line
		}
		if trimmed == "-" {
			items = append(items, "")
			continue
		}
		rest, isItem := strings.CutPrefix(trimmed, "- ")
		if !isItem {
			return nil, i, false
		}
		items = append(items, strings.TrimSpace(rest))
	}
	return items, i, true
}
