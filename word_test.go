package markdown_test

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// wordSource is the note the heading-word tests read: headings a word of the
// prose equals, headings that hold a word among others, a word that two
// headings hold, and words the rule keeps out — one inside a link, one inside
// an inline code span, and one that is only part of a longer word.
const wordSource = "# Setup\n\n" +
	"## Vault index\n\n" +
	"## Index of names\n\n" +
	"The setup is done once, and the SETUP holds. The index is rebuilt\n" +
	"when a note moves. Setups are not the setup, `index` is quoted out\n" +
	"of the prose, and [index](https://example.test) is a link already.\n"

// wordSize is the viewport the heading-word tests lay their note out in: wide
// enough that the prose wraps the way a reading column does.
var wordSize = image.Pt(420, 320)

// wordDoc parses the note and returns the document with the paragraph that
// carries the words.
func wordDoc(t *testing.T) (*markdown.Document, *markdown.Paragraph) {
	t.Helper()
	blocks := markdown.Parse([]byte(wordSource))
	d := markdown.NewDocument(blocks)
	for _, b := range blocks {
		if p, ok := b.(*markdown.Paragraph); ok {
			return d, p
		}
	}
	t.Fatal("the note holds no paragraph")
	return nil, nil
}

// TestHeadingWordLookup pins what a word of the prose is looked up against: a
// heading it equals, a heading that holds it among other words, the first
// such heading in reading order, its case ignored — and the words that are
// not heading words at all.
func TestHeadingWordLookup(t *testing.T) {
	d, para := wordDoc(t)
	got := markdown.HeadingWords(d, para)
	want := []markdown.HeadingWord{
		{Word: "setup", Heading: 0}, // equals the heading, case ignored
		{Word: "SETUP", Heading: 0},
		{Word: "index", Heading: 1}, // "Vault index" holds it, and it is first
		{Word: "setup", Heading: 0},
		{Word: "of", Heading: 2}, // "Index of names" holds it, whole word
	}
	if len(got) != len(want) {
		t.Fatalf("heading words = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("heading word %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestHeadingWordSkipsQuotedAndPartialWords is the other half of the rule,
// stated on the words the lookup must not offer: "Setups" holds "Setup" but
// is a longer word, and the two "index" occurrences inside an inline code
// span and inside a link are not prose to be linked.
func TestHeadingWordSkipsQuotedAndPartialWords(t *testing.T) {
	d, para := wordDoc(t)
	for _, w := range markdown.HeadingWords(d, para) {
		if w.Word == "Setups" {
			t.Error("Setups is offered as a heading word; the match must be a whole word")
		}
	}
	if n := len(markdown.HeadingWords(d, para)); n != 5 {
		t.Errorf("the paragraph offers %d heading words, want 5: the quoted and linked ones are not prose", n)
	}
}

// TestHeadingWordNothingIsLookedUpAtRest holds the rest state: with the key
// up the document looks nowhere, whatever the pointer is over.
func TestHeadingWordNothingIsLookedUpAtRest(t *testing.T) {
	shaper := defaultShaper(t)
	d, _ := wordDoc(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	golden.Capture(t, wordSize, themed(d, shaper, style, tokens.DefaultLight))
	if n := markdown.HeadingWordPlaces(d); n != 0 {
		t.Errorf("the document looked in %d places with the key up, want none", n)
	}
}

// TestHeadingWordRestsAsProse is the pixel half of the same: a document that
// has never seen the key and one whose key went down and came up again draw
// the same page. At rest a heading word is prose.
func TestHeadingWordRestsAsProse(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)

	plain, _ := wordDoc(t)
	rest := golden.Capture(t, wordSize, themed(plain, shaper, style, tokens.DefaultLight))

	d, para := wordDoc(t)
	w := themed(d, shaper, style, tokens.DefaultLight)
	held := headingWordFrames(t, d, para, w, 0)
	if diff := golden.PixelDiff(rest, held); diff == 0 {
		t.Error("the held key moved no pixels; the word under the pointer must draw as a link")
	}
	markdown.ReleaseHeadingWord(d)
	golden.Capture(t, wordSize, w)
	if diff := golden.PixelDiff(rest, golden.Capture(t, wordSize, w)); diff != 0 {
		t.Errorf("the released key left %d pixels moved; the prose must come back", diff)
	}
}

// headingWordFrames drives the frames a heading word takes: one to place the
// block's words, one to seat the pointer on the n-th of them, one to resolve
// it, and one that paints it as a link. It returns the last frame's capture.
func headingWordFrames(t *testing.T, d *markdown.Document, block any, w layout.Widget, n int) *image.RGBA {
	t.Helper()
	markdown.HoldHeadingWord(d, block)
	golden.Capture(t, wordSize, w)
	if _, ok := markdown.PointHeadingWord(d, block, n); !ok {
		t.Fatalf("the layout placed no heading word %d", n)
	}
	golden.Capture(t, wordSize, w)
	if _, ok := markdown.HeadingWordTarget(d); !ok {
		t.Fatal("the pointer sits on a heading word and the document resolved none")
	}
	return golden.Capture(t, wordSize, w)
}

// TestHeadingWordGolden records the note with the command key held and the
// pointer over a heading word, in both schemes: the word wears the link fill
// and the underline the document's own links wear, and the prose around it is
// untouched.
func TestHeadingWordGolden(t *testing.T) {
	shaper := defaultShaper(t)
	cases := []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"heading-word-light", tokens.DefaultLight},
		{"heading-word-dark", tokens.DefaultDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, para := wordDoc(t)
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography)
			w := themed(d, shaper, style, tc.colors)
			headingWordFrames(t, d, para, w, 0)
			golden.Render(t, tc.name, wordSize, w)
		})
	}
}

