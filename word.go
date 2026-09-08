package markdown

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"strings"
	"time"
	"unicode"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"

	"github.com/vibrantgio/components/paragraph"
	"github.com/vibrantgio/theme/tokens"
)

// The heading word: a word of the prose that a heading of the same document
// equals or contains, whole word and case-insensitively. It is a link only
// while the command key is held with the pointer over it; at rest it is
// prose, and operating it goes to that heading. See the package
// documentation.

// shortcutName is the key the platform maps the command key to: ⌘ on macOS,
// Ctrl on Windows and Linux, the key [key.ModShortcut] names. A modifier held
// alone arrives as a key event carrying no modifiers of its own, so the press
// and the release are read by name and not by modifier.
var shortcutName = shortcutKey()

func shortcutKey() key.Name {
	if runtime.GOOS == "darwin" {
		return key.NameCommand
	}
	return key.NameCtrl
}

// The arrival marking's life: it holds, then fades, and is never cut off. The
// numbers are the ones this system gives a mark that leaves by itself — a
// toast's lifetime, faded over the motion scale's slow stop.
const arrivalLife = 4 * time.Second

var arrivalFade = tokens.Motion.DurSlow

// wordRange is one candidate word of a block's prose: the byte range it
// occupies in the block's text, and the top-level index of the heading it
// names.
type wordRange struct {
	start, end int
	heading    int
}

// wordHit is the heading word under the pointer: the block it lies in and the
// word itself. A nil owner is no word at all.
type wordHit struct {
	owner any
	wordRange
}

// wordRect is where one candidate word landed, in its block's own
// coordinates: the candidate's index and the rectangle the layout reported.
type wordRect struct {
	cand int
	r    image.Rectangle
}

// wordState is the document's heading-word state: whether the command key is
// held, where the pointer is and in which block, the word the last layout
// resolved under it, and the marking a followed heading word left behind.
//
// The lookup is bounded twice over: nothing is looked up while the key is up,
// and while it is held only the one block under the pointer is looked in.
type wordState struct {
	held  bool
	owner any
	at    image.Point
	hit   wordHit
	// placed is where this frame's layout put the candidate words of the
	// block under the pointer; see [Document.wordPlaces].
	placed []wordRect
	// headings maps a lower-cased word to the first heading in reading order
	// that equals or contains it, and cands holds the candidate words of each
	// block already asked for. Both are built once: a document's block tree
	// does not change under it.
	headings map[string]int
	cands    map[any][]wordRange
	// block is the heading a followed heading word took the reader to and
	// armed the instant it was followed; -1 marks nothing. alpha is how much
	// of the marking this frame draws.
	block int
	armed time.Time
	alpha float64
}

// wordFrame prepares one frame's heading-word state: the command key's press
// and release, and what is left of the arrival marking. Every layout path
// calls it before it draws.
func (d *Document) wordFrame(gtx layout.Context) {
	if len(d.headingWords()) > 0 {
		for {
			e, ok := gtx.Event(key.Filter{
				Name:     shortcutName,
				Optional: key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt | key.ModSuper,
			})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok {
				d.hold(gtx, ke.State == key.Press)
			}
		}
	}
	d.arrivalFrame(gtx)
}

// hold takes the command key down or up, dropping the word under the pointer
// with the key and asking for the frame that restores the prose.
func (d *Document) hold(gtx layout.Context, held bool) {
	if held == d.word.held {
		return
	}
	d.word.held = held
	if !held {
		d.word.hit = wordHit{}
	}
	gtx.Execute(op.InvalidateCmd{})
}

// arrivalFrame is what is left of the arrival marking this frame: all of the
// fill until the fade opens, linearly less across the fade, and none of it
// once the life has run. Holding costs one frame at the instant the fade
// opens; fading costs every frame until it is done.
func (d *Document) arrivalFrame(gtx layout.Context) {
	d.word.alpha = 0
	if d.word.block < 0 {
		return
	}
	age := gtx.Now.Sub(d.word.armed)
	hold := arrivalLife - arrivalFade
	switch {
	case age >= arrivalLife:
		d.word.block = -1
	case arrivalFade <= 0 || age <= hold:
		d.word.alpha = 1
		gtx.Execute(op.InvalidateCmd{At: d.word.armed.Add(hold)})
	default:
		d.word.alpha = float64(arrivalLife-age) / float64(arrivalFade)
		gtx.Execute(op.InvalidateCmd{})
	}
}

// arrived returns the block the arrival marking is on, nil when nothing is
// marked. It resolves to a block value for the reason [Document.marked] does:
// the layout paths see one block at a time and know it by identity.
func (d *Document) arrived() Block {
	if d.word.alpha <= 0 || d.word.block < 0 || d.word.block >= len(d.blocks) {
		return nil
	}
	return d.blocks[d.word.block]
}

// arrivalFill is the fill the arrival marking draws with this frame: the
// style's own, at what the fade has left of its alpha.
func (d *Document) arrivalFill(style Style) color.NRGBA {
	fill := style.ArrivalFill
	fill.A = uint8(math.Round(float64(fill.A) * d.word.alpha))
	return fill
}

