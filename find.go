package markdown

import (
	"image"
	"image/color"
	"unicode"

	"github.com/vibrantgio/components/paragraph"
)

// Find sets the query the document marks its matches of and says which match
// is the current one. The query is plain text, matched case-insensitively;
// nothing in it is a pattern. Setting the same query again only moves the
// current index, so a caller may call it every frame.
//
// Every match is marked with [Style].MatchFill and the current one with
// [Style].CurrentMatchFill, painted behind the matched glyphs on the line's
// box and under the text. The current index is the caller's to step: it
// indexes [Document.Matches], and an index outside them leaves every match
// marked alike.
//
// An empty query marks nothing; see [Document.ClearFind].
func (d *Document) Find(query string, current int) {
	if query != d.find.query {
		d.find.query = query
		d.find.matches, d.find.marks = index(d.blocks, query)
	}
	d.find.current = current
}

// ClearFind drops the query, and every mark on the document with it.
func (d *Document) ClearFind() { d.Find("", 0) }

// Matches returns the document's matches in reading order: one entry per
// occurrence of the query, naming the top-level block it lies in and where
// the document last painted it.
//
// The slice is the caller's; the document keeps its own.
func (d *Document) Matches() []Match {
	out := make([]Match, len(d.find.matches))
	copy(out, d.find.matches)
	if d.find.rowbased {
		tops := d.rowTops()
		for b, span := range d.find.rows {
			top, ok := tops[b]
			if !ok {
				continue
			}
			collect(out, d.find.found[span[0]:span[1]], image.Pt(0, top))
		}
		return out
	}
	collect(out, d.find.found, image.Point{})
	return out
}

// MatchPlaces returns where each match lies in the document, in reading order
// and as a fraction of the content's height in [0,1] — one entry per entry of
// [Document.Matches], which is what [scrollbar.Style].Matches takes.
//
// The places are an estimate, and the layout cannot give a better one: a list
// lays out the rows in the viewport and no others, so the only block heights
// the document has are those of blocks it has drawn. A block it has measured
// counts its own height and one it has not counts the mean of the measured
// ones, which leaves a document that has not laid out at all placing its
// matches by block index over block count. Every match inside one block
// reports that block's place.
func (d *Document) MatchPlaces() []float32 {
	n := len(d.blocks)
	if n == 0 || len(d.find.matches) == 0 {
		return nil
	}
	mean, measured := 0, 0
	for _, h := range d.find.heights {
		mean += h
		measured++
	}
	if measured > 0 {
		mean = max(mean/measured, 1)
	} else {
		mean = 1
	}
	tops := make([]int, n)
	y := 0
	for i, b := range d.blocks {
		tops[i] = y
		h, ok := d.find.heights[b]
		if !ok || h <= 0 {
			h = mean
		}
		y += h
	}
	out := make([]float32, len(d.find.matches))
	if y <= 0 {
		return out
	}
	for k, m := range d.find.matches {
		i := min(max(m.Block, 0), n-1)
		out[k] = float32(tops[i]) / float32(y)
	}
	return out
}

// Match is one occurrence of the find query.
type Match struct {
	// Block indexes [Document.Blocks]: the top-level block the match lies
	// in, and the index [Document.ScrollToBlock] seats.
	Block int
	// Rect is where the document last painted the match, in the plane it was
	// laid out into — the viewport for [Document.Layout], the whole column
	// for [Document.LayoutColumn]. A match wrapped across lines reports the
	// box around all of it.
	//
	// It is empty for a match the last layout did not reach, which is every
	// match before the first frame and every match outside the viewport of a
	// scrolling document: the position is a result of wrapping and of where
	// the reader is, and neither is known until the document lays out. Block
	// is what takes the reader there whether or not there is a rectangle
	// yet, and the rectangle then says where on the screen it landed.
	Rect image.Rectangle
}

// findState is the document's find: the query, the marks it puts on the
// blocks, and where the last layout painted them.
type findState struct {
	query   string
	current int
	matches []Match
	// marks are the byte ranges to fill, keyed by the heading, paragraph,
	// table cell or code block whose text they are counted in — the same
	// keys the per-paragraph state is held under.
	marks map[any][]spanMark
	// found is every rectangle the last layout painted a mark at, in the
	// coordinates of whatever the document was drawing into at the time;
	// rows and heights carry them the rest of the way. See [Document.shift].
	found []foundRect
	// rows records, per top-level block, the range of found entries its row
	// painted, and heights the row height a layout gave it. Both are
	// filled only by the list paths, which is what rowbased reports: a
	// column lays its blocks out itself and its entries need no carrying.
	//
	// rows is one frame's; heights keeps every height ever measured, because
	// a list measures only the rows it draws and [Document.MatchPlaces] has
	// the whole document to place. A height held from an earlier frame is
	// replaced the next time its row is drawn.
	rows     map[Block][2]int
	heights  map[Block]int
	rowbased bool
	// seek is the match [Document.ScrollToMatch] was asked for and the next
	// layout carries out, or -1 when there is nothing to carry out; seated
	// records that the match's block has already been put at the top of the
	// viewport, which bounds the move to the two frames it can need.
	seek   int
	seated bool
}

