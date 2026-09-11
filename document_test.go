package markdown_test

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
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

// themed wraps a document in a Background-filled [layout.Widget] so goldens
// capture the document on its token background.
func themed(d *markdown.Document, shaper *text.Shaper, style markdown.Style, c tokens.PlatformColors) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, c.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return d.Layout(gtx, shaper, style)
		})
	}
}

// ---- Golden-image tests ----

// TestCorpusDocumentGolden records or diffs the corpus document — every
// supported construct — in light and dark token themes.
func TestCorpusDocumentGolden(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse(corpus(t))
	size := image.Pt(560, 1500)
	cases := []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"corpus-light", tokens.PlatformLight},
		{"corpus-dark", tokens.PlatformDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography, color.NRGBA{})
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
		colors tokens.PlatformColors
	}{
		{"table-light", tokens.PlatformLight},
		{"table-dark", tokens.PlatformDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography, color.NRGBA{})
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
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	d := markdown.NewDocument(blocks)
	golden.Render(t, "table-narrow-light", image.Pt(300, 260),
		themed(d, shaper, style, tokens.PlatformLight))
}

// TestScrolledDocumentGolden records or diffs the corpus scrolled to the task
// list, proving the components/list viewport renders later blocks.
func TestScrolledDocumentGolden(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse(corpus(t))
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	d := markdown.NewDocumentAt(blocks, 9)
	golden.Render(t, "corpus-scrolled", image.Pt(560, 420),
		themed(d, shaper, style, tokens.PlatformLight))
}

// scrolledWithBar is themed for LayoutScrollbar: the same document on the
// same background, with the design system's bar in a reserved gutter.
func scrolledWithBar(d *markdown.Document, shaper *text.Shaper, style markdown.Style, c tokens.PlatformColors) layout.Widget {
	bar := scrollbar.FromTokens(c, c.TextBackground)
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, c.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
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
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	d := markdown.NewDocumentAt(blocks, 9)
	golden.Render(t, "corpus-scrollbar", image.Pt(560, 420),
		scrolledWithBar(d, shaper, style, tokens.PlatformLight))
}

// TestScrollbarOnlyWhenTheDocumentOverflows asserts the appearing half of the
// contract at the document level: a corpus far taller than the viewport draws
// a bar, and a two-line document in the same viewport draws none. The probe
// is what is drawn outside the row area — with Occupy the gutter is reserved
// either way, so dimensions cannot tell the two apart, but pixels can.
func TestScrollbarOnlyWhenTheDocumentOverflows(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	bar := scrollbar.FromTokens(tokens.PlatformLight, tokens.PlatformLight.TextBackground)
	size := image.Pt(400, 300)

	render := func(blocks []markdown.Block) *image.RGBA {
		d := markdown.NewDocument(blocks)
		return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, tokens.PlatformLight.TextBackground,
				clip.Rect{Max: gtx.Constraints.Max}.Op())
			return d.LayoutScrollbar(gtx, shaper, style, bar, list.Occupy)
		})
	}
	// A blank background of the same size is the baseline: any difference in
	// the gutter column is the bar.
	blank := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, tokens.PlatformLight.TextBackground,
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

