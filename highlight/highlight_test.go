package highlight_test

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/tokens"
)

const goSnippet = "func greet(name string) string {\n" +
	"    return fmt.Sprintf(\"hello, %s\", name)\n" +
	"}"

// TestHighlightSpans asserts the chroma hook splits Go code into runs that
// concatenate back to the input, with the func keyword coloured differently
// from plain punctuation.
func TestHighlightSpans(t *testing.T) {
	spans := highlight.New("github")("go", goSnippet)
	if len(spans) < 2 {
		t.Fatalf("highlighter returned %d spans, want several", len(spans))
	}

	var b strings.Builder
	colors := make(map[string]markdown.CodeSpan)
	for _, s := range spans {
		b.WriteString(s.Text)
		colors[s.Text] = s
	}
	if b.String() != goSnippet {
		t.Errorf("spans concatenate to %q, want the input code", b.String())
	}

	kw, ok := colors["func"]
	if !ok {
		t.Fatal("no span for the func keyword")
	}
	paren, ok := colors["("]
	if !ok {
		t.Fatal("no span for plain punctuation")
	}
	if kw.Color == paren.Color {
		t.Errorf("keyword and punctuation share colour %v; want distinct highlighting", kw.Color)
	}
}

// TestPlainRunsCarryNoColour asserts runs chroma would render in the style's
// plain-text foreground — punctuation, whitespace, plain identifiers — come
// back with the zero Color, the CodeSpan contract for "fall back to
// Style.CodeColor". This is the FX.7 fix: before it, every run carried an
// explicit colour and the token theme never reached highlighted code.
func TestPlainRunsCarryNoColour(t *testing.T) {
	for _, styleName := range []string{"github", "github-dark"} {
		t.Run(styleName, func(t *testing.T) {
			spans := highlight.New(styleName)("go", goSnippet)
			var zero color.NRGBA
			plain, coloured := 0, 0
			for _, s := range spans {
				if s.Color == zero {
					plain++
				} else {
					coloured++
				}
				switch s.Text {
				case "(", ")", "{", "}":
					if s.Color != zero {
						t.Errorf("punctuation %q carries colour %v; want the zero colour so Style.CodeColor fires", s.Text, s.Color)
					}
				case "func", "return":
					if s.Color == zero {
						t.Errorf("keyword %q lost its highlight colour", s.Text)
					}
				}
			}
			if plain == 0 || coloured == 0 {
				t.Errorf("snippet split into %d plain and %d coloured runs; want both", plain, coloured)
			}
		})
	}
}

// TestNewUnknownStylePanics asserts an unrecognised chroma style name fails
// at construction. Chroma's silent fallback is a dark-background style whose
// near-white runs disappear only against the light theme's code fill, so a
// typo must not survive New.
func TestNewUnknownStylePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("New(\"no-such-style\") returned; want a panic naming the style")
		}
		if msg := fmt.Sprint(r); !strings.Contains(msg, "no-such-style") {
			t.Errorf("panic message %q does not name the unknown style", msg)
		}
	}()
	highlight.New("no-such-style")
}

// TestHighlightUnknownLanguage asserts unmatched fence languages yield nil,
// leaving the block plain.
func TestHighlightUnknownLanguage(t *testing.T) {
	if spans := highlight.New("github")("no-such-language", "x y z"); spans != nil {
		t.Errorf("unknown language returned %#v, want nil", spans)
	}
}

// themedSnippetWidget renders the fenced Go snippet document on the given
// theme's background, applying restyle to the token-derived style first.
func themedSnippetWidget(t *testing.T, colors tokens.ColorTokens, restyle func(*markdown.Style)) layout.Widget {
	t.Helper()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	blocks := markdown.Parse([]byte("```go\n" + goSnippet + "\n```\n"))
	style := markdown.FromTokens(colors, tokens.DefaultTypography)
	if restyle != nil {
		restyle(&style)
	}
	d := markdown.NewDocument(blocks)
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, colors.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return d.Layout(gtx, shaper, style)
		})
	}
}

