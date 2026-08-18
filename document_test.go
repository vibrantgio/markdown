package markdown_test

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/font"
	gioinput "gioui.org/io/input"
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

// TestFromTokensDefaults pins the FromTokens contract: heading levels take
// the typography's document heading scale, code shapes in the theme Code
// role's typeface and
// size on the Neutral 300 tinted fill with Neutral 700 low-contrast text,
// the quote bar is Primary with Neutral 700 text, and rules are separators
// using Divider.
func TestFromTokensDefaults(t *testing.T) {
	c, typo := tokens.DefaultLight, tokens.DefaultTypography
	st := markdown.FromTokens(c, typo)

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
	if st.CodeBackground != c.Ramps.Neutral.Step(300) || st.CodeColor != c.Ramps.Neutral.Step(700) {
		t.Errorf("code colours = %v/%v, want Neutral 300/700", st.CodeBackground, st.CodeColor)
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
