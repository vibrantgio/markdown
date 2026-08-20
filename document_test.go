package markdown_test

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	themecolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// defaultShaper returns the shaper every golden here draws with: the default
// typography's faces pinned, system fonts off, so the stored images are the
// same on every machine. A golden test pins its faces with
// DeterministicShaper; application code takes the fallback Shaper. See
// AGENTS.md.
func defaultShaper(t *testing.T) *text.Shaper {
	t.Helper()
	return tokens.DefaultTypography.DeterministicShaper()
}

// themed wraps a document in a Background-filled widget so goldens capture
// the document on its token background.
func themed(d *markdown.Document, shaper *text.Shaper, style markdown.Style, c tokens.ColorTokens) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, c.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return d.Layout(gtx, shaper, style)
		})
	}
}

// ---- Golden-image tests ----

// TestCorpusDocumentGolden records or diffs the corpus document — every G6.2
// construct — in light and dark token themes.
func TestCorpusDocumentGolden(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse(corpus(t))
	size := image.Pt(560, 1500)
	cases := []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"corpus-light", tokens.DefaultLight},
		{"corpus-dark", tokens.DefaultDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography)
			d := markdown.NewDocument(blocks)
			golden.Render(t, tc.name, size, themed(d, shaper, style, tc.colors))
		})
	}
}

// TestTableDocumentGolden records or diffs a GFM table — emphasised header
// row on its surface, token borders, and left/centre/right column alignment —
// in light and dark token themes. The sample rows keep the old prism and
// cadence names: they render into table-{light,dark}.png, and G-G0D moves
// no pixels. They rename when those goldens are next deliberately
// regenerated.
func TestTableDocumentGolden(t *testing.T) {
	shaper := defaultShaper(t)
	src := "| Package | Role | Stars |\n" +
		"|:--------|:----:|------:|\n" +
		"| `prism` | primitives | 1200 |\n" +
		"| **markdown** | document rendering | 87 |\n" +
		"| cadence | patterns | 5 |\n"
	blocks := markdown.Parse([]byte(src))
	size := image.Pt(560, 180)
	cases := []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"table-light", tokens.DefaultLight},
		{"table-dark", tokens.DefaultDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography)
			d := markdown.NewDocument(blocks)
			golden.Render(t, tc.name, size, themed(d, shaper, style, tc.colors))
		})
	}
}

// TestTableNarrowGolden records or diffs the min-content behaviour under a
// narrow constraint: the word column keeps its longest word on one line, the
// prose column absorbs the whole squeeze by wrapping, and nothing paints
// across a column rule.
func TestTableNarrowGolden(t *testing.T) {
	shaper := defaultShaper(t)
	src := "| Shell | Description |\n" +
		"|:------|:------------|\n" +
		"| Compactline | a shell arranging its regions around a compact single line of content |\n" +
		"| Sidebar | a shell with a leading navigation region and a trailing content region |\n"
	blocks := markdown.Parse([]byte(src))
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	d := markdown.NewDocument(blocks)
	golden.Render(t, "table-narrow-light", image.Pt(300, 260),
		themed(d, shaper, style, tokens.DefaultLight))
}

// TestScrolledDocumentGolden records or diffs the corpus scrolled to the task
// list, proving the components/list viewport renders later blocks.
func TestScrolledDocumentGolden(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse(corpus(t))
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	d := markdown.NewDocumentAt(blocks, 9)
	golden.Render(t, "corpus-scrolled", image.Pt(560, 420),
		themed(d, shaper, style, tokens.DefaultLight))
}

// scrolledWithBar is themed for LayoutScrollbar: the same document on the
// same ground, with the design system's bar in a reserved gutter.
func scrolledWithBar(d *markdown.Document, shaper *text.Shaper, style markdown.Style, c tokens.ColorTokens) layout.Widget {
	bar := scrollbar.FromTokens(c)
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, c.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return d.LayoutScrollbar(gtx, shaper, style, bar, list.Occupy)
		})
	}
}

// TestScrollbarDocumentGolden records the corpus mid-document with the bar:
// the thumb sits away from both ends and is shorter than the track, which is
// the whole point of the treatment — position and proportion at a glance.
func TestScrollbarDocumentGolden(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse(corpus(t))
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	d := markdown.NewDocumentAt(blocks, 9)
	golden.Render(t, "corpus-scrollbar", image.Pt(560, 420),
		scrolledWithBar(d, shaper, style, tokens.DefaultLight))
}

