package markdown_test

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// findSource holds the word "needle" once in every kind of block a query is
// searched in, in reading order: a heading, a paragraph, a list item, a
// quote, a table's header and body cells, and a fence.
var findSource = strings.Join([]string{
	"# A needle heading",
	"",
	"A paragraph carrying a needle in it.",
	"",
	"- a list item with a needle in it",
	"",
	"> a quoted line with a needle in it",
	"",
	"| a needle header | b |",
	"| --- | --- |",
	"| a needle cell | d |",
	"",
	"```",
	"code := needle(1)",
	"```",
	"",
}, "\n")

// countExactly returns how many pixels of img wear c exactly, ignoring alpha:
// glyphs drawn over a field are antialiased against it and do not answer, so
// the count measures the field and not the text on it.
func countExactly(img *image.RGBA, c color.NRGBA) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.R == c.R && p.G == c.G && p.B == c.B {
				n++
			}
		}
	}
	return n
}

// findShot lays the find source out at the given query and current index and
// returns the document and its capture. A nil query leaves the document
// unmarked.
func findShot(t *testing.T, colors tokens.PlatformColors, set func(*markdown.Document)) (*markdown.Document, *image.RGBA) {
	t.Helper()
	shaper := defaultShaper(t)
	style := markdown.FromTokens(colors, tokens.DefaultTypography, color.NRGBA{})
	d := markdown.NewDocument(markdown.Parse([]byte(findSource)))
	if set != nil {
		set(d)
	}
	return d, golden.Capture(t, image.Pt(560, 420), themed(d, shaper, style, colors))
}

// TestFindReadsEveryKindOfBlockInOrder is the reach of the query: prose,
// headings, list items, quoted blocks, both a table's header and its body
// cells, and code, each reported once and in the order they are read in.
func TestFindReadsEveryKindOfBlockInOrder(t *testing.T) {
	d := markdown.NewDocument(markdown.Parse([]byte(findSource)))
	d.Find("needle", 0)
	var got []int
	for _, m := range d.Matches() {
		got = append(got, m.Block)
	}
	want := []int{0, 1, 2, 3, 4, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("the query found %d matches in blocks %v, want %d in %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("match %d lies in block %d, want %d (all of them: %v, want %v)", i, got[i], want[i], got, want)
		}
	}
}

// TestFindIgnoresCase pins the query's one liberty: it matches whatever case
// the document is written in, and its own case decides nothing.
func TestFindIgnoresCase(t *testing.T) {
	blocks := markdown.Parse([]byte("Needle, needle, NEEDLE, and NeEdLe.\n"))
	for _, q := range []string{"needle", "NEEDLE", "NeedLE"} {
		d := markdown.NewDocument(blocks)
		d.Find(q, 0)
		if n := len(d.Matches()); n != 4 {
			t.Errorf("%q found %d matches, want 4", q, n)
		}
	}
}

// TestFindWithAnEmptyQueryMarksNothing is the resting state: a document that
// has never been asked for anything, and one whose query was cleared, are the
// document nobody searched.
func TestFindWithAnEmptyQueryMarksNothing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{{"light", tokens.PlatformLight}, {"dark", tokens.PlatformDark}} {
		t.Run(tc.name, func(t *testing.T) {
			_, plain := findShot(t, tc.colors, nil)
			d, empty := findShot(t, tc.colors, func(d *markdown.Document) { d.Find("", 0) })
			if n := len(d.Matches()); n != 0 {
				t.Errorf("an empty query found %d matches, want none", n)
			}
			if diff := golden.PixelDiff(plain, empty); diff != 0 {
				t.Errorf("the empty query moved %d pixels, want none", diff)
			}
			_, cleared := findShot(t, tc.colors, func(d *markdown.Document) {
				d.Find("needle", 0)
				d.ClearFind()
			})
			if diff := golden.PixelDiff(plain, cleared); diff != 0 {
				t.Errorf("clearing the query left %d pixels marked, want none", diff)
			}
		})
	}
}

