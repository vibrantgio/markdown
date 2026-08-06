// Package highlight provides a chroma-backed [markdown.Highlighter]. It is a
// separate package inside the markdown module so the chroma dependency stays
// out of the core package's graph: only importers of this package pay for it.
//
// Setting one on [markdown.Style].Highlight keeps a code block inside the
// token theme: runs the chroma style would render in its plain-text
// foreground — whitespace, punctuation, plain identifiers — are emitted
// without a colour, so they fall back to [markdown.Style].CodeColor, and only
// runs the style genuinely colours (keywords, strings, comments) carry
// chroma's own colour. Pass the style name that matches the theme, github
// against a light one and github-dark against a dark one, and build a new
// Highlighter when the theme changes: [New] resolves the style once and the
// returned func closes over it, so it cannot follow a theme observable.
//
// An unrecognised style name panics in [New]. Chroma's own fallback is a
// dark-background style whose runs come out near-white — on the light token
// theme that is near-white text on the light Neutral 300 code fill, so a typo
// in the style name would fail silently, and only on one of the two themes.
// Both known callers construct their highlighters in package-level var
// declarations, where the panic surfaces at process start on either theme.
package highlight

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/vibrantgio/markdown"
)

// New returns a Highlighter that colours code with the named chroma style
// (e.g. "github" on light themes, "github-dark" on dark ones). A name missing
// from chroma's style registry panics: construction is the only place the
// typo can fail on both themes at once (see the package comment). Runs the
// style would render in its plain-text foreground are emitted with the zero
// colour and take [markdown.Style].CodeColor instead, so plain code follows
// the token theme. The fence language is matched against chroma's lexer
// registry; an unrecognised language yields nil, rendering the block plain.
// Assign the result to [markdown.Style].Highlight.
func New(styleName string) markdown.Highlighter {
	style, ok := styles.Registry[strings.ToLower(styleName)]
	if !ok {
		panic(fmt.Sprintf("highlight: unknown chroma style %q (chroma's styles.Names lists the registry)", styleName))
	}
	// Get resolves unspecified token types to the style's plain-text
	// foreground by inheritance (Text, then Background), so a run whose
	// resolved colour equals it is one chroma had no opinion about — emit it
	// colourless and let Style.CodeColor theme it. A minority of styles
	// (github among them) declare no foreground at all and instead restate
	// their body colour per token type; for those the punctuation colour is
	// the de-facto body colour — punctuation is the least semantic ink a
	// style ever colours — so it stands in as the plain foreground.
	plain := style.Get(chroma.Text).Colour
	if !plain.IsSet() {
		plain = style.Get(chroma.Punctuation).Colour
	}
	return func(language, code string) []markdown.CodeSpan {
		lexer := lexers.Get(language)
		if lexer == nil {
			return nil
		}
		it, err := chroma.Coalesce(lexer).Tokenise(nil, code)
		if err != nil {
			return nil
		}
		var out []markdown.CodeSpan
		for _, tok := range it.Tokens() {
			if tok.Value == "" {
				continue
			}
			entry := style.Get(tok.Type)
			sp := markdown.CodeSpan{
				Text:   tok.Value,
				Bold:   entry.Bold == chroma.Yes,
				Italic: entry.Italic == chroma.Yes,
			}
			if entry.Colour.IsSet() && entry.Colour != plain {
				sp.Color = color.NRGBA{
					R: entry.Colour.Red(),
					G: entry.Colour.Green(),
					B: entry.Colour.Blue(),
					A: 0xFF,
				}
			}
			out = append(out, sp)
		}
		// Some lexers append a final newline the source never had; a trailing
		// "\n" is a hard break to richtext, so it would add a blank line.
		if len(out) > 0 && !strings.HasSuffix(code, "\n") {
			last := &out[len(out)-1]
			last.Text = strings.TrimSuffix(last.Text, "\n")
			if last.Text == "" {
				out = out[:len(out)-1]
			}
		}
		return out
	}
}