// TestScrollbarOnlyWhenTheDocumentOverflows asserts the appearing half of the
// contract at the document level: a corpus far taller than the viewport draws
// a bar, and a two-line document in the same viewport draws none. The probe
// is the ink outside the row area — with Occupy the gutter is reserved either
// way, so dimensions cannot tell the two apart, but pixels can.
func TestScrollbarOnlyWhenTheDocumentOverflows(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	bar := scrollbar.FromTokens(tokens.DefaultLight)
	size := image.Pt(400, 300)

	render := func(blocks []markdown.Block) *image.RGBA {
		d := markdown.NewDocument(blocks)
		return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, tokens.DefaultLight.Background,
				clip.Rect{Max: gtx.Constraints.Max}.Op())
			return d.LayoutScrollbar(gtx, shaper, style, bar, list.Occupy)
		})
	}
	// A blank ground of the same size is the baseline: any difference in the
	// gutter column is the bar.
	blank := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, tokens.DefaultLight.Background,
			clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	gutter := func(img *image.RGBA) int {
		n := 0
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Max.X - 10; x < img.Bounds().Max.X; x++ {
				if img.RGBAAt(x, y) != blank.RGBAAt(x, y) {
					n++
				}
			}
		}
		return n
	}

	long := gutter(render(markdown.Parse(corpus(t))))
	if long == 0 {
		t.Error("a document taller than the viewport drew no scrollbar")
	}
	short := gutter(render(markdown.Parse([]byte("A short note.\n"))))
	if short != 0 {
		t.Errorf("a document that fits drew %d scrollbar pixels, want none", short)
	}
}

// ---- Code block overflow ----

// codeOverflowSource is the note the overflow goldens render: a fence whose
// first line is far wider than the column beside a fence that fits, so one
// image carries both halves of the treatment — the wide block dissolving at
// its cut edge with a bar in its bottom padding, the short block untouched.
const codeOverflowSource = "## A sample\n\n" +
	"```go\n" +
	"// A wikilink inside code is a code sample, not navigation:\n" +
	"// [[Design/Principles]]\n" +
	"func main() {}\n" +
	"```\n\n" +
	"A short one:\n\n" +
	"```\nfits\n```\n"

// codeOverflowSize is the viewport the overflow goldens render in: narrow
// enough that the sample's first line runs well past its right edge.
var codeOverflowSize = image.Pt(420, 300)

// driveDocument lays w out through an input router for two settling frames,
// queues evs, and settles again — the frame that absorbs a scroll still draws
// from the old offset, so the second pair is what the capture that follows
// sees.
func driveDocument(w layout.Widget, size image.Point, evs ...event.Event) {
	r := new(gioinput.Router)
	var ops op.Ops
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Ops:         &ops,
			Source:      r.Source(),
		}
		w(gtx)
		r.Frame(&ops)
	}
	frame()
	frame()
	r.Queue(evs...)
	frame()
	frame()
}

// TestCodeOverflowGolden records or diffs the fence that exposed the defect,
// at rest in both schemes and scrolled. At rest the long line dissolves into the fence at the
// right edge — the affordance that says there is more while a desktop overlay
// bar would already have faded out — and the short fence below it draws
// neither dissolve nor bar. Scrolled, the far end of the line is on screen,
// the dissolve has moved to the left edge, and the bar has moved with it.
//
// Both schemes are recorded because the fence's bar is the one part of the
// treatment whose legibility is not scheme-symmetric: it rests on the tinted
// code fill rather than on the page, and the light scheme is where the two
// come closest. See codeScrollbar.
//
// The scroll arrives as a real pointer gesture through a router rather than
// as a seeded offset, so these two images also witness that the remainder is
// reachable by scrolling.
func TestCodeOverflowGolden(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte(codeOverflowSource))
	cases := []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"code-overflow-light", tokens.DefaultLight},
		{"code-overflow-dark", tokens.DefaultDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography)
			d := markdown.NewDocument(blocks)
			golden.Render(t, tc.name, codeOverflowSize, themed(d, shaper, style, tc.colors))
		})
	}

	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	scrolled := themed(markdown.NewDocument(blocks), shaper, style, tokens.DefaultLight)
	driveDocument(scrolled, codeOverflowSize, pointer.Event{
		Kind:     pointer.Scroll,
		Position: f32.Pt(200, 70),
		Scroll:   f32.Pt(400, 0),
		Source:   pointer.Mouse,
	})
	golden.Render(t, "code-overflow-scrolled-light", codeOverflowSize, scrolled)
}

