package markdown

import (
	"image"

	"gioui.org/layout"
)

// DistributeWidths exposes the column width distribution to the external
// test package.
var DistributeWidths = distributeWidths

// CodeOffset exposes how far a code block has been scrolled horizontally, in
// pixels, to the external test package. It is what a fence's own scrolling
// can be asserted on without going through the pixels.
func CodeOffset(d *Document, b *CodeBlock) int { return d.codeState(b).Offset() }

// HoldHeadingWord puts the command key down with the pointer inside block b,
// which is the state a held key and a hovering pointer make. A golden reaches
// it this way because a synthetic pointer move produces no hover repaint.
func HoldHeadingWord(d *Document, b any) {
	d.word.held = true
	d.word.owner = b
}

// PointHeadingWord moves the pointer to the middle of block b's n-th heading
// word, as the last layout placed it, and reports whether that layout placed
// it at all. It follows a frame laid out under [HoldHeadingWord].
func PointHeadingWord(d *Document, b any, n int) (image.Point, bool) {
	for _, p := range d.word.placed {
		if p.cand != n {
			continue
		}
		d.word.at = p.r.Min.Add(p.r.Max.Sub(p.r.Min).Div(2))
		return d.word.at, true
	}
	return image.Point{}, false
}

// ReleaseHeadingWord takes the command key up.
func ReleaseHeadingWord(d *Document) {
	d.word.held = false
	d.word.hit = wordHit{}
}

// HeadingWordPlaces is how many places the last layout put a heading word.
// It is zero while the key is up: nothing is looked up at rest.
func HeadingWordPlaces(d *Document) int { return len(d.word.placed) }

// HeadingWordTarget is the top-level heading the word under the pointer goes
// to, and whether there is a word under it at all.
func HeadingWordTarget(d *Document) (int, bool) {
	if d.word.hit.owner == nil {
		return 0, false
	}
	return d.word.hit.heading, true
}

// ArrivalBlock is the block a followed heading word marked, -1 for none.
func ArrivalBlock(d *Document) int { return d.word.block }

// HeadingWord is one word a block offers and the heading it goes to.
type HeadingWord struct {
	Word    string
	Heading int
}

// HeadingWords is what one block's prose offers, in reading order.
func HeadingWords(d *Document, b Block) []HeadingWord {
	var model []Span
	switch b := b.(type) {
	case *Heading:
		model = b.Spans
	case *Paragraph:
		model = b.Spans
	default:
		return nil
	}
	text := spanText(model)
	var out []HeadingWord
	for _, w := range d.candidates(b, model) {
		out = append(out, HeadingWord{Word: text[w.start:w.end], Heading: w.heading})
	}
	return out
}

// ArmArrival follows a heading word to the heading at index i, which is what
// a click on the word does.
func ArmArrival(d *Document, gtx layout.Context, i int) { d.follow(gtx, i) }

// ArrivalAlpha is how much of the arrival marking the last layout drew.
func ArrivalAlpha(d *Document) float64 { return d.word.alpha }

// ArrivalLife and ArrivalFade are the marking's hold and its fade.
var (
	ArrivalLife = arrivalLife
	ArrivalFade = arrivalFade
)