// TestFindMarksItsMatchesAndTheCurrentOneMoreStrongly samples the two fields
// off the render: every match is marked with the plain fill, the current one
// with the stronger, and the page between them is untouched.
func TestFindMarksItsMatchesAndTheCurrentOneMoreStrongly(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.PlatformColors
	}{{"light", tokens.PlatformLight}, {"dark", tokens.PlatformDark}} {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography, color.NRGBA{})
			if style.MatchFill == style.CurrentMatchFill {
				t.Fatalf("both fills are %v; the current match cannot be told from the others", style.MatchFill)
			}
			_, plain := findShot(t, tc.colors, nil)
			if n := countExactly(plain, style.MatchFill); n != 0 {
				t.Fatalf("the unmarked page already wears the fill in %d pixels; the sampling cannot tell a mark from the page", n)
			}
			_, marked := findShot(t, tc.colors, func(d *markdown.Document) { d.Find("needle", 1) })
			if n := countExactly(marked, style.MatchFill); n == 0 {
				t.Errorf("no pixel wears %v; the matches are not marked", style.MatchFill)
			}
			if n := countExactly(marked, style.CurrentMatchFill); n == 0 {
				t.Errorf("no pixel wears %v; the current match is marked like the rest", style.CurrentMatchFill)
			}
			page := countExactly(marked, tc.colors.TextBackground)
			if page == 0 {
				t.Error("nothing is left of the page; the marking covered the document")
			}
		})
	}
}

// TestFindMovesTheStrongerFillWithTheCurrentIndex is the stepping the caller
// owns: the same query at a different index marks the same matches and moves
// only which of them is the current one.
func TestFindMovesTheStrongerFillWithTheCurrentIndex(t *testing.T) {
	colors := tokens.PlatformLight
	style := markdown.FromTokens(colors, tokens.DefaultTypography, color.NRGBA{})
	_, first := findShot(t, colors, func(d *markdown.Document) { d.Find("needle", 0) })
	_, third := findShot(t, colors, func(d *markdown.Document) { d.Find("needle", 2) })
	if golden.PixelDiff(first, third) == 0 {
		t.Fatal("stepping to another match changed nothing")
	}
	for _, img := range []*image.RGBA{first, third} {
		if n := countExactly(img, style.CurrentMatchFill); n == 0 {
			t.Fatal("one of the two steps marks no current match")
		}
	}
}

// TestFindReportsWhereTheMatchesLanded is what a caller scrolls by: a match
// has no rectangle until the document has been laid out, and once it has, the
// rectangles run down the page in the order the matches are read in.
func TestFindReportsWhereTheMatchesLanded(t *testing.T) {
	d := markdown.NewDocument(markdown.Parse([]byte(findSource)))
	d.Find("needle", 0)
	for i, m := range d.Matches() {
		if !m.Rect.Empty() {
			t.Fatalf("match %d reports %v before the first layout, want the empty rectangle", i, m.Rect)
		}
	}
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	golden.Capture(t, image.Pt(560, 420), themed(d, shaper, style, tokens.PlatformLight))

	matches := d.Matches()
	prev := -1
	for i, m := range matches {
		if m.Rect.Empty() {
			continue
		}
		if m.Rect.Min.Y < prev {
			t.Errorf("match %d sits at y %d, above match %d at y %d; the rectangles are out of reading order", i, m.Rect.Min.Y, i-1, prev)
		}
		prev = m.Rect.Min.Y
	}
	if prev < 0 {
		t.Fatal("no match reported a rectangle after the document was laid out")
	}
}

// TestFindRectanglesLandOnTheMarks walks the whole reporting chain: a
// rectangle is read in the document's own plane, so the inset the capture
// mounts the document at is the only thing between it and the pixels — and
// inside it the fill the match was marked with is there to be found, in a
// paragraph, in a list item, in a quote, in a table cell and in a fence
// alike.
func TestFindRectanglesLandOnTheMarks(t *testing.T) {
	colors := tokens.PlatformLight
	shaper := defaultShaper(t)
	style := markdown.FromTokens(colors, tokens.DefaultTypography, color.NRGBA{})
	d := markdown.NewDocument(markdown.Parse([]byte(findSource)))
	d.Find("needle", 1)
	// themed mounts the document at a uniform 8 dp inset, which is what
	// separates the document's plane from the capture's.
	const mount = 8
	img := golden.Capture(t, image.Pt(560, 420), themed(d, shaper, style, colors))

	seen := 0
	for i, m := range d.Matches() {
		if m.Rect.Empty() {
			continue
		}
		want := style.MatchFill
		if i == 1 {
			want = style.CurrentMatchFill
		}
		r := m.Rect.Add(image.Pt(mount, mount)).Intersect(img.Bounds())
		if !inside(img, r, want) {
			t.Errorf("match %d in block %d reports %v, but nothing there wears %v", i, m.Block, m.Rect, want)
			continue
		}
		seen++
	}
	if seen < 5 {
		t.Fatalf("only %d matches could be checked against their rectangle, want the marks in every kind of block", seen)
	}
}