// snippetWidget renders the fenced Go snippet document on the light theme
// background, with or without the chroma hook.
func snippetWidget(t *testing.T, highlighted bool) layout.Widget {
	t.Helper()
	return themedSnippetWidget(t, tokens.DefaultLight, func(s *markdown.Style) {
		if highlighted {
			s.Highlight = highlight.New("github")
		}
	})
}

// TestGoSnippetGolden records or diffs a fenced Go snippet highlighted by
// the chroma hook, once per theme with the matching chroma style. In both
// images the plain runs — punctuation, whitespace, plain identifiers — render
// in the theme's Style.CodeColor (Neutral 700), not chroma's foreground;
// TestCodeColorReachesHighlightedBlock asserts that reach directly.
func TestGoSnippetGolden(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
		style  string
	}{
		{"go-snippet-light", tokens.DefaultLight, "github"},
		{"go-snippet-dark", tokens.DefaultDark, "github-dark"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			golden.Render(t, tc.name, image.Pt(560, 120), themedSnippetWidget(t, tc.colors, func(s *markdown.Style) {
				s.Highlight = highlight.New(tc.style)
			}))
		})
	}
}

// TestWornSnippetGolden records or diffs the same fenced snippet with the
// default base on it: the palette's own ground under the block, its own inks
// in the runs it colours, its own body colour in the runs it leaves plain, and
// the edge that keeps a ground this near the page a block. Beside the two
// images above — a stock style's inks on the theme's own fill — it is what
// wearing a base buys: a plate, rather than a set of hues borrowed from one.
func TestWornSnippetGolden(t *testing.T) {
	code := "// greet returns a greeting\n" + goSnippet
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"go-snippet-worn-light", tokens.DefaultLight},
		{"go-snippet-worn-dark", tokens.DefaultDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shaper := tokens.DefaultTypography.DeterministicShaper()
			blocks := markdown.Parse([]byte("```go\n" + code + "\n```\n"))
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography)
			highlight.Wear(&style, highlight.DefaultBase, tc.colors)
			d := markdown.NewDocument(blocks)
			golden.Render(t, tc.name, image.Pt(560, 140), func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, tc.colors.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return d.Layout(gtx, shaper, style)
				})
			})
		})
	}
}

// TestInlineChipsStayOnTheQuietFill is the other half of a worn fence: the
// document around it does not change. A chip is a word of code inside a
// sentence, and giving it a foreign ground would spot a page of prose with
// grounds that belong to a palette rather than to this theme — so the chip
// keeps the theme's fill and the body's own ink while the block down the page
// shows the base whole. Measured on a document holding both, by counting the
// pixels of each fill.
func TestInlineChipsStayOnTheQuietFill(t *testing.T) {
	const source = "A sentence with an `inline chip` in it.\n\n" +
		"```go\n" + goSnippet + "\n```\n"
	size := image.Pt(560, 160)
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shaper := tokens.DefaultTypography.DeterministicShaper()
			blocks := markdown.Parse([]byte(source))
			plain := markdown.FromTokens(tc.colors, tokens.DefaultTypography)
			style := plain
			highlight.Wear(&style, highlight.DefaultBase, tc.colors)
			if style.CodeChip != plain.CodeChip {
				t.Errorf("the chip's fill moved to %v; the theme fills it with %v", style.CodeChip, plain.CodeChip)
			}
			if style.CodeBackground == plain.CodeChip {
				t.Fatal("the fence's ground is the chip's fill, so counting the two apart proves nothing")
			}
			d := markdown.NewDocument(blocks)
			img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, tc.colors.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return d.Layout(gtx, shaper, style)
				})
			})
			if n := countPixels(img, plain.CodeChip); n == 0 {
				t.Errorf("nothing on the page is filled with the chip's %v", plain.CodeChip)
			}
			if n := countPixels(img, style.CodeBackground); n == 0 {
				t.Errorf("nothing on the page is filled with the base's ground %v", style.CodeBackground)
			}
		})
	}
}