// TestCodeOverflowGolden records or diffs an over-wide fence, at rest in both
// schemes and scrolled. At rest the long line dissolves into the fence at the
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
		colors tokens.PlatformColors
	}{
		{"code-overflow-light", tokens.PlatformLight},
		{"code-overflow-dark", tokens.PlatformDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography, color.NRGBA{})
			d := markdown.NewDocument(blocks)
			golden.Render(t, tc.name, codeOverflowSize, themed(d, shaper, style, tc.colors))
		})
	}

	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	scrolled := themed(markdown.NewDocument(blocks), shaper, style, tokens.PlatformLight)
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
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
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
			driveDocument(themed(d, shaper, style, tokens.PlatformLight), codeOverflowSize,
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
// however far forward — never on empty background past the code.
func TestCodeOffsetBounds(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	blocks := markdown.Parse([]byte(codeOverflowSource))
	cb := blocks[1].(*markdown.CodeBlock)
	over := f32.Pt(200, 70)

	d := markdown.NewDocument(blocks)
	w := themed(d, shaper, style, tokens.PlatformLight)
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

// TestCodeBorderEdgesTheFenceWithoutMovingIt: a fence whose fill is too
// near the page to be seen against it takes a hairline, and taking one costs
// the document nothing. The block occupies the same box either way — the rim
// is drawn inside it, not around it — so a border can be switched on without
// anything below the block moving; the fill still covers the middle, and the
// line is on screen where it was not before.
//
// The probe is one fence rendered twice, differing in CodeBorder alone, on a
// fill deliberately set to the page's own colour: with no line that block is
// invisible, which is the case the field exists for.
func TestCodeBorderEdgesTheFenceWithoutMovingIt(t *testing.T) {
	shaper := defaultShaper(t)
	c := tokens.PlatformLight
	size := image.Pt(420, 120)
	blocks := markdown.Parse([]byte("```\nfits\n```\n"))

	style := markdown.FromTokens(c, tokens.DefaultTypography, color.NRGBA{})
	style.CodeBackground = c.TextBackground
	edged := style
	edged.CodeBorder = themecolor.Flatten(c.Separator, c.TextBackground)

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
	if got := count(rimmed, edged.CodeBorder) - count(plain, edged.CodeBorder); got < width {
		t.Errorf("%d pixels came out in the border colour; a rim around a block %d px wide is more than that", got, width)
	}
	if n := count(rimmed, style.CodeBackground); n == 0 {
		t.Error("the fill no longer covers the block")
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
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	size := image.Pt(420, 120)
	blocks := markdown.Parse([]byte("```\nfits\n```\n"))

	withBar := golden.Capture(t, size,
		themed(markdown.NewDocument(blocks), shaper, style, tokens.PlatformLight))

	barless := style
	barless.CodeScrollbar = scrollbar.Style{}
	plain := golden.Capture(t, size,
		themed(markdown.NewDocument(blocks), shaper, barless, tokens.PlatformLight))

	if n := golden.PixelDiff(withBar, plain); n != 0 {
		t.Errorf("a fence that fits drew %d pixels the same fence without a bar style did not; want none", n)
	}
}

// ---- Measure ----

// measureSource is the note the measure tests render: prose long enough to
// wrap several times at a reading width and not once at the viewport's, above
// a fence whose first line runs past the reading width either way. One image
// then carries both halves of what a measure is for — lines that stop where
// the reader can find the next one, and content that keeps its own size
// scrolling inside the column instead of widening it.
const measureSource = "## A sample\n\n" +
	"A line of prose is easier to read when it stops well short of a wide " +
	"window: the eye finds the start of the next line without hunting back " +
	"along it. So the paragraph wraps at the measure however wide the " +
	"window is opened, and the page shows on both sides of it.\n\n" +
	"```go\n" +
	"// A wikilink inside code is a code sample, not navigation:\n" +
	"// [[Design/Principles]]\n" +
	"func main() {}\n" +
	"```\n"

// measureWidth is the reading width the measure tests set, and
// measureWideSize the viewport they set it in: wide enough that the column
// stands well clear of both edges, so a capture shows the page beside it
// rather than a column that merely happens to fit.
const measureWidth = unit.Dp(360)

var measureWideSize = image.Pt(900, 380)

// measurePage is the leading edge of the region a themed document is laid
// out in — themed's own inset — and measureRegion the width of that region
// inside measureWideSize. The insets the measure divides are taken from the
// region and not from the image.
const measurePage = 8

var measureRegion = measureWideSize.X - 2*measurePage

// measuredStyle builds the light or dark measure style: the token style with
// the reading width set and nothing else changed.
func measuredStyle(c tokens.PlatformColors) markdown.Style {
	style := markdown.FromTokens(c, tokens.DefaultTypography, color.NRGBA{})
	style.Measure = measureWidth
	return style
}

// pageExtent returns the first and last column of img holding anything other
// than the page colour, and reports whether it found any. With a measure set
// that pair is the reading column's own edges, which is what says where the
// column was seated and how wide it came out.
func pageExtent(img *image.RGBA, page color.NRGBA) (lead, trail int, ok bool) {
	b := img.Bounds()
	lead, trail = b.Max.X, b.Min.X-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.R == page.R && p.G == page.G && p.B == page.B && p.A == page.A {
				continue
			}
			lead = min(lead, x)
			trail = max(trail, x)
		}
	}
	return lead, trail, lead <= trail
}

// TestMeasureGolden records or diffs a document read at a measure in a
// viewport far wider than it: the column is centred with page showing on both
// sides, and the fence inside it is cut at the measure — the long line
// dissolving at the column's own trailing edge rather than running on to the
// window's. Both schemes are recorded because the fence's fill and the
// dissolve over it are what say where the column ends, and neither is
// scheme-symmetric.
func TestMeasureGolden(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte(measureSource))
	cases := []struct {
		name   string
		colors tokens.PlatformColors
	}{
		{"measure-wide-light", tokens.PlatformLight},
		{"measure-wide-dark", tokens.PlatformDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := markdown.NewDocument(blocks)
			golden.Render(t, tc.name, measureWideSize,
				themed(d, shaper, measuredStyle(tc.colors), tc.colors))
		})
	}
}

