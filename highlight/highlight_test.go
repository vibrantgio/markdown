package highlight_test

import (
	"fmt"
	"image"
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

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	golden "github.com/vibrantgio/markdown/internal/golden"
	"github.com/vibrantgio/spectrum/tokens"
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

// TestHighlightUnknownLanguage asserts unmatched fence languages yield nil,
// leaving the block plain.
func TestHighlightUnknownLanguage(t *testing.T) {
	if spans := highlight.New("github")("no-such-language", "x y z"); spans != nil {
		t.Errorf("unknown language returned %#v, want nil", spans)
	}
}

// snippetWidget renders the fenced Go snippet document on the light theme
// background, with or without the chroma hook.
func snippetWidget(t *testing.T, highlighted bool) layout.Widget {
	t.Helper()
	shaper := tokens.DefaultTypography.Shaper()
	blocks := markdown.Parse([]byte("```go\n" + goSnippet + "\n```\n"))
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypeScale)
	if highlighted {
		style.Highlight = highlight.New("github")
	}
	d := markdown.NewDocument(blocks)
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, tokens.DefaultLight.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return d.Layout(gtx, shaper, style)
		})
	}
}

// TestGoSnippetGolden records or diffs a fenced Go snippet highlighted by the
// chroma hook in the light theme.
func TestGoSnippetGolden(t *testing.T) {
	golden.Render(t, "go-snippet-light", image.Pt(560, 120), snippetWidget(t, true))
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
// face identity, not just metrics (the F0.1 technique).
func shapeRun(t *testing.T, f font.Font) (fixed.Int26_6, []text.GlyphID) {
	t.Helper()
	shaper := tokens.DefaultTypography.Shaper()
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
	style := markdown.FromTokens(tokens.DefaultDark, tokens.DefaultTypeScale)

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
	shaper := tokens.DefaultTypography.Shaper()
	blocks := markdown.Parse([]byte("```go\n" + goSnippet + "\n```\n"))
	measure := func(hl markdown.Highlighter) int {
		style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypeScale)
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
