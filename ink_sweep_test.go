package markdown

// This file is an internal test (package markdown, not markdown_test) so it
// can exercise checkboxBorder, checkboxFill and checkmarkInk directly, the
// way theme/tokens/ink_test.go exercises ColorTokens.InkOn and
// patterns/tabs/ink_sweep_test.go exercises its underline ink. The Style
// fields these three feed are exported and could be read back through
// FromTokens, but the derivations are the seam the claims belong to: a test
// that went through the constructor would still be measuring these functions
// and would say so less plainly.

import (
	"fmt"
	stdcolor "image/color"
	"math/rand"
	"testing"

	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// checkboxSweepSeeds is the seed population this file reads the checkbox's
// ink claims against, the same one theme/tokens, components/richtext and the
// pattern strips sweep their derivations with: the default seed, the nine
// macOS system accents, both ends of the tonal axis, three pastels stated at
// a dark scheme's tone, and four hundred random colours from a fixed source.
//
// The three pastels are the shape that produced the defect this split
// repairs. A palette published for a dark scheme states its accents high on
// the tonal axis, and a brand seeded with one of them derives a light scheme
// whose primary pin sits a whisper off its own paper — which is exactly what
// an open task's box used to be outlined in.
func checkboxSweepSeeds() []stdcolor.NRGBA {
	rng := rand.New(rand.NewSource(20260827))
	seeds := []stdcolor.NRGBA{
		tokens.DefaultSeed,
		{0xff, 0x3b, 0x30, 0xff}, {0xff, 0x95, 0x00, 0xff}, {0xff, 0xcc, 0x00, 0xff},
		{0x28, 0xcd, 0x41, 0xff}, {0x00, 0x7a, 0xff, 0xff}, {0xaf, 0x52, 0xde, 0xff},
		{0xff, 0x2d, 0x55, 0xff}, {0x8e, 0x8e, 0x93, 0xff}, {0x00, 0x00, 0x00, 0xff},
		{0xff, 0xff, 0xff, 0xff},
		{0x89, 0xb4, 0xfa, 0xff}, {0xcb, 0xa6, 0xf7, 0xff}, {0xa6, 0xe3, 0xa1, 0xff},
	}
	for i := 0; i < 400; i++ {
		seeds = append(seeds, stdcolor.NRGBA{
			R: uint8(rng.Intn(256)), G: uint8(rng.Intn(256)), B: uint8(rng.Intn(256)), A: 0xff})
	}
	return seeds
}

func checkboxHex(c stdcolor.NRGBA) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

// checkboxSweepSchemes yields every palette the sweep reads a seed as: both
// derivations, both schemes.
func checkboxSweepSchemes(seed stdcolor.NRGBA) []struct {
	name  string
	tok   tokens.ColorTokens
	light bool
} {
	light, dark := tokens.FromSeed(seed)
	hcLight, hcDark := tokens.FromSeedHighContrast(seed)
	return []struct {
		name  string
		tok   tokens.ColorTokens
		light bool
	}{
		{"FromSeed light", light, true},
		{"FromSeed dark", dark, false},
		{"FromSeedHighContrast light", hcLight, true},
		{"FromSeedHighContrast dark", hcDark, false},
	}
}

// TestCheckboxBorderClearsTheGraphicFloorForEverySeed is this site's half of
// the palette's guarantee: whatever a caller seeds the brand with, an open
// task's box is visible on the page it is drawn on.
//
// The ground is the theme's own background, which is what FromTokens puts in
// Style.Paper in the same literal, and nothing in this package paints
// anything between the two: an unchecked box is a stroke and no fill, laid
// straight on the document's ground.
func TestCheckboxBorderClearsTheGraphicFloorForEverySeed(t *testing.T) {
	worstLight, worstDark := 99.0, 99.0
	var worstLightAt, worstDarkAt string
	seeds := checkboxSweepSeeds()
	for _, seed := range seeds {
		for _, s := range checkboxSweepSchemes(seed) {
			page := s.tok.SurfaceAt(tokens.Level0)
			border := checkboxBorder(s.tok)
			got := color.ContrastRatio(border, page)
			if got < tokens.GraphicFloor {
				t.Errorf("seed %s: %s: checkbox border %s on page %s measures %.2f:1, under the %.1f:1 graphic floor",
					checkboxHex(seed), s.name, checkboxHex(border), checkboxHex(page), got, tokens.GraphicFloor)
			}
			if s.light && got < worstLight {
				worstLight, worstLightAt = got, checkboxHex(seed)
			}
			if !s.light && got < worstDark {
				worstDark, worstDarkAt = got, checkboxHex(seed)
			}
		}
	}
	t.Logf("over %d seeds: worst light border %.2f:1 (%s), worst dark border %.2f:1 (%s)",
		len(seeds), worstLight, worstLightAt, worstDark, worstDarkAt)
}

// TestCheckmarkClearsItsFloorOnTheFillForEverySeed is the other half, and it
// is the pairing the split makes possible to get wrong. The tick's ground is
// the fill and not the page, so the two have to be read together; asserting
// them apart is what let one shared field look correct while the box it drew
// was invisible in one of its two states.
//
// The floor here is the graphic floor. A tick is a mark — a stroked path
// shaped like a gesture, carrying "done" with no words in it — and 1.4.11's
// 3:1 is what such a mark owes its ground; 1.4.3's 4.5:1 is what a run of
// words owes. The measured margin is far wider than the floor asked for,
// because while the fill is the brand's pin the tick is that pin's derived
// on-colour, which the palette holds to the text floor for every seed. The
// looser number is nonetheless the honest contract: it is what a future fill
// would have to clear, and pinning the test at 4.5 would forbid fills that
// are perfectly legal.
func TestCheckmarkClearsItsFloorOnTheFillForEverySeed(t *testing.T) {
	worst := 99.0
	var worstAt string
	seeds := checkboxSweepSeeds()
	for _, seed := range seeds {
		for _, s := range checkboxSweepSchemes(seed) {
			fill := checkboxFill(s.tok)
			tick := checkmarkInk(s.tok)
			got := color.ContrastRatio(tick, fill)
			if got < tokens.GraphicFloor {
				t.Errorf("seed %s: %s: check mark %s on fill %s measures %.2f:1, under the %.1f:1 graphic floor",
					checkboxHex(seed), s.name, checkboxHex(tick), checkboxHex(fill), got, tokens.GraphicFloor)
			}
			if got < worst {
				worst, worstAt = got, checkboxHex(seed)
			}
		}
	}
	t.Logf("over %d seeds: worst tick-on-fill %.2f:1 (%s), floor %.1f:1",
		len(seeds), worst, worstAt, tokens.GraphicFloor)
}

// TestTheCanonicalSeedKeepsEveryCheckboxPin states what this split costs
// every stored image in the design system, which is nothing. On the seed
// every golden is rendered from, the brand's own colour clears the page and
// is what the outline gets, the fill was never going to move, and the tick is
// the on-colour it always was — so all three fields hold the exact bytes the
// single shared field used to produce, and no task list anywhere renders a
// different pixel.
func TestTheCanonicalSeedKeepsEveryCheckboxPin(t *testing.T) {
	for _, s := range []struct {
		name string
		tok  tokens.ColorTokens
	}{
		{"DefaultLight", tokens.DefaultLight},
		{"DefaultDark", tokens.DefaultDark},
	} {
		if got := checkboxBorder(s.tok); got != s.tok.Primary {
			t.Errorf("%s: checkbox border is %s, not the Primary pin %s — a golden moved",
				s.name, checkboxHex(got), checkboxHex(s.tok.Primary))
		}
		if got := checkboxFill(s.tok); got != s.tok.Primary {
			t.Errorf("%s: checkbox fill is %s, not the Primary pin %s — a golden moved",
				s.name, checkboxHex(got), checkboxHex(s.tok.Primary))
		}
		if got := checkmarkInk(s.tok); got != s.tok.OnPrimary {
			t.Errorf("%s: check mark is %s, not the OnPrimary pin %s — a golden moved",
				s.name, checkboxHex(got), checkboxHex(s.tok.OnPrimary))
		}
	}
}

// TestAPastelSeedLeavesTheBorderPinAndKeepsTheFill is the regression itself,
// read on the shape that produced it: a light scheme seeded with a dark
// scheme's accent. Before the split this seed outlined an open task in the
// bare pin at a sub-floor ratio against its own paper — and it is the same
// assertion in reverse for the fill, which must NOT have followed the border
// off the pin, because a fill that walked would slide out from under a tick
// derived for the pin it left.
func TestAPastelSeedLeavesTheBorderPinAndKeepsTheFill(t *testing.T) {
	seed := stdcolor.NRGBA{0x89, 0xb4, 0xfa, 0xff}
	light, dark := tokens.FromSeed(seed)

	lightPage := light.SurfaceAt(tokens.Level0)
	if bare := color.ContrastRatio(light.Primary, lightPage); bare >= tokens.GraphicFloor {
		t.Fatalf("this seed's bare light pin now measures %.2f:1 on the page — the test no longer reads the shape it was written for", bare)
	}
	if border := checkboxBorder(light); border == light.Primary {
		t.Errorf("light checkbox border is still the bare pin %s", checkboxHex(light.Primary))
	}
	if fill := checkboxFill(light); fill != light.Primary {
		t.Errorf("light checkbox fill walked to %s; a fill keeps its brand and its tick is derived for the pin %s",
			checkboxHex(fill), checkboxHex(light.Primary))
	}

	if border := checkboxBorder(dark); border != dark.Primary {
		t.Errorf("dark checkbox border walked to %s; the dark pin %s clears its page and should stand",
			checkboxHex(border), checkboxHex(dark.Primary))
	}
}