// TestMeasureCentresTheColumn asserts the two halves of the rule over the
// same pixels: a block stops at the measure however wide the viewport is, and
// what is left over is divided evenly on either side of it. The zero measure
// is the control, taken in the same viewport: it gives the blocks the whole
// region, which is what every other stored golden here was recorded at.
func TestMeasureCentresTheColumn(t *testing.T) {
	shaper := defaultShaper(t)
	c := tokens.PlatformLight
	blocks := markdown.Parse([]byte(measureSource))

	full := golden.Capture(t, measureWideSize,
		themed(markdown.NewDocument(blocks), shaper, markdown.FromTokens(c, tokens.DefaultTypography, color.NRGBA{}), c))
	lead, trail, ok := pageExtent(full, c.TextBackground)
	if !ok {
		t.Fatal("a document with no measure drew nothing")
	}
	if got := trail - lead + 1; got != measureRegion {
		t.Errorf("with no measure the blocks came out %d px wide in a %d px region; a zero measure takes the width", got, measureRegion)
	}

	held := golden.Capture(t, measureWideSize,
		themed(markdown.NewDocument(blocks), shaper, measuredStyle(c), c))
	lead, trail, ok = pageExtent(held, c.TextBackground)
	if !ok {
		t.Fatal("a document read at a measure drew nothing")
	}
	if got, want := trail-lead+1, int(measureWidth); got != want {
		t.Errorf("the column came out %d px wide, want the measure's %d", got, want)
	}
	before, after := lead-measurePage, measurePage+measureRegion-1-trail
	if d := before - after; d > 1 || d < -1 {
		t.Errorf("the column sits %d px from the leading edge and %d from the trailing one; a measure centres what it does not fill", before, after)
	}
}

// fenceRow returns a row of img running through the code block — the middle
// of the longest run of rows holding the fence's own fill — or -1 when the
// capture holds no fence. A gesture aimed at a hard-coded row lands wherever
// the prose above it happens to end.
func fenceRow(img *image.RGBA, fill color.NRGBA) int {
	b := img.Bounds()
	best, bestLen, run := -1, 0, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		on := false
		for x := b.Min.X; x < b.Max.X && !on; x++ {
			p := img.RGBAAt(x, y)
			on = p.R == fill.R && p.G == fill.G && p.B == fill.B && p.A == fill.A
		}
		if on {
			run++
			if run > bestLen {
				bestLen, best = run, y-run/2
			}
			continue
		}
		run = 0
	}
	return best
}