// TestCodeBlockClaimsHorizontalAxisOnly is the axis-separation proof at the
// document level: over the very same pixels, a horizontal gesture moves the
// code inside the fence and leaves the document where it was, and a vertical
// one scrolls the document and leaves the code where it was. A reader
// wheeling down a note therefore never gets stuck on a code block.
func TestCodeBlockClaimsHorizontalAxisOnly(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	// A long tail below the fence, so the document has somewhere to scroll to.
	blocks := markdown.Parse([]byte(codeOverflowSource + strings.Repeat("Filler paragraph.\n\n", 40)))
	cb, ok := blocks[1].(*markdown.CodeBlock)
	if !ok {
		t.Fatalf("block 1 is %T, want *CodeBlock", blocks[1])
	}
	// Aim at the fence's second line, clear of its scrollbar strip.
	over := f32.Pt(200, 70)

	cases := []struct {
		name       string
		scroll     f32.Point
		wantCode   bool
		wantColumn bool
	}{
		{name: "horizontal", scroll: f32.Pt(400, 0), wantCode: true},
		{name: "vertical", scroll: f32.Pt(0, 400), wantColumn: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := markdown.NewDocument(blocks)
			driveDocument(themed(d, shaper, style, tokens.DefaultLight), codeOverflowSize,
				pointer.Event{Kind: pointer.Scroll, Position: over, Scroll: tc.scroll, Source: pointer.Mouse})

			code := markdown.CodeOffset(d, cb) > 0
			pos := d.Position()
			column := pos.First > 0 || pos.Offset > 0
			if code != tc.wantCode {
				t.Errorf("code scrolled = %v (offset %d), want %v", code, markdown.CodeOffset(d, cb), tc.wantCode)
			}
			if column != tc.wantColumn {
				t.Errorf("document scrolled = %v (block %d, offset %d), want %v", column, pos.First, pos.Offset, tc.wantColumn)
			}
		})
	}
}

// TestCodeOffsetBounds pins where a fence's own scrolling stops: at the start
// however far back it is pushed, and at the last column of the widest line
// however far forward — never on empty ground past the code.
func TestCodeOffsetBounds(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	blocks := markdown.Parse([]byte(codeOverflowSource))
	cb := blocks[1].(*markdown.CodeBlock)
	over := f32.Pt(200, 70)

	d := markdown.NewDocument(blocks)
	w := themed(d, shaper, style, tokens.DefaultLight)
	driveDocument(w, codeOverflowSize, pointer.Event{
		Kind: pointer.Scroll, Position: over, Scroll: f32.Pt(10_000, 0), Source: pointer.Mouse,
	})
	end := markdown.CodeOffset(d, cb)
	if end <= 0 {
		t.Fatalf("code offset %d after scrolling to the end, want the overflow", end)
	}

	// Asking for more must not move it further: the end is the end.
	driveDocument(w, codeOverflowSize, pointer.Event{
		Kind: pointer.Scroll, Position: over, Scroll: f32.Pt(10_000, 0), Source: pointer.Mouse,
	})
	if got := markdown.CodeOffset(d, cb); got != end {
		t.Errorf("code offset %d after a second scroll past the end, want it held at %d", got, end)
	}

	driveDocument(w, codeOverflowSize, pointer.Event{
		Kind: pointer.Scroll, Position: over, Scroll: f32.Pt(-10_000, 0), Source: pointer.Mouse,
	})
	if got := markdown.CodeOffset(d, cb); got != 0 {
		t.Errorf("code offset %d after scrolling back past the start, want 0", got)
	}
}