// prose lays out one text block — a heading or a paragraph — with the
// heading-word machinery around it: the pointer events that say where the
// pointer is, the colourless fills that say where the block's candidate words
// landed, and the link the resolved word is drawn as.
//
// The events are drained before the spans are built, so the word this frame
// paints is the word this frame's pointer is on, and the places are reported
// during the layout, so the word is resolved against the wrapping the frame
// itself drew.
func (d *Document) prose(gtx layout.Context, shaper *text.Shaper, style Style, ps paragraph.Style, owner any, model []Span, spans []paragraph.SpanStyle) layout.Dimensions {
	d.wordPointer(gtx, owner)
	spans = d.wordLink(owner, spans)
	spans, key := d.fills(style, owner, spans)
	spans, key = d.wordPlaces(owner, model, spans, key)
	ps = d.wordClick(d.filled(ps, key), owner)
	if d.looking(owner) {
		d.word.placed = d.word.placed[:0]
	}
	dims := paragraph.Layout(gtx, d.textState(owner), shaper, ps, spans)
	d.wordResolve(gtx, owner)
	d.wordArea(gtx, owner, dims)
	d.wordPointer(gtx, owner)
	return dims
}

// looking reports whether owner is the block whose candidate words this frame
// looks for: the one under the pointer, and only while the key is held.
func (d *Document) looking(owner any) bool {
	return d.word.held && d.word.owner == owner
}

// wordPointer drains one text block's pointer events: where the pointer is
// inside it, and whether the command key is held — which the pointer carries
// as a modifier, and which is what answers on a platform that reports no key
// event for the modifier alone.
func (d *Document) wordPointer(gtx layout.Context, owner any) {
	if len(d.headingWords()) == 0 {
		return
	}
	for {
		e, ok := gtx.Event(pointer.Filter{
			Target: owner,
			Kinds:  pointer.Enter | pointer.Move | pointer.Leave | pointer.Cancel,
		})
		if !ok {
			break
		}
		pe, ok := e.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Leave, pointer.Cancel:
			if d.word.owner != owner {
				break
			}
			d.word.owner = nil
			if d.word.hit.owner == owner {
				d.word.hit = wordHit{}
				gtx.Execute(op.InvalidateCmd{})
			}
		default:
			d.word.owner = owner
			d.word.at = image.Pt(int(pe.Position.X), int(pe.Position.Y))
			d.hold(gtx, pe.Modifiers.Contain(key.ModShortcut))
		}
	}
}

// wordArea registers the block's own pointer area, the whole of what it laid
// out. It is pass-through: the links and the task boxes inside the block
// still take the events they registered for, and it carries no cursor of its
// own, so the pointing hand a link asks for is still the pointer's.
func (d *Document) wordArea(gtx layout.Context, owner any, dims layout.Dimensions) {
	if len(d.headingWords()) == 0 {
		return
	}
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, owner)
}

// wordPlaces asks where the block's candidate heading words land, by filling
// each of them with no colour: a colourless fill paints nothing and is
// reported all the same, so the word under the pointer is found out of the
// wrapping the frame already did rather than by measuring the prose again.
func (d *Document) wordPlaces(owner any, model []Span, spans []paragraph.SpanStyle, key [][]int) ([]paragraph.SpanStyle, [][]int) {
	if !d.looking(owner) {
		return spans, key
	}
	cands := d.candidates(owner, model)
	if len(cands) == 0 {
		return spans, key
	}
	if key == nil {
		key = make([][]int, len(spans))
	}
	base := 0
	for i := range spans {
		n := len(spans[i].Content)
		for c, w := range cands {
			if w.end <= base || w.start >= base+n {
				continue
			}
			spans[i].Fills = append(spans[i].Fills, paragraph.Fill{
				Start: max(w.start-base, 0),
				End:   min(w.end-base, n),
			})
			key[i] = append(key[i], placeKey(c))
		}
		base += n
	}
	return spans, key
}

// placeKey is how a word's place is told from a find match in the key
// [Document.onFill] dispatches on: matches count up from zero, words down
// from minus one.
func placeKey(cand int) int { return -1 - cand }

// wordResolve picks the candidate the pointer is inside out of what the
// layout reported, and asks for the frame that paints it as a link. A word
// carried onto two lines answers on whichever line the pointer is on.
func (d *Document) wordResolve(gtx layout.Context, owner any) {
	if !d.looking(owner) {
		return
	}
	cands := d.word.cands[owner]
	hit := wordHit{}
	for _, p := range d.word.placed {
		if p.cand >= len(cands) || !d.word.at.In(p.r) {
			continue
		}
		hit = wordHit{owner: owner, wordRange: cands[p.cand]}
		break
	}
	if hit == d.word.hit {
		return
	}
	d.word.hit = hit
	gtx.Execute(op.InvalidateCmd{})
}

