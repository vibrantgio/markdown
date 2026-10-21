package markdown_test

import (
	"image"
	stdcolor "image/color"
	"testing"

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

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// hoverSource is the note the hover tests read: two paragraphs, each with one
// link, so the pointer can cross from a link in one block onto a link in
// another — the crossing a document has to report as one arrival.
const hoverSource = "Read [the docs](https://docs.example/one) first.\n\n" +
	"Then [the guide](https://guide.example/two) after.\n"

// hoverSize is the viewport the hover tests lay the note out in.
var hoverSize = image.Pt(420, 240)

// TestDocumentReportsOneAddressUnderThePointer drives the pointer onto a
// link, straight onto a link in another block, back again, and off both,
// counting what the document told its caller.
func TestDocumentReportsOneAddressUnderThePointer(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, stdcolor.NRGBA{})
	var got []string
	style.Text.OnLink = func(gtx layout.Context, url string) {
		if gtx.Ops == nil {
			t.Error("OnLink received a layout.Context without Ops; callbacks must carry the live gtx")
		}
		got = append(got, url)
	}
	d := markdown.NewDocument(markdown.Parse([]byte(hoverSource)))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	frame := func(evs ...event.Event) {
		r.Queue(evs...)
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(hoverSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		paint.FillShape(gtx.Ops, tokens.PlatformLight.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
		d.Layout(gtx, shaper, style)
		r.Frame(ops)
	}
	// A link's place in the window is found the way the follow test finds a
	// word's: a query marks it, and the mark is in the window's own
	// coordinates, which is what a pointer event is aimed in.
	at := func(query string) image.Point {
		t.Helper()
		d.Find(query, 0)
		frame()
		for _, m := range d.Matches() {
			if !m.Rect.Empty() {
				p := m.Rect.Min.Add(m.Rect.Max.Sub(m.Rect.Min).Div(2))
				d.ClearFind()
				frame()
				return p
			}
		}
		d.ClearFind()
		t.Fatalf("the layout marked no place for %q", query)
		return image.Point{}
	}
	move := func(p image.Point) {
		frame(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(p.X), float32(p.Y)), Source: pointer.Mouse})
	}

	frame()
	frame()
	docs, guide := at("docs"), at("guide")
	got = got[:0]

	move(docs)
	if want := []string{"https://docs.example/one"}; !equalAddresses(got, want) {
		t.Fatalf("entering the first link reported %v, want %v", got, want)
	}
	frame()
	if len(got) != 1 {
		t.Fatalf("a frame with the pointer at rest reported %v, want one call in all", got)
	}

	// Down the page onto the second block's link, and back up to the first:
	// one arrival each way, never the departure the other block reported on
	// the same frame.
	move(guide)
	move(docs)
	want := []string{"https://docs.example/one", "https://guide.example/two", "https://docs.example/one"}
	if !equalAddresses(got, want) {
		t.Fatalf("crossing between blocks reported %v, want %v", got, want)
	}

	// Off the links onto the prose.
	move(image.Pt(docs.X, hoverSize.Y-2))
	want = append(want, "")
	if !equalAddresses(got, want) {
		t.Fatalf("leaving the links reported %v, want %v", got, want)
	}
}

func equalAddresses(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// boxNoteSource is the note the box tests read: a link in plain prose, one
// inside a blockquote, one in each of two list items and one in a table
// cell, so a box has to be carried out of every kind of nesting the document
// draws a paragraph in.
const boxNoteSource = "Read [the docs](https://docs.example/one) first.\n\n" +
	"> Quoted [the note](https://quote.example/three) inside.\n\n" +
	"- item one with [the first](https://list.example/four)\n" +
	"- item two with [the second](https://list.example/five)\n\n" +
	"| head | other |\n|---|---|\n| [the cell](https://cell.example/six) | plain |\n"

// TestTheLinkBoxHoldsThePointerWhereverTheLinkIs walks the pointer over
// the whole viewport and checks, at every position the document declares a
// link under, that the box it reports holds that position. The pointing hand
// is the document's own answer to "is the pointer on a link", so the two
// cannot disagree without the box being in the wrong plane — which is what
// carrying it out of a blockquote, a list item or a table cell can get
// wrong.
func TestTheLinkBoxHoldsThePointerWhereverTheLinkIs(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, stdcolor.NRGBA{})
	d := markdown.NewDocument(markdown.Parse([]byte(boxNoteSource)))

	size := image.Pt(360, 260)
	r := new(gioinput.Router)
	ops := new(op.Ops)
	frame := func(evs ...event.Event) {
		r.Queue(evs...)
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Ops:         ops,
			Source:      r.Source(),
		}
		paint.FillShape(gtx.Ops, tokens.PlatformLight.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
		d.Layout(gtx, shaper, style)
		r.Frame(ops)
	}
	frame()
	frame()

	on := 0
	for y := 1; y < size.Y; y += 2 {
		for x := 1; x < size.X; x += 5 {
			p := image.Pt(x, y)
			frame(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(x), float32(y)), Source: pointer.Mouse})
			if r.Cursor() != pointer.CursorPointer {
				if box, ok := d.LinkBox(); ok && p.In(box) {
					t.Fatalf("%v is on no link and the document reports a box %v holding it", p, box)
				}
				continue
			}
			on++
			box, ok := d.LinkBox()
			if !ok {
				t.Fatalf("%v is on a link and the document reports no box", p)
			}
			if !p.In(box) {
				t.Fatalf("%v is on a link and the document reports the box %v, which does not hold it", p, box)
			}
		}
	}
	// Five links in five kinds of nesting; a walk that found none of them
	// would pass every check above without testing anything.
	if on < 5 {
		t.Fatalf("the walk stood on a link at %d positions, too few for the note's five links", on)
	}
}

