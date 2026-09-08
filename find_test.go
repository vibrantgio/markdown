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
func findShot(t *testing.T, colors tokens.ColorTokens, set func(*markdown.Document)) (*markdown.Document, *image.RGBA) {
	t.Helper()
	shaper := defaultShaper(t)
	style := markdown.FromTokens(colors, tokens.DefaultTypography)
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
		colors tokens.ColorTokens
	}{{"light", tokens.DefaultLight}, {"dark", tokens.DefaultDark}} {
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
		colors tokens.ColorTokens
	}{{"light", tokens.DefaultLight}, {"dark", tokens.DefaultDark}} {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography)
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
			page := countExactly(marked, tc.colors.Background)
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
	colors := tokens.DefaultLight
	style := markdown.FromTokens(colors, tokens.DefaultTypography)
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
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	golden.Capture(t, image.Pt(560, 420), themed(d, shaper, style, tokens.DefaultLight))

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
	colors := tokens.DefaultLight
	shaper := defaultShaper(t)
	style := markdown.FromTokens(colors, tokens.DefaultTypography)
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
	colors := tokens.DefaultLight
	shaper := defaultShaper(t)
	style := markdown.FromTokens(colors, tokens.DefaultTypography)
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