// TestCodeBorderEdgesTheFenceWithoutMovingIt: a fence whose ground is too
// near the page to be seen against it takes a hairline, and taking one costs
// the document nothing. The block occupies the same box either way — the rim
// is drawn inside it, not around it — so a border can be switched on without
// anything below the block moving; the ground still fills the middle, and the
// line is on screen where it was not before.
//
// The probe is one fence rendered twice, differing in CodeBorder alone, on a
// ground deliberately set to the page's own colour: with no line that block is
// invisible, which is the case the field exists for.
func TestCodeBorderEdgesTheFenceWithoutMovingIt(t *testing.T) {
	shaper := defaultShaper(t)
	c := tokens.DefaultLight
	size := image.Pt(420, 120)
	blocks := markdown.Parse([]byte("```\nfits\n```\n"))

	style := markdown.FromTokens(c, tokens.DefaultTypography)
	style.CodeBackground = c.Background
	edged := style
	edged.CodeBorder = c.Divider

	measure := func(st markdown.Style) int {
		var ops op.Ops
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: size},
			Ops:         &ops,
		}
		return markdown.NewDocument(blocks).Layout(gtx, shaper, st).Size.Y
	}
	if a, b := measure(style), measure(edged); a != b {
		t.Errorf("the edged fence is %d px tall and the unedged one %d; an edge is drawn inside the block", b, a)
	}

	plain := golden.Capture(t, size, themed(markdown.NewDocument(blocks), shaper, style, c))
	rimmed := golden.Capture(t, size, themed(markdown.NewDocument(blocks), shaper, edged, c))
	if n := golden.PixelDiff(plain, rimmed); n == 0 {
		t.Fatal("the edged fence is pixel-identical to the unedged one; no line was drawn")
	}
	count := func(img *image.RGBA, want color.NRGBA) int {
		n := 0
		for i := 0; i+3 < len(img.Pix); i += 4 {
			if img.Pix[i] == want.R && img.Pix[i+1] == want.G && img.Pix[i+2] == want.B && img.Pix[i+3] == want.A {
				n++
			}
		}
		return n
	}
	// The document is inset by the themed helper, so the block is this wide;
	// a rim runs at least twice that far around it. Counting the difference
	// rather than the total ignores the odd anti-aliased glyph pixel that
	// happens to land on the same value.
	width := size.X - 16
	if got := count(rimmed, c.Divider) - count(plain, c.Divider); got < width {
		t.Errorf("%d pixels came out in the border colour; a rim around a block %d px wide is more than that", got, width)
	}
	if n := count(rimmed, style.CodeBackground); n == 0 {
		t.Error("the ground no longer fills the block")
	}
}

// TestShortCodeBlockDrawsNoScroller asserts the untouched half of the
// contract in pixels: a fence that fits draws nothing the scroll treatment
// brought. The probe is the same fence rendered with the bar style cleared —
// the one Style field the treatment added — and the two must be pixel-equal.
// That the fence is also unchanged from before the treatment existed is what
// every stored golden in this package says, none of which moved.
func TestShortCodeBlockDrawsNoScroller(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	size := image.Pt(420, 120)
	blocks := markdown.Parse([]byte("```\nfits\n```\n"))

	withBar := golden.Capture(t, size,
		themed(markdown.NewDocument(blocks), shaper, style, tokens.DefaultLight))

	barless := style
	barless.CodeScrollbar = scrollbar.Style{}
	plain := golden.Capture(t, size,
		themed(markdown.NewDocument(blocks), shaper, barless, tokens.DefaultLight))

	if n := golden.PixelDiff(withBar, plain); n != 0 {
		t.Errorf("a fence that fits drew %d pixels the same fence without a bar style did not; want none", n)
	}
}

// ---- Layout tests ----

func measureDoc(shaper *text.Shaper, style markdown.Style, blocks []markdown.Block, size image.Point) layout.Dimensions {
	var ops op.Ops
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: size},
		Ops:         &ops,
	}
	return markdown.NewDocument(blocks).Layout(gtx, shaper, style)
}