// spanMark is one match inside one block's text: the byte range to fill and
// the match it belongs to.
type spanMark struct {
	start, end int
	match      int
}

// foundRect is one rectangle a mark was painted at.
type foundRect struct {
	match int
	r     image.Rectangle
}

// collect folds a run of painted rectangles, offset by off, into the matches
// they belong to: a match painted more than once — wrapped across lines, or
// split by a change of style mid-word — takes the box around all of it.
func collect(out []Match, found []foundRect, off image.Point) {
	for _, f := range found {
		if f.match < 0 || f.match >= len(out) {
			continue
		}
		r := f.r.Add(off)
		if out[f.match].Rect.Empty() {
			out[f.match].Rect = r
			continue
		}
		out[f.match].Rect = out[f.match].Rect.Union(r)
	}
}

// rowTops is where the last layout put each row it laid out, measured from
// the viewport's top edge.
//
// The list draws its rows from the resolved position's First downwards,
// seating that first row one Offset above the viewport's top edge, so walking
// the row heights the frame recorded from there places every row it drew —
// and the one row above First a list draws when it has one.
func (d *Document) rowTops() map[Block]int {
	p := d.list.Position()
	tops := make(map[Block]int, len(d.find.heights))
	y := -p.Offset
	for i := p.First; i >= 0 && i < len(d.blocks); i++ {
		b := d.blocks[i]
		h, ok := d.find.heights[b]
		if !ok {
			break
		}
		tops[b] = y
		y += h
	}
	if i := p.First - 1; i >= 0 && i < len(d.blocks) {
		b := d.blocks[i]
		if h, ok := d.find.heights[b]; ok {
			tops[b] = -p.Offset - h
		}
	}
	return tops
}

// active reports whether anything is marked, which is what the layout paths
// spend their bookkeeping on.
func (f *findState) active() bool { return len(f.marks) > 0 }

// index walks the blocks in reading order and records every match of query,
// each a byte range inside one block's text. Blocks that hold no text — a
// rule, an image — hold no match.
func index(blocks []Block, query string) ([]Match, map[any][]spanMark) {
	if query == "" {
		return nil, nil
	}
	ix := indexer{query: query, marks: make(map[any][]spanMark)}
	for i, b := range blocks {
		ix.block = i
		ix.walk(b)
	}
	if len(ix.matches) == 0 {
		return nil, nil
	}
	return ix.matches, ix.marks
}

// indexer accumulates the matches of one query across a block tree.
type indexer struct {
	query   string
	block   int
	matches []Match
	marks   map[any][]spanMark
}

// walk indexes one block and everything inside it, in reading order.
func (ix *indexer) walk(b Block) {
	switch b := b.(type) {
	case *Heading:
		ix.text(b, spanText(b.Spans))
	case *Paragraph:
		ix.text(b, spanText(b.Spans))
	case *CodeBlock:
		ix.text(b, b.Code)
	case *List:
		for _, item := range b.Items {
			for _, in := range item.Blocks {
				ix.walk(in)
			}
		}
	case *Blockquote:
		for _, in := range b.Blocks {
			ix.walk(in)
		}
	case *Table:
		for _, c := range b.Header {
			ix.text(c, spanText(c.Spans))
		}
		for _, row := range b.Rows {
			for _, c := range row {
				ix.text(c, spanText(c.Spans))
			}
		}
	}
}

// text records the query's occurrences in one owner's text.
func (ix *indexer) text(owner any, s string) {
	for _, r := range occurrences(s, ix.query) {
		ix.marks[owner] = append(ix.marks[owner], spanMark{start: r[0], end: r[1], match: len(ix.matches)})
		ix.matches = append(ix.matches, Match{Block: ix.block})
	}
}

// spanText is the text a run of spans reads as: their contents in order,
// which is what a byte range into a block's text is counted in and what the
// spans laid out for it concatenate back to.
func spanText(spans []Span) string {
	if len(spans) == 1 {
		return spans[0].Text
	}
	n := 0
	for _, sp := range spans {
		n += len(sp.Text)
	}
	b := make([]byte, 0, n)
	for _, sp := range spans {
		b = append(b, sp.Text...)
	}
	return string(b)
}

// occurrences returns the byte ranges of q in s, case-insensitively and
// without overlapping.
//
// The folding is per rune against the original bytes rather than over a
// lowercased copy of the text: lowercasing can change a rune's byte length,
// and a range counted in the copy would then land somewhere else in the text
// the document draws.
func occurrences(s, q string) [][2]int {
	if q == "" || s == "" {
		return nil
	}
	qr := fold(q)
	sr, at := foldAt(s)
	if len(qr) == 0 || len(sr) < len(qr) {
		return nil
	}
	var out [][2]int
	for i := 0; i+len(qr) <= len(sr); {
		if !equalRunes(sr[i:i+len(qr)], qr) {
			i++
			continue
		}
		out = append(out, [2]int{at[i], at[i+len(qr)]})
		i += len(qr)
	}
	return out
}