// TestMeasureBoundsTheScrollArea asserts that wide content inside a measured
// column scrolls inside the column: a horizontal gesture over the fence moves
// code, and every pixel it moves lies between the column's own edges. A
// scroll area wider than the measure would paint the moved code out over the
// page beside the column, and this is what would catch it.
func TestMeasureBoundsTheScrollArea(t *testing.T) {
	shaper := defaultShaper(t)
	c := tokens.PlatformLight
	blocks := markdown.Parse([]byte(measureSource))
	style := measuredStyle(c)

	rest := golden.Capture(t, measureWideSize,
		themed(markdown.NewDocument(blocks), shaper, style, c))
	lead, trail, ok := pageExtent(rest, c.TextBackground)
	if !ok {
		t.Fatal("a document read at a measure drew nothing")
	}

	fence := fenceRow(rest, style.CodeBackground)
	if fence < 0 {
		t.Fatal("no fence in the capture; the gesture would land on prose")
	}
	w := themed(markdown.NewDocument(blocks), shaper, style, c)
	driveDocument(w, measureWideSize, pointer.Event{
		Kind:     pointer.Scroll,
		Position: f32.Pt(float32(lead+40), float32(fence)),
		Scroll:   f32.Pt(400, 0),
		Source:   pointer.Mouse,
	})
	scrolled := golden.Capture(t, measureWideSize, w)
	if n := golden.PixelDiff(rest, scrolled); n == 0 {
		t.Fatal("a horizontal gesture over the fence moved nothing; the code inside the measure does not scroll")
	}
	outside := 0
	b := rest.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if x >= lead && x <= trail {
				continue
			}
			if rest.RGBAAt(x, y) != scrolled.RGBAAt(x, y) {
				outside++
			}
		}
	}
	if outside != 0 {
		t.Errorf("%d pixels outside the column changed when the fence scrolled; the scroll area is wider than the measure", outside)
	}
}

// TestMeasureKeepsTheGutterAtTheEdge asserts the one place the two rules meet.
// A measure within a gutter's width of the region cannot be centred without
// running the column under the bar, so the column is seated leading of centre
// and the gutter is left whole — the strip a scrollbar needs is the
// viewport's, not the measure's to spend.
func TestMeasureKeepsTheGutterAtTheEdge(t *testing.T) {
	shaper := defaultShaper(t)
	c := tokens.PlatformLight
	const gutter = 12
	style := markdown.FromTokens(c, tokens.DefaultTypography, color.NRGBA{})
	style.Gutter = gutter
	style.Measure = unit.Dp(measureRegion - gutter - 2)
	blocks := markdown.Parse([]byte(measureSource))

	img := golden.Capture(t, measureWideSize,
		themed(markdown.NewDocument(blocks), shaper, style, c))
	lead, trail, ok := pageExtent(img, c.TextBackground)
	if !ok {
		t.Fatal("a document read at a near-region measure drew nothing")
	}
	if got, want := trail-lead+1, int(style.Measure); got != want {
		t.Errorf("the column came out %d px wide, want the measure's %d", got, want)
	}
	if got := measurePage + measureRegion - 1 - trail; got != gutter {
		t.Errorf("the column left %d px at the trailing edge, want the gutter's %d", got, gutter)
	}
	if got := lead - measurePage; got != 2 {
		t.Errorf("the column sits %d px from the leading edge, want the 2 px the gutter leaves it", got)
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
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
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
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
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
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
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
// [layout.Widget] requests and painting a fixed-size rect so layout is
// observable.
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
		paint.FillShape(gtx.Ops, tokens.PlatformLight.ControlAccent, clip.Rect{Max: sz}.Op())
		return layout.Dimensions{Size: sz}
	}, nil
}

// TestWidgetImageProvider verifies the vector hook: a provider implementing
// WidgetImageProvider serves the image as a [layout.Widget] (its size shows
// up in the layout), and it is requested once per block, not per frame.
func TestWidgetImageProvider(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
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
		t.Errorf("document height %d; want at least the 30 px layout.Widget", dims.Size.Y)
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
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})

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
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
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

// ---- Task checkbox interaction ----

func driveTaskFrame(w layout.Widget, ops *op.Ops, r *gioinput.Router, size image.Point) {
	ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: size},
		Ops:         ops,
		Source:      r.Source(),
	}
	w(gtx)
	r.Frame(ops)
}

func taskColumn(d *markdown.Document, shaper *text.Shaper, style markdown.Style) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, tokens.PlatformLight.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return d.LayoutColumn(gtx, shaper, style)
	}
}