// TestLayoutColumnNaturalHeight verifies LayoutColumn takes exactly its
// content's height — no viewport filling, no internal scrolling: more blocks
// lay out strictly taller, and the height is independent of the vertical
// constraint.
func TestLayoutColumnNaturalHeight(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	one := markdown.Parse([]byte("alpha\n"))
	three := markdown.Parse([]byte("alpha\n\nbravo\n\n```\ncode\n```\n"))

	column := func(blocks []markdown.Block, size image.Point) layout.Dimensions {
		var ops op.Ops
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: size},
			Ops:         &ops,
		}
		return markdown.NewDocument(blocks).LayoutColumn(gtx, shaper, style)
	}

	oneH := column(one, image.Pt(560, 10_000)).Size.Y
	threeH := column(three, image.Pt(560, 10_000)).Size.Y
	if oneH == 0 || threeH <= oneH {
		t.Errorf("column heights one=%d three=%d; want 0 < one < three (natural content height)", oneH, threeH)
	}
	if short := column(three, image.Pt(560, 40)).Size.Y; short != threeH {
		t.Errorf("column height %d under a short constraint != unconstrained height %d; LayoutColumn must not fit a viewport", short, threeH)
	}
}

// TestCodeBlockOverflowScrolls verifies a code block never wraps or exceeds
// its constraint: an over-wide line keeps the block inside the narrow width,
// and the height matches the wide layout (same line count — the overflow
// scrolls horizontally instead of wrapping).
func TestCodeBlockOverflowScrolls(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	blocks := markdown.Parse([]byte("```\nthe first line is much much much much much wider than the narrow viewport\nshort line\n```\n"))

	wide := measureDoc(shaper, style, blocks, image.Pt(2000, 1000))
	narrow := measureDoc(shaper, style, blocks, image.Pt(240, 1000))

	if narrow.Size.X > 240 {
		t.Errorf("narrow code block width %d exceeds constraint 240", narrow.Size.X)
	}
	if narrow.Size.Y != wide.Size.Y {
		t.Errorf("narrow code block height %d != wide height %d; over-wide code must scroll, not wrap", narrow.Size.Y, wide.Size.Y)
	}
}

// TestDistributeWidths verifies the table column distribution: natural
// widths when they fit, slack-proportional shrinking floored at min-content
// when they don't, and the plain minima when even those overflow.
func TestDistributeWidths(t *testing.T) {
	naturals := []int{100, 300}
	mins := []int{80, 40}

	if got := markdown.DistributeWidths(naturals, mins, 500); !slicesEqual(got, naturals) {
		t.Errorf("fitting distribution %v; want naturals %v", got, naturals)
	}

	got := markdown.DistributeWidths(naturals, mins, 200)
	if sum(got) != 200 {
		t.Errorf("shrunk widths %v sum to %d; want the full 200", got, sum(got))
	}
	for i := range got {
		if got[i] < mins[i] {
			t.Errorf("column %d width %d below its min-content %d", i, got[i], mins[i])
		}
		if got[i] > naturals[i] {
			t.Errorf("column %d width %d above its natural %d", i, got[i], naturals[i])
		}
	}
	// The deficit comes out of slack: column 0 (slack 20) must give up far
	// less than column 1 (slack 260).
	if lost0, lost1 := naturals[0]-got[0], naturals[1]-got[1]; lost0 >= lost1 {
		t.Errorf("shrink %v took %d from the low-slack column and %d from the high-slack one; want the slack-rich column to absorb more", got, lost0, lost1)
	}

	if got := markdown.DistributeWidths(naturals, mins, 60); !slicesEqual(got, mins) {
		t.Errorf("overflow distribution %v; want minima %v", got, mins)
	}
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func slicesEqual(a, b []int) bool {
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

// TestTableNarrowKeepsWords verifies the min-content floor end to end: at a
// width where the proportional shrink would squeeze the first column below
// its longest word, the table still fits the constraint (the prose column
// wraps instead), and only truly impossible widths overflow into the
// horizontal scroll fallback.
func TestTableNarrowKeepsWords(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	src := "| Shell | Description |\n" +
		"|:------|:------------|\n" +
		"| Compactline | a shell arranging its regions around a compact single line of content |\n" +
		"| Sidebar | a shell with a leading navigation region and a trailing content region |\n"
	blocks := markdown.Parse([]byte(src))

	wide := measureDoc(shaper, style, blocks, image.Pt(2000, 2000))
	narrow := measureDoc(shaper, style, blocks, image.Pt(300, 2000))
	if narrow.Size.X > 300 {
		t.Errorf("narrow table width %d exceeds constraint 300; want the prose column to wrap within it", narrow.Size.X)
	}
	if narrow.Size.Y <= wide.Size.Y {
		t.Errorf("narrow table height %d not taller than wide height %d; the prose column should have wrapped", narrow.Size.Y, wide.Size.Y)
	}

	if tiny := measureDoc(shaper, style, blocks, image.Pt(60, 2000)); tiny.Size.X > 60 {
		t.Errorf("tiny table reports width %d beyond constraint 60; want the scroll fallback to clip the viewport", tiny.Size.X)
	}
}

// widgetProvider implements ImageProvider and WidgetImageProvider, counting
// widget requests and painting a fixed-size rect so layout is observable.
type widgetProvider struct {
	calls int
}

func (p *widgetProvider) Image(string) (image.Image, error) {
	return nil, fmt.Errorf("no raster")
}

func (p *widgetProvider) ImageWidget(string) (layout.Widget, error) {
	p.calls++
	return func(gtx layout.Context) layout.Dimensions {
		sz := image.Pt(40, 30)
		paint.FillShape(gtx.Ops, tokens.DefaultLight.Primary, clip.Rect{Max: sz}.Op())
		return layout.Dimensions{Size: sz}
	}, nil
}

// TestWidgetImageProvider verifies the vector hook: a provider implementing
// WidgetImageProvider serves the image as a widget (its size shows up in
// the layout), and the widget is requested once per block, not per frame.
func TestWidgetImageProvider(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	prov := &widgetProvider{}
	style.Images = prov
	blocks := markdown.Parse([]byte("![icon](icon.svg)\n"))
	d := markdown.NewDocument(blocks)

	layoutOnce := func() layout.Dimensions {
		var ops op.Ops
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(560, 560)},
			Ops:         &ops,
		}
		return d.Layout(gtx, shaper, style)
	}

	dims := layoutOnce()
	if dims.Size.Y < 30 {
		t.Errorf("document height %d; want at least the 30 px widget", dims.Size.Y)
	}
	layoutOnce()
	if prov.calls != 1 {
		t.Errorf("provider asked %d times over two frames; want the per-block cache to ask once", prov.calls)
	}
}