// wordLink draws the resolved heading word as a link: the word carries the
// heading it goes to as its URL, so the paragraph gives it the link fill, the
// underline and the pointing hand every other link in the document has. The
// spans it falls inside are split around it, and the prose either side of it
// stays prose.
func (d *Document) wordLink(owner any, spans []paragraph.SpanStyle) []paragraph.SpanStyle {
	if !d.word.held || d.word.hit.owner != owner {
		return spans
	}
	url := d.wordURL()
	if url == "" {
		return spans
	}
	hit := d.word.hit
	out := make([]paragraph.SpanStyle, 0, len(spans)+2)
	base := 0
	for _, s := range spans {
		n := len(s.Content)
		from, to := max(hit.start-base, 0), min(hit.end-base, n)
		if s.URL != "" || from >= to {
			out = append(out, s)
			base += n
			continue
		}
		for _, part := range [3][2]int{{0, from}, {from, to}, {to, n}} {
			if part[0] >= part[1] {
				continue
			}
			p := s
			p.Content = s.Content[part[0]:part[1]]
			if part[0] == from {
				p.URL = url
			}
			out = append(out, p)
		}
		base += n
	}
	return out
}

// wordURL is where the resolved heading word goes: the heading's own text
// under a fragment mark, which is what a reader hearing the link announced is
// told about it.
func (d *Document) wordURL() string {
	i := d.word.hit.heading
	if i < 0 || i >= len(d.blocks) {
		return ""
	}
	h, ok := d.blocks[i].(*Heading)
	if !ok {
		return ""
	}
	return "#" + spanText(h.Spans)
}

// wordClick takes the heading word's own activation out of the paragraph's
// link callback: the word is the document's link and goes to the document's
// own heading, and every other link is still the caller's.
func (d *Document) wordClick(ps paragraph.Style, owner any) paragraph.Style {
	if !d.word.held || d.word.hit.owner != owner {
		return ps
	}
	url := d.wordURL()
	if url == "" {
		return ps
	}
	next := ps.OnLinkClick
	target := d.word.hit.heading
	ps.OnLinkClick = func(gtx layout.Context, u string) {
		if u == url {
			d.follow(gtx, target)
			return
		}
		if next != nil {
			next(gtx, u)
		}
	}
	return ps
}

// follow goes to the heading a heading word names: the document seats that
// heading and marks it, which is how a followed link arrives.
func (d *Document) follow(gtx layout.Context, heading int) {
	d.ScrollToBlock(heading)
	d.word.block = heading
	d.word.armed = gtx.Now
	gtx.Execute(op.InvalidateCmd{})
}

// candidates returns the heading words of one block's prose, in reading
// order: every word of its text that a heading names. A word inside a link or
// an inline code span is not one — it is a control already, or it is quoted
// out of the prose.
//
// The block's words are found once and kept: what a block reads is fixed, and
// which of its words are heading words is fixed with it.
func (d *Document) candidates(owner any, model []Span) []wordRange {
	if w, ok := d.word.cands[owner]; ok {
		return w
	}
	headings := d.headingWords()
	text := spanText(model)
	var out []wordRange
	for _, w := range words(text) {
		if quoted(model, w[0], w[1]) {
			continue
		}
		h, ok := headings[strings.ToLower(text[w[0]:w[1]])]
		if !ok {
			continue
		}
		out = append(out, wordRange{start: w[0], end: w[1], heading: h})
	}
	d.word.cands[owner] = out
	return out
}

// quoted reports whether the byte range [start,end) of what the spans read as
// reaches a link or an inline code span. Words are counted in the block's
// whole text, so a word set part in bold and part in body is one word and is
// looked up whole; a word that so much as touches a link or a code span is
// not a heading word at all.
func quoted(spans []Span, start, end int) bool {
	base := 0
	for _, s := range spans {
		n := len(s.Text)
		if start < base+n && end > base && (s.URL != "" || s.Code) {
			return true
		}
		base += n
	}
	return false
}

// headingWords indexes the document's headings by the words they are made of:
// a word answers the first heading in reading order that equals it or holds
// it whole, which is what makes the first match in reading order the one a
// word goes to.
//
// The index is of top-level headings only: [Document.ScrollToBlock] seats a
// top-level block, and a heading nested inside a quote or a list item has no
// index of its own to be seated by.
func (d *Document) headingWords() map[string]int {
	if d.word.headings != nil {
		return d.word.headings
	}
	ix := make(map[string]int)
	for i, b := range d.blocks {
		h, ok := b.(*Heading)
		if !ok {
			continue
		}
		text := spanText(h.Spans)
		for _, w := range words(text) {
			word := strings.ToLower(text[w[0]:w[1]])
			if _, seen := ix[word]; !seen {
				ix[word] = i
			}
		}
	}
	d.word.headings = ix
	return ix
}

// words returns the byte ranges of s's words: runs of letters and digits
// between runes that are neither.
func words(s string) [][2]int {
	var out [][2]int
	start := -1
	for i, r := range s {
		if isWord(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			out = append(out, [2]int{start, i})
			start = -1
		}
	}
	if start >= 0 {
		out = append(out, [2]int{start, len(s)})
	}
	return out
}

func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