// TestTaskClickFiresOnTaskClick drives a pointer press+release over the 14 dp
// mark and expects OnTaskClick to fire with the *ListItem Parse produced —
// the same pointer — and a live layout.Context (GX.8). A click on the item
// text does not fire: the mark is the hit target, not the row.
func TestTaskClickFiresOnTaskClick(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte("- [ ] open task\n- [x] done task\n"))
	l, ok := blocks[0].(*markdown.List)
	if !ok {
		t.Fatalf("Parse returned %T, want *List", blocks[0])
	}
	if len(l.Items) != 2 {
		t.Fatalf("list has %d items, want 2", len(l.Items))
	}
	open, done := l.Items[0], l.Items[1]

	var got *markdown.ListItem
	var gotOps bool
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	style.OnTaskClick = func(gtx layout.Context, item *markdown.ListItem) {
		got = item
		gotOps = gtx.Ops != nil
	}

	d := markdown.NewDocument(blocks)
	size := image.Pt(400, 80)
	w := taskColumn(d, shaper, style)

	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveTaskFrame(w, ops, r, size)

	hit := f32.Pt(7, 13)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: hit, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: hit, Source: pointer.Mouse},
	)
	driveTaskFrame(w, ops, r, size)
	if got != open {
		t.Fatalf("OnTaskClick item = %p, want the open task %p", got, open)
	}
	if !gotOps {
		t.Error("OnTaskClick received a layout.Context without Ops; callbacks must carry the live gtx (GX.8)")
	}

	got = nil
	miss := f32.Pt(80, hit.Y)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: miss, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: miss, Source: pointer.Mouse},
	)
	driveTaskFrame(w, ops, r, size)
	if got != nil {
		t.Error("click on the item text fired OnTaskClick; the mark is the hit target, not the row")
	}

	hit2 := f32.Pt(7, 41)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: hit2, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: hit2, Source: pointer.Mouse},
	)
	driveTaskFrame(w, ops, r, size)
	if got != done {
		t.Fatalf("second box OnTaskClick item = %p, want the done task %p", got, done)
	}
}

// TestTaskClickKeyboardActivation moves focus onto the first task box and
// activates it with Enter, then the second with Space — the platform
// activate keys, once the target is focused.
func TestTaskClickKeyboardActivation(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte("- [ ] open task\n- [x] done task\n"))
	l := blocks[0].(*markdown.List)
	open, done := l.Items[0], l.Items[1]

	var clicks []*markdown.ListItem
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	style.OnTaskClick = func(_ layout.Context, item *markdown.ListItem) {
		clicks = append(clicks, item)
	}

	d := markdown.NewDocument(blocks)
	size := image.Pt(400, 80)
	w := taskColumn(d, shaper, style)

	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveTaskFrame(w, ops, r, size)

	r.MoveFocus(key.FocusForward)
	driveTaskFrame(w, ops, r, size)
	r.Queue(
		key.Event{Name: key.NameReturn, State: key.Press},
		key.Event{Name: key.NameReturn, State: key.Release},
	)
	driveTaskFrame(w, ops, r, size)
	if len(clicks) != 1 || clicks[0] != open {
		t.Fatalf("after Enter, clicks = %v, want [%p open]", clicks, open)
	}

	r.MoveFocus(key.FocusForward)
	driveTaskFrame(w, ops, r, size)
	r.Queue(
		key.Event{Name: key.NameSpace, State: key.Press},
		key.Event{Name: key.NameSpace, State: key.Release},
	)
	driveTaskFrame(w, ops, r, size)
	if len(clicks) != 2 || clicks[1] != done {
		t.Fatalf("after Space, clicks = %v, want [open %p done]", clicks, done)
	}
}

// TestTaskClickIdlePixelsUnchanged holds the idle half of the contract: a
// nil OnTaskClick and a set one draw the same pixels while nothing is
// hovered or focused. The hook is a hit target, not a new glyph.
func TestTaskClickIdlePixelsUnchanged(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte("- [ ] open\n- [x] done\n"))
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	live := style
	live.OnTaskClick = func(layout.Context, *markdown.ListItem) {}
	size := image.Pt(400, 80)

	draw := func(st markdown.Style) layout.Widget {
		return taskColumn(markdown.NewDocument(blocks), shaper, st)
	}
	idle := golden.Capture(t, size, draw(style))
	hooked := golden.Capture(t, size, draw(live))
	if n := golden.PixelDiff(idle, hooked); n != 0 {
		t.Errorf("setting OnTaskClick moved %d idle pixels; the mark is a hit target, not a new glyph", n)
	}
}