// TestNestedListIndents verifies each nesting level shifts content by the
// Indent column: three levels lay out strictly wider than one when width is
// unconstrained.
func TestNestedListIndents(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)

	flat := markdown.Parse([]byte("- alpha\n"))
	nested := markdown.Parse([]byte("- alpha\n  - alpha\n    - alpha\n"))

	fw := measureDoc(shaper, style, flat, image.Pt(2000, 1000)).Size.X
	nw := measureDoc(shaper, style, nested, image.Pt(2000, 1000)).Size.X
	if nw < fw+2*int(style.Indent) {
		t.Errorf("nested list width %d not at least two indents past flat width %d (indent %d)", nw, fw, int(style.Indent))
	}
}

// memProvider is an in-memory ImageProvider serving images by URL.
type memProvider map[string]image.Image

func (p memProvider) Image(url string) (image.Image, error) {
	img, ok := p[url]
	if !ok {
		return nil, fmt.Errorf("no image for %q", url)
	}
	return img, nil
}

// TestImageProvider lays out image blocks through an in-memory provider: a
// fitting image takes its natural height, an over-wide image is scaled down
// to the width constraint, and a URL the provider cannot serve falls back to
// the alt-text paragraph.
func TestImageProvider(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	style.Images = memProvider{
		"logo.png": image.NewNRGBA(image.Rect(0, 0, 48, 100)),
		"wide.png": image.NewNRGBA(image.Rect(0, 0, 2000, 100)),
	}

	fits := measureDoc(shaper, style, markdown.Parse([]byte("![the logo](logo.png)\n")), image.Pt(560, 400))
	if fits.Size.Y < 100 {
		t.Errorf("fitting image document height %d < image height 100", fits.Size.Y)
	}

	wide := measureDoc(shaper, style, markdown.Parse([]byte("![panorama](wide.png)\n")), image.Pt(560, 400))
	if wide.Size.Y >= 100 {
		t.Errorf("over-wide image document height %d not scaled down below 100", wide.Size.Y)
	}

	missing := measureDoc(shaper, style, markdown.Parse([]byte("![absent art](gone.png)\n")), image.Pt(560, 400))
	if missing.Size.Y == 0 {
		t.Error("missing image laid out with zero height; want alt-text fallback")
	}
	if missing.Size.Y >= fits.Size.Y {
		t.Errorf("alt-text fallback height %d not below image height %d", missing.Size.Y, fits.Size.Y)
	}
}