// fold returns s as case-folded runes.
func fold(s string) []rune {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		out = append(out, unicode.ToLower(r))
	}
	return out
}

// foldAt returns s as case-folded runes together with the byte offset each
// rune starts at, one past the last being the length of s.
func foldAt(s string) ([]rune, []int) {
	runes := make([]rune, 0, len(s))
	at := make([]int, 0, len(s)+1)
	for i, r := range s {
		runes = append(runes, unicode.ToLower(r))
		at = append(at, i)
	}
	return runes, append(at, len(s))
}

// equalRunes reports whether two rune runs of the same length are equal.
func equalRunes(a, b []rune) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fills puts the document's marks for owner on the spans that render it and
// returns the spans together with the match each fill belongs to, indexed the
// way [paragraph.Style].OnFill reports them: by span, then by fill.
//
// The spans laid out for a block concatenate back to the text the marks were
// counted in — that is the contract [spanText] and [Highlighter] both keep —
// so a range is placed by walking the spans and subtracting what came before.
func (d *Document) fills(style Style, owner any, spans []paragraph.SpanStyle) ([]paragraph.SpanStyle, [][]int) {
	marks := d.find.marks[owner]
	if len(marks) == 0 {
		return spans, nil
	}
	key := make([][]int, len(spans))
	base, next := 0, 0
	for i := range spans {
		n := len(spans[i].Content)
		for j := next; j < len(marks) && marks[j].start < base+n; j++ {
			m := marks[j]
			if m.end <= base {
				continue
			}
			spans[i].Fills = append(spans[i].Fills, paragraph.Fill{
				Start: max(m.start-base, 0),
				End:   min(m.end-base, n),
				Color: d.matchFill(style, m.match),
			})
			key[i] = append(key[i], m.match)
		}
		for next < len(marks) && marks[next].end <= base+n {
			next++
		}
		base += n
	}
	return spans, key
}

// matchFill is the fill one match is marked with: the current match's own,
// and the plain one for every other.
func (d *Document) matchFill(style Style, match int) color.NRGBA {
	if match == d.find.current {
		return style.CurrentMatchFill
	}
	return style.MatchFill
}

// onFill returns the hook that records where the fills land, keyed by the
// map [Document.fills] returned. A block with neither a mark nor a word on
// it gets no hook, so a document with no query and no key held pays nothing
// for this.
//
// The key carries both kinds: a find match counts up from zero and a heading
// word down from minus one, so one hook serves the marks the reader asked for
// and the colourless fills that say where the words under the pointer are.
func (d *Document) onFill(key [][]int) func(span, fill int, r image.Rectangle) {
	if key == nil {
		return nil
	}
	return func(span, fill int, r image.Rectangle) {
		if span < 0 || span >= len(key) || fill < 0 || fill >= len(key[span]) {
			return
		}
		if m := key[span][fill]; m < 0 {
			d.word.placed = append(d.word.placed, wordRect{cand: placeKey(m), r: r})
			return
		}
		d.find.found = append(d.find.found, foundRect{match: key[span][fill], r: r})
	}
}

// filled returns the paragraph style a block is laid out with when the
// document has marked matches in it: the style given, carrying the hook that
// records where the fills landed.
func (d *Document) filled(ps paragraph.Style, key [][]int) paragraph.Style {
	ps.OnFill = d.onFill(key)
	return ps
}

// shift moves every rectangle recorded since from by off. It is how a
// rectangle reported in a paragraph's own coordinates is carried out to the
// plane the document was laid out into: each layout path that offsets what it
// has drawn offsets the marks inside it by the same amount.
func (d *Document) shift(from int, off image.Point) {
	d.shiftRange(from, len(d.find.found), off)
}

// shiftRange moves the rectangles recorded in [from, to) by off.
func (d *Document) shiftRange(from, to int, off image.Point) {
	if off == (image.Point{}) {
		return
	}
	for i := from; i < to && i < len(d.find.found); i++ {
		d.find.found[i].r = d.find.found[i].r.Add(off)
	}
}

// row records what one top-level block's row painted: the range of marks it
// holds and the height the list stacks it by.
func (d *Document) recordRow(b Block, from int, height int) {
	d.find.rows[b] = [2]int{from, len(d.find.found)}
	d.find.heights[b] = height
}

// findFrame starts a frame's record of where the marks land. rows reports
// that the blocks are laid out as list rows, whose places the list decides
// and [findState.origin] reconstructs.
func (d *Document) findFrame(rows bool) {
	d.find.found = d.find.found[:0]
	d.find.rowbased = rows
	if !rows {
		return
	}
	clear(d.find.rows)
}