// inside reports whether any pixel of img within r wears c.
func inside(img *image.RGBA, r image.Rectangle, c color.NRGBA) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.R == c.R && p.G == c.G && p.B == c.B {
				return true
			}
		}
	}
	return false
}

// TestFindRectanglesFollowTheScroll is the same chain with the reader
// somewhere else: the rectangles are read in the plane the document laid out
// into, so seating a later block at the top of the viewport moves them with
// it, and a match above the viewport is reported above it.
func TestFindRectanglesFollowTheScroll(t *testing.T) {
	colors := tokens.PlatformLight
	shaper := defaultShaper(t)
	style := markdown.FromTokens(colors, tokens.DefaultTypography, color.NRGBA{})
	blocks := markdown.Parse([]byte(findSource))
	d := markdown.NewDocumentAt(blocks, 3)
	d.Find("needle", 0)
	const mount = 8
	img := golden.Capture(t, image.Pt(560, 240), themed(d, shaper, style, colors))

	seen := 0
	for i, m := range d.Matches() {
		if m.Rect.Empty() {
			continue
		}
		if m.Rect.Max.Y <= 0 {
			// A list lays out one row above the one it starts at, and a
			// match in it is reported where it sits: off the top.
			if m.Block >= 3 {
				t.Errorf("match %d lies in block %d, at or below the viewport's first block, yet reports %v, above it", i, m.Block, m.Rect)
			}
			continue
		}
		if !inside(img, m.Rect.Add(image.Pt(mount, mount)).Intersect(img.Bounds()), style.MatchFill) {
			t.Errorf("match %d in block %d reports %v, but nothing there wears the fill", i, m.Block, m.Rect)
		}
		seen++
	}
	if seen == 0 {
		t.Fatal("a document seated at its fourth block reported no rectangle at all")
	}
}

// longFindSource is a document far taller than any viewport it is read in,
// carrying the query in its first section, in its middle and at its end: the
// document a reader steps through rather than reads at a glance.
func longFindSource() string {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		switch i {
		case 0, 20, 39:
			b.WriteString("Section with a needle in it.\n\n")
		default:
			b.WriteString("A section of prose that carries nothing anybody is looking for.\n\n")
		}
	}
	return b.String()
}

// findViewport is the viewport the stepping tests read the long document in:
// tall enough for several sections, far short of the whole.
var findViewport = image.Pt(560, 240)

// TestScrollToMatchBringsTheMatchIntoView is the step a reader takes: a match
// far below the viewport is asked for, and the frames that follow put it on
// screen — near the middle, where the eye is, rather than at the edge it came
// over.
func TestScrollToMatchBringsTheMatchIntoView(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	d := markdown.NewDocument(markdown.Parse([]byte(longFindSource())))
	d.Find("needle", 0)
	shot := func() { golden.Capture(t, findViewport, themed(d, shaper, style, tokens.PlatformLight)) }
	shot()
	if r := d.Matches()[2].Rect; !r.Empty() {
		t.Fatalf("the last match reports %v from the top of the document; it is not laid out there", r)
	}

	d.ScrollToMatch(2)
	// Two frames: the first seats the match's block, the second places the
	// match inside it.
	shot()
	shot()

	r := d.Matches()[2].Rect
	if r.Empty() {
		t.Fatal("the match was asked for and still reports no rectangle")
	}
	if r.Min.Y < 0 || r.Max.Y > findViewport.Y {
		t.Fatalf("the match sits at %v, outside the %d-high viewport", r, findViewport.Y)
	}
}