// TestDocumentLiveFrame drives Document.Layout through an input router for
// two frames, exercising the live richtext path (link registration and event
// draining) over the full corpus without a GPU.
func TestDocumentLiveFrame(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	d := markdown.NewDocument(markdown.Parse(corpus(t)))

	r := new(gioinput.Router)
	var ops op.Ops
	for range 2 {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(560, 3000)},
			Ops:         &ops,
			Source:      r.Source(),
		}
		if dims := d.Layout(gtx, shaper, style); dims.Size.Y == 0 {
			t.Fatal("document laid out with zero height")
		}
		r.Frame(&ops)
	}
}

// ---- Token defaults ----

// TestCodeReadsAtItsPagesWeight is the measurement behind codeInk, kept as a
// gate so the two appearances cannot drift apart again.
//
// A document's code is quieter than its prose in both appearances, deliberately
// — a fence is quoted matter, and it is set on its own fill besides. What must
// not differ is how much quieter, because a reader who switches appearance is
// reading the same document: code that recedes a step in one and half a page in
// the other is two different documents.
//
// The measurement is the travel from the fence's own fill to the code's ink,
// against the travel from the page to the prose's, on the perceptual lightness
// axis — "how far into the page's own range does this text go" — with the WCAG
// ratios logged beside it because the floor the syntax palette is fitted to is
// stated in those. Before the light half was moved a step it travelled 58% of
// its page's range where the dark half travelled 80%.
func TestCodeReadsAtItsPagesWeight(t *testing.T) {
	const wantAtLeast = 0.66
	travel := func(from, to color.NRGBA) float64 {
		a, _, _ := themecolor.OKLChFromNRGBA(from)
		b, _, _ := themecolor.OKLChFromNRGBA(to)
		return math.Abs(a - b)
	}
	var share [2]float64
	for i, tc := range []struct {
		name string
		c    tokens.ColorTokens
	}{{"light", tokens.DefaultLight}, {"dark", tokens.DefaultDark}} {
		st := markdown.FromTokens(tc.c, tokens.DefaultTypography)
		prose := travel(tc.c.Background, st.Text.Color)
		code := travel(st.CodeBackground, st.CodeColor)
		share[i] = code / prose
		t.Logf("%s: prose %.2f:1 on the page, code %.2f:1 on the fence — code travels %.0f%% of the page's own range",
			tc.name, themecolor.ContrastRatio(st.Text.Color, tc.c.Background),
			themecolor.ContrastRatio(st.CodeColor, st.CodeBackground), 100*share[i])
		if share[i] < wantAtLeast {
			t.Errorf("%s: code travels %.0f%% of the range its prose does; under %.0f%% a screenful of it reads washed",
				tc.name, 100*share[i], 100*wantAtLeast)
		}
		if st.CodeColor == st.Text.Color {
			t.Errorf("%s: code is inked in the prose colour; a fence is quoted matter and reads as such", tc.name)
		}
	}
	if d := math.Abs(share[0] - share[1]); d > 0.15 {
		t.Errorf("code travels %.0f%% of its page's range in one appearance and %.0f%% in the other", 100*share[0], 100*share[1])
	}
}