// TestDocumentLiveFrame drives Document.Layout through an input router for
// two frames, exercising the live paragraph path (link registration and event
// draining) over the full corpus without a GPU.
func TestDocumentLiveFrame(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
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

// TestFromTokensDefaults pins the FromTokens contract against the platform's
// colour set: the page a document is read on, the fence and its chip on the
// platform's one step off that page inside a separator, code set in the text
// colour, the quote bar and an open task's box at the weakest label strength,
// rules the separator and a table's lines the platform's grid.
func TestFromTokensDefaults(t *testing.T) {
	typo := tokens.DefaultTypography
	for _, tc := range []struct {
		name string
		p    tokens.PlatformColors
	}{{"light", tokens.PlatformLight}, {"dark", tokens.PlatformDark}} {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.p
			st := markdown.FromTokens(p, typo, color.NRGBA{})

			// Unstated, the surface a document is read on is the platform's
			// text background — what it fills a text view with.
			if st.ContentSurface != p.TextBackground {
				t.Errorf("ContentSurface = %v, want TextBackground %v", st.ContentSurface, p.TextBackground)
			}
			// Stated, it is the caller's, and every mark laid over it follows.
			card := p.CardFill
			onCard := markdown.FromTokens(p, typo, card)
			if onCard.ContentSurface != card {
				t.Errorf("stated ContentSurface = %v, want %v", onCard.ContentSurface, card)
			}
			if onCard.MatchFill == st.MatchFill {
				t.Errorf("a document on %v marks a match in %v, the same fill as one on %v; the marks are not following the surface",
					card, onCard.MatchFill, st.ContentSurface)
			}

			var wantSizes [6]unit.Sp
			for i := range wantSizes {
				wantSizes[i] = unit.Sp(typo.DocumentHeadings.Level(i + 1).Size)
			}
			if st.HeadingSizes != wantSizes {
				t.Errorf("HeadingSizes = %v, want the document scale's %v", st.HeadingSizes, wantSizes)
			}
			// The scale a document sets its headings in is not the one a screen
			// sets its own headline in: borrowing the display roles back would
			// put a document's title a quarter again taller than a reading
			// surface sets one.
			if st.HeadingSizes[0] >= unit.Sp(typo.HeadlineLarge.Size) {
				t.Errorf("level 1 sets at %v, the HeadlineLarge display role at %v; the document scale must be the smaller of the two",
					st.HeadingSizes[0], typo.HeadlineLarge.Size)
			}

			if st.Text.Color != p.Text || st.Text.LinkColor != p.Link {
				t.Errorf("prose colours = %v/%v, want Text %v and Link %v", st.Text.Color, st.Text.LinkColor, p.Text, p.Link)
			}
			if want := themecolor.Flatten(p.AlternatingContentBackground, p.TextBackground); st.CodeBackground != want {
				t.Errorf("CodeBackground = %v, want the alternating content fill over the page %v", st.CodeBackground, want)
			}
			if want := themecolor.Flatten(p.Separator, st.CodeBackground); st.CodeBorder != want {
				t.Errorf("CodeBorder = %v, want the separator over the fence %v", st.CodeBorder, want)
			}
			if st.CodeColor != p.Text {
				t.Errorf("CodeColor = %v, want Text %v — code is text, in a monospace face", st.CodeColor, p.Text)
			}
			// A fence and an inline chip are one surface, so the constructor may
			// not quietly drift them apart — the edge included, the edge being
			// half of what a code surface is.
			if st.CodeChip != st.CodeBackground {
				t.Errorf("CodeChip = %v, CodeBackground = %v; the code surface is one value", st.CodeChip, st.CodeBackground)
			}
			if st.CodeChipBorder != st.CodeBorder {
				t.Errorf("CodeChipBorder = %v, CodeBorder = %v; the code surface's edge is one value", st.CodeChipBorder, st.CodeBorder)
			}

			mark := themecolor.Flatten(p.TertiaryLabel, p.TextBackground)
			if st.QuoteBar != mark || st.CheckboxBorder != mark {
				t.Errorf("quote bar %v and open task box %v, want the weakest label over the page %v", st.QuoteBar, st.CheckboxBorder, mark)
			}
			if want := themecolor.Flatten(p.SecondaryLabel, p.TextBackground); st.QuoteColor != want {
				t.Errorf("QuoteColor = %v, want SecondaryLabel over the page %v", st.QuoteColor, want)
			}
			if want := themecolor.Flatten(p.Separator, p.TextBackground); st.RuleColor != want {
				t.Errorf("RuleColor = %v, want the separator over the page %v", st.RuleColor, want)
			}
			if st.TableBorder != p.Grid {
				t.Errorf("TableBorder = %v, want Grid %v", st.TableBorder, p.Grid)
			}
			if st.TableHeaderBackground != st.CodeBackground {
				t.Errorf("TableHeaderBackground = %v, want the one step a document takes off its page %v", st.TableHeaderBackground, st.CodeBackground)
			}
			if st.CheckboxFill != p.ControlAccent || st.CheckmarkColor != p.AlternateSelectedControlText {
				t.Errorf("a set task's box = %v under %v, want ControlAccent under AlternateSelectedControlText", st.CheckboxFill, st.CheckmarkColor)
			}

			// The find marks are one measured colour at two strengths: the mark
			// the reader is on is the platform's own pixel, the rest the same
			// colour laid on less. An arrival is one mark and wears the strong end.
			if st.CurrentMatchFill != p.FindHighlight {
				t.Errorf("CurrentMatchFill = %v, want the platform's find highlight %v", st.CurrentMatchFill, p.FindHighlight)
			}
			if st.ArrivalFill != st.CurrentMatchFill {
				t.Errorf("ArrivalFill = %v, CurrentMatchFill = %v; one highlight, whatever brought the reader", st.ArrivalFill, st.CurrentMatchFill)
			}
			if st.MatchFill == st.CurrentMatchFill {
				t.Error("a match and the current match wear the same fill; the reader cannot tell which one they are on")
			}
			if st.MatchFill == st.ContentSurface {
				t.Errorf("a match wears the page's own fill %v, so nothing is marked", st.MatchFill)
			}
			// Nothing this constructor hands the rasterizer is translucent.
			for _, f := range []struct {
				name string
				c    color.NRGBA
			}{
				{"ContentSurface", st.ContentSurface}, {"CodeBackground", st.CodeBackground},
				{"CodeBorder", st.CodeBorder}, {"QuoteBar", st.QuoteBar}, {"QuoteColor", st.QuoteColor},
				{"RuleColor", st.RuleColor}, {"CheckboxBorder", st.CheckboxBorder},
				{"MatchFill", st.MatchFill}, {"CurrentMatchFill", st.CurrentMatchFill}, {"ArrivalFill", st.ArrivalFill},
			} {
				if f.c.A != 0xff {
					t.Errorf("%s = %v carries a coverage; every name is flattened onto the surface it lands on", f.name, f.c)
				}
			}

			if want := font.Typeface(typo.Code.Typeface); st.Mono != want {
				t.Errorf("Mono = %q, want the Code role's %q", st.Mono, want)
			}
			if want := unit.Sp(typo.Code.Size); st.CodeSize != want {
				t.Errorf("CodeSize = %v, want the Code role's %v", st.CodeSize, want)
			}
			// Heading space is derived, not left to the caller: wider than the
			// block gap above every level and tighter below it.
			for i := range st.HeadingSpaceAbove {
				if st.HeadingSpaceAbove[i] <= st.BlockGap || st.HeadingSpaceBelow[i] >= st.BlockGap {
					t.Errorf("level %d heading space = %v/%v above/below against a %v block gap; want more above and less below",
						i+1, st.HeadingSpaceAbove[i], st.HeadingSpaceBelow[i], st.BlockGap)
				}
			}
		})
	}
}