// TestScrollToMatchSeatsTheMatchNearTheMiddle is where a match that had to be
// brought in lands: near the middle of the viewport, which is where the eye
// is, and not at the edge it came over. The match is one in the middle of the
// document, because a match at either end is bounded by the document itself.
func TestScrollToMatchSeatsTheMatchNearTheMiddle(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	d := markdown.NewDocument(markdown.Parse([]byte(longFindSource())))
	d.Find("needle", 0)
	shot := func() { golden.Capture(t, findViewport, themed(d, shaper, style, tokens.PlatformLight)) }
	shot()

	d.ScrollToMatch(1)
	shot()
	shot()

	r := d.Matches()[1].Rect
	if r.Empty() {
		t.Fatal("the match was asked for and still reports no rectangle")
	}
	mid := findViewport.Y / 2
	if c := (r.Min.Y + r.Max.Y) / 2; c < mid-findViewport.Y/4 || c > mid+findViewport.Y/4 {
		t.Errorf("the match centres on %d in a %d-high viewport; a match brought in is seated near the middle", c, findViewport.Y)
	}
}

// TestScrollToMatchLeavesAMatchOnScreenWhereItIs is the other half of the
// step: the reader is not moved for a match they are already looking at.
func TestScrollToMatchLeavesAMatchOnScreenWhereItIs(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	d := markdown.NewDocument(markdown.Parse([]byte(longFindSource())))
	d.Find("needle", 0)
	shot := func() { golden.Capture(t, findViewport, themed(d, shaper, style, tokens.PlatformLight)) }
	shot()
	shot()
	before := d.Position()
	if r := d.Matches()[0].Rect; r.Empty() || r.Min.Y < 0 || r.Max.Y > findViewport.Y {
		t.Fatalf("the first match reports %v; the test needs it on screen to start with", r)
	}

	d.ScrollToMatch(0)
	shot()
	shot()
	if after := d.Position(); after.First != before.First || after.Offset != before.Offset {
		t.Errorf("stepping to a match already on screen moved the document from %+v to %+v", before, after)
	}
}

// TestMatchPlacesRunDownTheDocument pins what the scrollbar is handed: one
// place per match, in reading order, inside the unit interval — and, before
// the document has measured anything, the block's index over the block count,
// which is the approximation the estimate falls back to.
func TestMatchPlacesRunDownTheDocument(t *testing.T) {
	blocks := markdown.Parse([]byte(longFindSource()))
	d := markdown.NewDocument(blocks)
	d.Find("needle", 0)

	places := d.MatchPlaces()
	if len(places) != len(d.Matches()) {
		t.Fatalf("%d matches have %d places; the scrollbar takes one for each", len(d.Matches()), len(places))
	}
	prev := float32(-1)
	for i, p := range places {
		if p < 0 || p > 1 {
			t.Errorf("match %d lies at %v, outside the content", i, p)
		}
		if p < prev {
			t.Errorf("match %d lies at %v, above match %d at %v; the places are in reading order", i, p, i-1, prev)
		}
		prev = p
	}
	n := float32(len(blocks))
	for i, want := range []float32{0 / n, 20 / n, 39 / n} {
		if places[i] != want {
			t.Errorf("match %d of an unmeasured document lies at %v, want its block's index over the block count, %v", i, places[i], want)
		}
	}

}

// TestMatchPlacesCountTheHeightsTheLayoutMeasured is the other half of the
// estimate: a document opening on a block far taller than the rest has that
// height once it has drawn it, and the matches below the block are placed
// further down than the bare block index would put them.
func TestMatchPlacesCountTheHeightsTheLayoutMeasured(t *testing.T) {
	var b strings.Builder
	b.WriteString("```\n")
	for i := 0; i < 20; i++ {
		b.WriteString("a line of a fence that opens the document\n")
	}
	b.WriteString("```\n\n")
	for i := 0; i < 20; i++ {
		b.WriteString("A short paragraph with a needle in it.\n\n")
	}
	d := markdown.NewDocument(markdown.Parse([]byte(b.String())))
	d.Find("needle", 0)
	before := d.MatchPlaces()

	golden.Capture(t, findViewport, themed(d, defaultShaper(t),
		markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{}), tokens.PlatformLight))
	after := d.MatchPlaces()
	if after[0] <= before[0] {
		t.Errorf("the first match lies at %v once the fence over it has been measured and at %v before; a measured height counts", after[0], before[0])
	}
}