// TestTheLinkBoxMovesWithTheViewport scrolls the document under a
// resting pointer and reads the box: it is where the link is on the screen
// now, not where it was when the reader was at the top.
func TestTheLinkBoxMovesWithTheViewport(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, stdcolor.NRGBA{})
	d := markdown.NewDocument(markdown.Parse([]byte(boxNoteSource)))

	size := image.Pt(360, 160)
	r := new(gioinput.Router)
	ops := new(op.Ops)
	frame := func(evs ...event.Event) {
		r.Queue(evs...)
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Ops:         ops,
			Source:      r.Source(),
		}
		d.Layout(gtx, shaper, style)
		r.Frame(ops)
	}
	move := func(p image.Point) {
		frame(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(p.X), float32(p.Y)), Source: pointer.Mouse})
	}
	frame()
	frame()

	// Stand on the first link, wherever the faces put it on the first line.
	var at image.Point
	for x := 1; x < size.X && at == (image.Point{}); x += 2 {
		p := image.Pt(x, 8)
		move(p)
		if r.Cursor() == pointer.CursorPointer {
			at = p
		}
	}
	if at == (image.Point{}) {
		t.Fatal("the first line declares no link")
	}
	before, ok := d.LinkBox()
	if !ok {
		t.Fatal("the pointer is on the first link and the document reports no box")
	}

	d.ScrollToEnd()
	frame()
	frame()
	after, ok := d.LinkBox()
	if ok && after == before {
		t.Errorf("the document scrolled to the end and the link's box stayed at %v", before)
	}
}

// TestDocumentReportsTheLinkTheKeyboardReaches moves the keyboard through
// the note's links and reads back what the document told its caller and
// where it says the link is: a link reached by the keyboard says its address
// as one reached by the pointer does, and it has a box for an annotation to
// stand against.
func TestDocumentReportsTheLinkTheKeyboardReaches(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, stdcolor.NRGBA{})
	var got []string
	style.Text.OnLink = func(_ layout.Context, url string) { got = append(got, url) }
	d := markdown.NewDocument(markdown.Parse([]byte(hoverSource)))

	r := new(gioinput.Router)
	ops := new(op.Ops)
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(hoverSize),
			Ops:         ops,
			Source:      r.Source(),
		}
		paint.FillShape(gtx.Ops, tokens.PlatformLight.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
		d.Layout(gtx, shaper, style)
		r.Frame(ops)
	}

	frame()
	frame()
	if len(got) != 0 {
		t.Fatalf("a document nothing has reached reported %v", got)
	}
	if _, ok := d.LinkBox(); ok {
		t.Fatal("a document nothing has reached reports a box")
	}

	r.MoveFocus(key.FocusForward)
	frame()
	if want := []string{"https://docs.example/one"}; !equalAddresses(got, want) {
		t.Fatalf("the keyboard reaching the first link reported %v, want %v", got, want)
	}
	first, ok := d.LinkBox()
	if !ok || first.Empty() {
		t.Fatalf("the first link holds the keyboard and the document reports the box %v (%v)", first, ok)
	}

	r.MoveFocus(key.FocusForward)
	frame()
	want := []string{"https://docs.example/one", "https://guide.example/two"}
	if !equalAddresses(got, want) {
		t.Fatalf("the keyboard reaching the second link reported %v, want %v", got, want)
	}
	second, ok := d.LinkBox()
	if !ok || second.Empty() {
		t.Fatalf("the second link holds the keyboard and the document reports the box %v (%v)", second, ok)
	}
	if second.Min.Y <= first.Min.Y {
		t.Errorf("the second block's link reports the box %v, which does not stand under the first block's %v", second, first)
	}
}
