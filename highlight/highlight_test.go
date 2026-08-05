package highlight_test

import (
	"image"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

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
