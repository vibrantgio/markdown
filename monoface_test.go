package markdown_test

import (
	"fmt"
	"testing"

	"gioui.org/font"
	"gioui.org/text"
	"golang.org/x/image/math/fixed"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/tokens"
)

// fencedJetBrainsSource is a fence whose highlighted runs cover every
// weight and slant the renderer will ask the shaper for: an italic
// comment and a bold keyword, plus the upright regular body between.
const fencedJetBrainsSource = "// greet returns a greeting\n" +
	"func greet(name string) string {\n" +
	"    return fmt.Sprintf(\"hello, %s\", name)\n" +
	"}"

// TestFencedBlockShapesInJetBrainsMono: a Style whose Code role names
// JetBrains Mono, with those faces in the collection, shapes a fenced
// block in that face at normal and bold, upright and italic — not
// Roboto Mono, not Roboto. Default-path goldens stay on Roboto Mono.
func TestFencedBlockShapesInJetBrainsMono(t *testing.T) {
	typ := tokens.CodeFace("JetBrains Mono")
	if typ.Code.Typeface != "JetBrains Mono" {
		t.Fatalf("Code.Typeface = %q, want JetBrains Mono", typ.Code.Typeface)
	}
	style := markdown.FromTokens(tokens.DefaultDark, typ)
	if style.Mono != "JetBrains Mono" {
		t.Fatalf("FromTokens set Mono %q, want JetBrains Mono", style.Mono)
	}

	spans := highlight.New("github-dark")("go", fencedJetBrainsSource)
	if spans == nil {
		t.Fatal("highlighter returned nil for Go code")
	}
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
		advance, glyphIDs := shapeThrough(t, typ, f)
		ids[name] = glyphIDs

		robotoMono, _ := shapeThrough(t, typ, font.Font{Typeface: "Roboto Mono", Style: f.Style, Weight: f.Weight})
		if advance == robotoMono {
			t.Errorf("%s: JetBrains advance %v equals Roboto Mono's; %q likely fell back",
				name, advance, f.Typeface)
		}
		roboto, _ := shapeThrough(t, typ, font.Font{Typeface: "Roboto", Style: f.Style, Weight: f.Weight})
		if advance == roboto {
			t.Errorf("%s: JetBrains advance %v equals Roboto's; %q likely fell back to proportional",
				name, advance, f.Typeface)
		}
	}
	names := make([]string, 0, len(ids))
	for name := range ids {
		names = append(names, name)
	}
	for i, a := range names {
		for _, b := range names[i+1:] {
			if glyphIDsEqual(ids[a], ids[b]) {
				t.Errorf("%s and %s shaped to identical glyph IDs; the two requests collapsed onto one face", a, b)
			}
		}
	}
}

// TestFromTokensDefaultMonoStaysRobotoMono pins the default path the
// stored goldens draw through. Choosing JetBrains Mono is a runtime
// fact; it must not move these.
func TestFromTokensDefaultMonoStaysRobotoMono(t *testing.T) {
	st := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	if st.Mono != "Roboto Mono" {
		t.Errorf("default Mono = %q, want Roboto Mono", st.Mono)
	}
}

func shapeThrough(t *testing.T, typ tokens.Typography, f font.Font) (fixed.Int26_6, []text.GlyphID) {
	t.Helper()
	shaper := typ.DeterministicShaper()
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

func glyphIDsEqual(a, b []text.GlyphID) bool {
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
