// Package highlight provides a chroma-backed [markdown.Highlighter]. It is a
// separate package inside the markdown module so the chroma dependency stays
// out of the core package's graph: only importers of this package pay for it.
//
// The confinement is stricter than the package boundary alone: no chroma type
// appears in any signature this package exports. A style is named by string,
// colours arrive as theme tokens, and what comes back is a
// [markdown.Highlighter]. Chroma's major version is therefore a fact about
// this one package, and moving to a later one is a change here and nowhere
// else — no consumer's code mentions the dependency it would be migrating.
//
// Setting a Highlighter on [markdown.Style].Highlight keeps a code block
// inside the token theme: runs the chroma style would render in its plain-text
// foreground — whitespace, punctuation, plain identifiers — are emitted
// without a colour, so they fall back to [markdown.Style].CodeColor, and only
// runs the style genuinely colours (keywords, strings, comments) carry a
// colour of their own.
//
// There are two constructors, and they differ in whether the style is worn or
// derived. [New] wears a stock style verbatim: pass the name that matches the
// theme, github against a light one and github-dark against a dark one. [Adapt]
// derives a new style from a named base and fits it to the theme's own
// colours — the base's hues and chromas held, its lightness re-fitted against
// the actual code surface until every ink clears a contrast floor. Stock
// styles stay curated artifacts either way: adaptation builds beside the
// registry and never mutates it.
//
// Adapt takes any name chroma's registry holds. The default, [DefaultBase], is
// catppuccin-latte, whose registered counterpart catppuccin-mocha is the dark
// member the same name reaches: a pair whose accents sit in one
// perceptual-lightness band with the hues carrying the semantics, which is the
// shape a lightness re-fit leaves intact.
//
// It also takes the name of a style read from a folder. A chroma style is a
// small XML document, and [LoadDir] reads a folder of them and makes each one
// choosable by its own name; [Bases] is the whole list, embedded and loaded
// together, and [Known] answers for one name. [BaseSuits] measures which
// appearance a base was fitted to, for a chooser that offers one half of the
// list at a time. Loaded styles are held beside chroma's registry and never
// inside it — see bases.go.
//
// Build a new Highlighter when the theme changes. Both constructors resolve
// their style once and the returned func closes over it, so neither can follow
// a theme observable.
//
// An unrecognised style name panics in both. Chroma's own fallback is a
// dark-background style whose runs come out near-white — on the light token
// theme that is near-white text on the light Neutral 300 code fill, so a typo
// in the style name would fail silently, and only on one of the two themes.
package highlight

import (
	"fmt"
	stdcolor "image/color"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/vibrantgio/markdown"
)

// New returns a Highlighter that colours code with the named chroma style
// (e.g. "github" on light themes, "github-dark" on dark ones), worn exactly as
// the style's author wrote it. A name that resolves to no style — neither an
// embedded one nor one [LoadDir] read — panics: construction is the only place
// the typo can fail on both themes at once (see the package comment). Runs the
// style would render in its plain-text
// foreground are emitted with the zero colour and take [markdown.Style].CodeColor
// instead, so plain code follows the token theme. The fence language is matched
// against chroma's lexer registry; an unrecognised language yields nil,
// rendering the block plain. Assign the result to [markdown.Style].Highlight.
//
// A stock style's inks were fitted to the background its author drew them on.
// Where that is not the background the theme puts under a fence, [Adapt] fits
// them to the one that is.
func New(styleName string) markdown.Highlighter {
	style, ok := lookup(styleName)
	if !ok {
		panic(fmt.Sprintf("highlight: unknown style %q (Bases lists every name that resolves)", styleName))
	}
	return spanner(style, plainForeground(style))
}

// plainForeground is the colour a style renders ordinary text in.
//
// Get resolves unspecified token types to the style's plain-text foreground by
// inheritance (Text, then Background), so a run whose resolved colour equals it
// is one chroma had no opinion about — emit it colourless and let
// Style.CodeColor theme it. A minority of styles (github among them) declare no
// foreground at all and instead restate their body colour per token type; for
// those the punctuation colour is the de-facto body colour — punctuation is the
// least semantic ink a style ever colours — so it stands in as the plain
// foreground.
func plainForeground(style *chroma.Style) chroma.Colour {
	plain := style.Get(chroma.Text).Colour
	if !plain.IsSet() {
		plain = style.Get(chroma.Punctuation).Colour
	}
	return plain
}

// spanner returns the Highlighter that tokenises through style, emitting the
// zero colour for every run resolving to plain.
func spanner(style *chroma.Style, plain chroma.Colour) markdown.Highlighter {
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
				sp.Color = stdcolor.NRGBA{
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
