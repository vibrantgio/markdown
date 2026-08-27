package markdown_test

import (
	"fmt"
	stdcolor "image/color"
	"testing"

	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"

	"github.com/vibrantgio/markdown"
)

func barHex(c stdcolor.NRGBA) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

// TestQuoteBarClearsTheGraphicFloor reads the blockquote bar against the page
// it is drawn on for the two seed shapes that matter: the canonical one every
// stored image in this design system is rendered from, and an accent stated at
// a dark scheme's tone — the shape that a palette published for dark mode
// hands out, and the shape whose light primary pin lands a whisper off the
// paper. The bar carries the whole of "these lines are quoted" without being
// text, so it owes the page WCAG 1.4.11's 3:1 whichever seed it came from.
//
// The population claim behind it is the palette's, not this package's: the
// gate that every brand ink clears its floor for every swept seed lives with
// the derivation, in theme/tokens. What is asserted here is that a document's
// bar is derived through that gate rather than naming a pin.
func TestQuoteBarClearsTheGraphicFloor(t *testing.T) {
	for _, s := range []struct {
		name string
		seed stdcolor.NRGBA
	}{
		{"the canonical seed", tokens.DefaultSeed},
		{"a pastel accent", stdcolor.NRGBA{0x89, 0xb4, 0xfa, 0xff}},
	} {
		light, dark := tokens.FromSeed(s.seed)
		for _, sc := range []struct {
			name string
			tok  tokens.ColorTokens
		}{{"light", light}, {"dark", dark}} {
			style := markdown.FromTokens(sc.tok, tokens.DefaultTypography)
			got := color.ContrastRatio(style.QuoteBar, style.Paper)
			if got < 3.0 {
				t.Errorf("%s, %s: quote bar %s on paper %s measures %.2f:1, under the 3:1 graphic floor",
					s.name, sc.name, barHex(style.QuoteBar), barHex(style.Paper), got)
			}
			t.Logf("%s, %s: quote bar %s on paper %s %.2f:1 (pin %s %.2f:1)",
				s.name, sc.name, barHex(style.QuoteBar), barHex(style.Paper), got,
				barHex(sc.tok.Primary), color.ContrastRatio(sc.tok.Primary, style.Paper))
		}
	}
}

// TestTheCanonicalSeedsQuoteBarIsThePrimaryPin states what deriving the bar
// costs the stored images, which is nothing: on the seed every golden is
// rendered from, the brand's own colour clears the floor and is what the bar
// is drawn in, exactly as when this named the pin outright.
func TestTheCanonicalSeedsQuoteBarIsThePrimaryPin(t *testing.T) {
	for _, s := range []struct {
		name string
		tok  tokens.ColorTokens
	}{
		{"DefaultLight", tokens.DefaultLight},
		{"DefaultDark", tokens.DefaultDark},
	} {
		style := markdown.FromTokens(s.tok, tokens.DefaultTypography)
		if style.QuoteBar != s.tok.Primary {
			t.Errorf("%s: quote bar is %s, not the Primary pin %s — a golden moved",
				s.name, barHex(style.QuoteBar), barHex(s.tok.Primary))
		}
	}
}

// TestAPastelSeedsQuoteBarLeavesThePin is the regression: on the shape that
// produced it, the bar is no longer the bare pin.
func TestAPastelSeedsQuoteBarLeavesThePin(t *testing.T) {
	light, _ := tokens.FromSeed(stdcolor.NRGBA{0x89, 0xb4, 0xfa, 0xff})
	style := markdown.FromTokens(light, tokens.DefaultTypography)
	if bare := color.ContrastRatio(light.Primary, style.Paper); bare >= 3.0 {
		t.Fatalf("this seed's bare light pin now measures %.2f:1 — the test no longer reads the shape it was written for", bare)
	}
	if style.QuoteBar == light.Primary {
		t.Errorf("light quote bar is still the bare pin %s", barHex(light.Primary))
	}
}
