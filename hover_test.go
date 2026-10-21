package markdown_test

import (
	"image"
	stdcolor "image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
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
	style.Text.OnLinkHover = func(gtx layout.Context, url string) {
		if gtx.Ops == nil {
			t.Error("OnLinkHover received a layout.Context without Ops; callbacks must carry the live gtx")
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