// TestCodeColorReachesHighlightedBlock renders the highlighted snippet twice
// per theme — once with the token CodeColor, once with CodeColor overridden —
// and counts pixels of each colour exactly. The token colour has to be on
// screen in the first image and gone from the second, replaced by the
// override: only runs the highlighter leaves colourless can move that way, so
// this is the plain runs taking Style.CodeColor and nothing else. Before FX.7
// every run carried an explicit chroma colour, the Neutral 700 step appeared
// in neither image, and the two were identical.
func TestCodeColorReachesHighlightedBlock(t *testing.T) {
	size := image.Pt(560, 120)
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
		style  string
	}{
		{"light", tokens.DefaultLight, "github"},
		{"dark", tokens.DefaultDark, "github-dark"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := markdown.FromTokens(tc.colors, tokens.DefaultTypography).CodeColor
			override := tc.colors.Primary
			themed := golden.Capture(t, size, themedSnippetWidget(t, tc.colors, func(s *markdown.Style) {
				s.Highlight = highlight.New(tc.style)
			}))
			moved := golden.Capture(t, size, themedSnippetWidget(t, tc.colors, func(s *markdown.Style) {
				s.Highlight = highlight.New(tc.style)
				s.CodeColor = override
			}))
			if n := countPixels(themed, token); n == 0 {
				t.Errorf("nothing renders in the token CodeColor %v; the plain runs are not taking it", token)
			}
			if n := countPixels(moved, token); n != 0 {
				t.Errorf("%d pixel(s) still render in the token CodeColor %v after overriding it; those runs are not following Style.CodeColor", n, token)
			}
			if n := countPixels(moved, override); n == 0 {
				t.Errorf("nothing renders in the overridden CodeColor %v", override)
			}
			if n := golden.PixelDiff(themed, moved); n <= 0 {
				t.Errorf("moving Style.CodeColor changed %d pixels; want > 0 — the theme is not reaching the highlighted block", n)
			}
		})
	}
}

// countPixels counts the pixels rendering in exactly c. golden.Capture returns
// straight-alpha bytes, so a fully-covered glyph pixel holds the paint colour
// unchanged — anti-aliased edges hold a blend, which is why this counts exact
// matches rather than comparing whole images.
func countPixels(img *image.RGBA, c color.NRGBA) int {
	n := 0
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i] == c.R && img.Pix[i+1] == c.G && img.Pix[i+2] == c.B && img.Pix[i+3] == c.A {
			n++
		}
	}
	return n
}

// TestHighlightChangesPixels renders the snippet with and without the hook
// and asserts the highlighted output actually differs.
func TestHighlightChangesPixels(t *testing.T) {
	size := image.Pt(560, 120)
	plain := golden.Capture(t, size, snippetWidget(t, false))
	lit := golden.Capture(t, size, snippetWidget(t, true))
	if n := golden.PixelDiff(plain, lit); n <= 0 {
		t.Errorf("highlighted render differs from plain in %d pixels; want > 0", n)
	}
}

// shapeRun shapes one string through the default theme shaper in the given
// font and returns its total advance and glyph IDs. The shaper excludes
// system fonts, so glyphs coming back at all proves the collection resolved
// the request; a Gio GlyphID packs the face index the glyph resolved to, so
// identical strings shaped by different faces yield different ID sequences —
// face identity, not just metrics.
func shapeRun(t *testing.T, f font.Font) (fixed.Int26_6, []text.GlyphID) {
	t.Helper()
	shaper := tokens.DefaultTypography.DeterministicShaper()
	shaper.LayoutString(text.Parameters{
		Font:     f,
		PxPerEm:  fixed.I(16),
		MaxWidth: 100000,
	}, "wiiim... {mono[0] != prose}")
	var advance fixed.Int26_6
	var ids []text.GlyphID
	for g, ok := shaper.NextGlyph(); ok; g, ok = shaper.NextGlyph() {
		advance += g.Advance
		ids = append(ids, g.ID)
	}
	if len(ids) == 0 {
		t.Fatalf("font %+v: no glyphs shaped; the face did not resolve", f)
	}
	return advance, ids
}