// followSource is the note the follow test reads: the note above, and enough
// after it that the paragraph carrying the words can be seated at the top of
// the viewport with the headings scrolled off it.
const followSource = wordSource + "\nFiller.\n\nFiller.\n\nFiller.\n\nFiller.\n\nFiller.\n"

// TestHeadingWordFollowGoesToTheHeading is the step the reader takes: with
// the command key held, a click on a heading word seats the heading it names
// and marks it, the way a followed link arrives.
func TestHeadingWordFollowGoesToTheHeading(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	blocks := markdown.Parse([]byte(followSource))
	d := markdown.NewDocument(blocks)

	r := new(gioinput.Router)
	ops := new(op.Ops)
	frame := func(evs ...event.Event) {
		r.Queue(evs...)
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(wordSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		paint.FillShape(gtx.Ops, tokens.DefaultLight.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
		d.Layout(gtx, shaper, style)
		r.Frame(ops)
	}
	// The paragraph is seated at the top of the viewport, and the find marks
	// say where in that viewport its first "setup" landed: a pointer event is
	// aimed in the window's own coordinates, which is not what the block's
	// own places are counted in.
	d.ScrollToBlock(3)
	frame()
	d.Find("setup", 0)
	frame()
	var at image.Point
	for _, m := range d.Matches() {
		if !m.Rect.Empty() {
			at = m.Rect.Min.Add(m.Rect.Max.Sub(m.Rect.Min).Div(2))
			break
		}
	}
	if at == (image.Point{}) {
		t.Fatal("no match of the word was laid out to aim at")
	}
	d.ClearFind()
	frame()

	pos := f32.Pt(float32(at.X), float32(at.Y))
	// The pointer arrives with the command key held, which is how the word
	// under it becomes a link.
	frame(pointer.Event{Kind: pointer.Move, Position: pos, Modifiers: key.ModShortcut, Source: pointer.Mouse})
	frame()
	frame()
	if target, ok := markdown.HeadingWordTarget(d); !ok || target != 0 {
		t.Fatalf("the word under the pointer goes to block %d (resolved %v), want the Setup heading at 0", target, ok)
	}
	// The word is a link now, so the click is the ordinary one a link takes.
	frame(
		pointer.Event{Kind: pointer.Press, Position: pos, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse, Modifiers: key.ModShortcut},
		pointer.Event{Kind: pointer.Release, Position: pos, Source: pointer.Mouse, Modifiers: key.ModShortcut},
	)
	frame()
	if first := d.Position().First; first != 0 {
		t.Errorf("the followed heading word left the viewport on block %d, want the heading at 0", first)
	}
	if b := markdown.ArrivalBlock(d); b != 0 {
		t.Errorf("the arrival marking is on block %d, want the heading at 0", b)
	}
}

// TestHeadingWordArrivalFades holds the marking's life: it shows whole, fades
// over the last of its life, and is gone once the life has run — never cut
// off, and never left behind.
func TestHeadingWordArrivalFades(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	if style.ArrivalFill.A == 0 {
		t.Fatal("FromTokens left the arrival fill with no alpha in it")
	}
	d, _ := wordDoc(t)
	ops := new(op.Ops)
	t0 := time.Now()
	frame := func(at time.Time) {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(wordSize),
			Ops:         ops,
			Now:         at,
		}
		d.Layout(gtx, shaper, style)
	}
	arm := layout.Context{Ops: ops, Now: t0}
	markdown.ArmArrival(d, arm, 0)

	hold := markdown.ArrivalLife - markdown.ArrivalFade
	frame(t0)
	if a := markdown.ArrivalAlpha(d); a != 1 {
		t.Errorf("the arrival marking opens at alpha %v, want all of it", a)
	}
	frame(t0.Add(hold))
	if a := markdown.ArrivalAlpha(d); a != 1 {
		t.Errorf("the arrival marking holds at alpha %v, want all of it until the fade opens", a)
	}
	frame(t0.Add(hold + markdown.ArrivalFade/2))
	if a := markdown.ArrivalAlpha(d); a <= 0 || a >= 1 {
		t.Errorf("half way through the fade the marking is at alpha %v, want some of it", a)
	}
	frame(t0.Add(markdown.ArrivalLife))
	if b := markdown.ArrivalBlock(d); b != -1 {
		t.Errorf("the marking is still on block %d once its life has run, want none", b)
	}
}

// TestHeadingWordInACodeSpanIsNotOffered is the fence's half of the rule,
// stated where a reader would look for it.
func TestHeadingWordInACodeSpanIsNotOffered(t *testing.T) {
	blocks := markdown.Parse([]byte("# Index\n\nThe `index` is quoted out of the prose.\n"))
	d := markdown.NewDocument(blocks)
	para := blocks[1].(*markdown.Paragraph)
	if got := markdown.HeadingWords(d, para); len(got) != 0 {
		t.Errorf("a word inside a code span is offered as a heading word: %v", got)
	}
}

// TestHeadingWordInALinkIsNotOffered is the same for a word that is already a
// control.
func TestHeadingWordInALinkIsNotOffered(t *testing.T) {
	blocks := markdown.Parse([]byte("# Index\n\nThe [index](https://example.test) is a link already.\n"))
	d := markdown.NewDocument(blocks)
	para := blocks[1].(*markdown.Paragraph)
	if got := markdown.HeadingWords(d, para); len(got) != 0 {
		t.Errorf("a word inside a link is offered as a heading word: %v", got)
	}
}
