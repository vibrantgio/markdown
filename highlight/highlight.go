// Package highlight provides a chroma-backed [markdown.Highlighter]. It is a
// separate package inside the markdown module so the chroma dependency stays
// out of the core package's graph: only importers of this package pay for it.
package highlight

import (
	"image/color"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/vibrantgio/markdown"
)

// New returns a Highlighter that colours code with the named chroma style
// (e.g. "github" on light themes, "github-dark" on dark ones); unknown names
// fall back to chroma's default style. The fence language is matched against
// chroma's lexer registry; an unrecognised language yields nil, rendering the
// block plain. Assign the result to [markdown.Style].Highlight.
func New(styleName string) markdown.Highlighter {
	style := styles.Get(styleName)
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
			if entry.Colour.IsSet() {
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