// TestFromTokensDefaults pins the FromTokens contract: the paper is the
// theme's page, heading levels take the typography's document heading scale,
// code shapes in the theme Code role's typeface and size on the Neutral 200
// fill, the quote bar is Primary with Neutral 700 text, and rules are
// separators using Divider.
func TestFromTokensDefaults(t *testing.T) {
	c, typo := tokens.DefaultLight, tokens.DefaultTypography
	st := markdown.FromTokens(c, typo)

	// The ground a document is read on is a role of the document's, and its
	// value is the theme's page — the same colour the furniture round it
	// fills a window with, held in the document's own name so that the two
	// can part later without either being renamed for it.
	if st.Paper != c.Background {
		t.Errorf("Paper = %v, want the theme's background %v", st.Paper, c.Background)
	}
	if dark := markdown.FromTokens(tokens.DefaultDark, typo); dark.Paper != tokens.DefaultDark.Background {
		t.Errorf("dark Paper = %v, want that theme's background %v", dark.Paper, tokens.DefaultDark.Background)
	}

	var wantSizes [6]unit.Sp
	for i := range wantSizes {
		wantSizes[i] = unit.Sp(typo.DocumentHeadings.Level(i + 1).Size)
	}
	if st.HeadingSizes != wantSizes {
		t.Errorf("HeadingSizes = %v, want the document scale's %v", st.HeadingSizes, wantSizes)
	}
	// The scale a document sets its headings in is not the one a screen sets
	// its own headline in: borrowing the display roles back would put a
	// document's title a quarter again taller than a reading surface sets one.
	if st.HeadingSizes[0] >= unit.Sp(typo.HeadlineLarge.Size) {
		t.Errorf("level 1 sets at %v, the HeadlineLarge display role at %v; the document scale must be the quieter of the two",
			st.HeadingSizes[0], typo.HeadlineLarge.Size)
	}
	if st.Text.Color != c.Text || st.Text.LinkColor != c.Primary {
		t.Errorf("Text colours = %v/%v, want Text/Primary", st.Text.Color, st.Text.LinkColor)
	}
	if st.CodeBackground != c.Ramps.Neutral.Step(200) {
		t.Errorf("CodeBackground = %v, want Neutral 200 %v", st.CodeBackground, c.Ramps.Neutral.Step(200))
	}
	// Plain code is the one colour the two appearances take a different step
	// for, and it is a measured difference rather than a taste: see codeInk.
	// A light document sets code the step below its body text and a dark one
	// the low-contrast text step, which is where both had it before the light
	// half was measured against the dark half.
	if st.CodeColor != c.Ramps.Neutral.Step(800) {
		t.Errorf("light CodeColor = %v, want Neutral 800 %v", st.CodeColor, c.Ramps.Neutral.Step(800))
	}
	if dark := markdown.FromTokens(tokens.DefaultDark, typo); dark.CodeColor != tokens.DefaultDark.Ramps.Neutral.Step(700) {
		t.Errorf("dark CodeColor = %v, want Neutral 700 %v", dark.CodeColor, tokens.DefaultDark.Ramps.Neutral.Step(700))
	}
	// A fence and an inline chip are one surface, so the constructor may not
	// quietly drift them apart.
	if st.CodeChip != st.CodeBackground {
		t.Errorf("CodeChip = %v, CodeBackground = %v; the code surface is one value", st.CodeChip, st.CodeBackground)
	}
	if st.QuoteBar != c.Primary || st.QuoteColor != c.Ramps.Neutral.Step(700) {
		t.Errorf("quote colours = %v/%v, want Primary/Neutral 700", st.QuoteBar, st.QuoteColor)
	}
	if st.RuleColor != c.Divider {
		t.Errorf("RuleColor = %v, want Divider %v", st.RuleColor, c.Divider)
	}
	if st.TableBorder != c.Divider || st.TableHeaderBackground != c.Ramps.Neutral.Step(300) {
		t.Errorf("table colours = %v/%v, want Divider/Neutral 300", st.TableBorder, st.TableHeaderBackground)
	}
	if want := font.Typeface(tokens.DefaultTypography.Code.Typeface); st.Mono != want {
		t.Errorf("Mono = %q, want the Code role's %q", st.Mono, want)
	}
	if want := unit.Sp(tokens.DefaultTypography.Code.Size); st.CodeSize != want {
		t.Errorf("CodeSize = %v, want the Code role's %v", st.CodeSize, want)
	}
	// Heading space is derived, not left to the caller: wider than the block
	// gap above every level and tighter below it.
	for i := range st.HeadingSpaceAbove {
		if st.HeadingSpaceAbove[i] <= st.BlockGap || st.HeadingSpaceBelow[i] >= st.BlockGap {
			t.Errorf("level %d heading space = %v/%v above/below against a %v block gap; want more above and less below",
				i+1, st.HeadingSpaceAbove[i], st.HeadingSpaceBelow[i], st.BlockGap)
		}
	}
}