// TestHighlightRunsShapeInMono asserts the bold and italic runs the chroma
// hook emits shape in the mono face rather than falling back to Roboto's
// weights. It highlights a snippet whose github-dark tokens carry a bold
// keyword and an italic comment, builds for every emitted flag combination
// exactly the font.Font the renderer builds (Style.Mono as the typeface, the
// span flags as weight and slant — markdown's codeSpans path), and shapes it
// through the default theme shaper: each combination must resolve (the
// shaper holds no system fonts), must measure differently from proportional
// Roboto at the same weight and slant (no silent fallback), and the
// combinations must resolve to pairwise-distinct faces (an italic mono keeps
// the upright's fixed pitch, so advances alone could not tell them apart).
func TestHighlightRunsShapeInMono(t *testing.T) {
	code := "// greet returns a greeting\n" + goSnippet
	spans := highlight.New("github-dark")("go", code)
	if spans == nil {
		t.Fatal("highlighter returned nil for Go code")
	}
	style := markdown.FromTokens(tokens.DefaultDark, tokens.DefaultTypography)

	combos := map[string]font.Font{}
	var bold, italic int
	for _, sp := range spans {
		f := font.Font{Typeface: style.Mono}
		if sp.Bold {
			f.Weight = font.Bold
			bold++
		}
		if sp.Italic {
			f.Style = font.Italic
			italic++
		}
		combos[fmt.Sprintf("bold=%t italic=%t", sp.Bold, sp.Italic)] = f
	}
	if bold == 0 || italic == 0 {
		t.Fatalf("snippet emitted %d bold and %d italic runs; want both, or the test proves nothing", bold, italic)
	}

	ids := map[string][]text.GlyphID{}
	for name, f := range combos {
		monoAdvance, monoIDs := shapeRun(t, f)
		ids[name] = monoIDs

		// The same string in proportional Roboto at the same weight and
		// slant must measure differently: 'w', 'i', 'm', '.' collapse to one
		// width only under the mono face.
		robotoAdvance, _ := shapeRun(t, font.Font{Typeface: "Roboto", Style: f.Style, Weight: f.Weight})
		if monoAdvance == robotoAdvance {
			t.Errorf("%s: mono advance %v equals proportional Roboto's; %q likely fell back to Roboto",
				name, monoAdvance, f.Typeface)
		}
	}
	names := make([]string, 0, len(ids))
	for name := range ids {
		names = append(names, name)
	}
	for i, a := range names {
		for _, b := range names[i+1:] {
			if idsEqual(ids[a], ids[b]) {
				t.Errorf("%s and %s shaped to identical glyph IDs; the two requests collapsed onto one face", a, b)
			}
		}
	}
}

func idsEqual(a, b []text.GlyphID) bool {
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

// TestHighlightKeepsLayout asserts highlighting only recolours: the
// highlighted code block lays out at exactly the plain block's height, so
// chroma's newline-leading whitespace tokens do not skew line metrics.
func TestHighlightKeepsLayout(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	blocks := markdown.Parse([]byte("```go\n" + goSnippet + "\n```\n"))
	measure := func(hl markdown.Highlighter) int {
		style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
		style.Highlight = hl
		var ops op.Ops
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(560, 1000)},
			Ops:         &ops,
		}
		return markdown.NewDocument(blocks).Layout(gtx, shaper, style).Size.Y
	}
	plain := measure(nil)
	lit := measure(highlight.New("github"))
	if plain != lit {
		t.Errorf("highlighted height %d != plain height %d; highlighting must not change layout", lit, plain)
	}
}
