// The adaptation's own tests live inside the package: the derivation returns a
// chroma style, and chroma types stay unexported here, so a black-box test
// could only ever see the spans and never the entries they came from. The
// contrast sweep needs the entries.

package highlight

import (
	"fmt"
	stdcolor "image/color"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// schemes are the two appearances every adapted style has to hold up in.
func schemes() []struct {
	name string
	tok  tokens.ColorTokens
} {
	return []struct {
		name string
		tok  tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	}
}

// stockGithub is the member of chroma's github pair a scheme derives from,
// unadapted — the before half of every before/after measurement here.
func stockGithub(scheme string) *chroma.Style {
	if scheme == "dark" {
		return styles.Registry["github-dark"]
	}
	return styles.Registry["github"]
}

// emitted walks the token entries a derived style can actually put on screen:
// those carrying a colour of their own, which is to say every entry except the
// ones resolving to the plain foreground — those come back colourless and take
// Style.CodeColor instead, and gating them here would be gating a colour this
// package never emits.
func emitted(style *chroma.Style) []chroma.TokenType {
	plain := plainForeground(style)
	types := style.Types()
	slices.Sort(types)
	var out []chroma.TokenType
	for _, tt := range types {
		e := style.Get(tt)
		if e.Colour.IsSet() && e.Colour != plain {
			out = append(out, tt)
		}
	}
	return out
}

// TestAdaptedContrastSweep is the gate: every one of chroma's embedded bases,
// derived against both schemes, must emit no token entry that fails the
// contrast floor on the code surface it was fitted to. A base is 74 styles'
// worth of other people's judgment about backgrounds that are not ours, so the
// only way "the contrast is enforced rather than hoped for" means anything is
// if a low-contrast entry is a test failure — for every base, not the two the
// apps happen to name.
func TestAdaptedContrastSweep(t *testing.T) {
	names := styles.Names()
	if len(names) < 50 {
		t.Fatalf("chroma's registry holds %d styles; the sweep is meant to cover all of them", len(names))
	}
	for _, sc := range schemes() {
		surface := codeSurface(sc.tok)
		t.Run(sc.name, func(t *testing.T) {
			worst, worstAt := math.Inf(1), ""
			for _, name := range names {
				style := derive(name, sc.tok, Options{})
				for _, tt := range emitted(style) {
					ink := fromChroma(style.Get(tt).Colour)
					r := color.ContrastRatio(ink, surface)
					if r < worst {
						worst, worstAt = r, name+" "+tt.String()
					}
					if r < contrastFloor {
						t.Errorf("%s adapted: %s renders %v on the %v code surface at %.2f:1, under the %.1f:1 floor",
							name, tt, ink, surface, r, contrastFloor)
					}
				}
			}
			t.Logf("%d bases swept on %v; worst emitted entry %.2f:1 (%s), floor %.1f:1",
				len(names), surface, worst, worstAt, contrastFloor)
		})
	}
}

// TestAdaptationLiftsWhatStockLeavesShort is the same measurement made twice —
// once on the stock base, once on what the derivation makes of it — so the
// difference is on the record rather than assumed. The stock github pair is
// where the defect was measured: on this theme's code fill its keyword red and
// its comment grey both read under the floor.
func TestAdaptationLiftsWhatStockLeavesShort(t *testing.T) {
	for _, sc := range schemes() {
		surface := codeSurface(sc.tok)
		t.Run(sc.name, func(t *testing.T) {
			stock := stockGithub(sc.name)
			adapted := derive("github", sc.tok, Options{})
			stockPlain := plainForeground(stock)

			short := 0
			for _, tt := range emitted(adapted) {
				before := stock.Get(tt).Colour
				if !before.IsSet() || before == stockPlain {
					continue
				}
				b := color.ContrastRatio(fromChroma(before), surface)
				a := color.ContrastRatio(fromChroma(adapted.Get(tt).Colour), surface)
				if b < contrastFloor {
					short++
					t.Logf("%-24s %v %.2f:1 -> %v %.2f:1", tt, fromChroma(before), b,
						fromChroma(adapted.Get(tt).Colour), a)
				}
				if a < b-0.01 {
					t.Errorf("%s: adaptation lowered contrast, %.2f:1 -> %.2f:1", tt, b, a)
				}
			}
			if short == 0 {
				t.Errorf("no stock github entry measured under the floor on %v; the defect this derivation answers is not reproduced", surface)
			}
			t.Logf("%d stock entries were under the %.1f:1 floor on %v; none are after adaptation", short, contrastFloor, surface)
		})
	}
}

// measurableChroma is the chroma below which an 8-bit colour's hue is not a
// fact about it. #57606A and #555D67 are both "a grey with a hint of blue" and
// their hues differ by three degrees; the quantisation, not the colour, is what
// moved. Hue assertions skip below this, chroma assertions do not.
const measurableChroma = 0.03

// TestAdaptationHoldsHueAndChroma asserts the re-fit moves lightness and
// nothing else: the hue and chroma of a corrected ink are the base's, which is
// the curated part the derivation is supposed to keep. Both survive an 8-bit
// round trip only approximately, hence the tolerances — a degree of hue and a
// hundredth of chroma are well under a just-noticeable difference, and a
// derivation that recoloured rather than re-fitted would miss by far more.
func TestAdaptationHoldsHueAndChroma(t *testing.T) {
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			stock := stockGithub(sc.name)
			adapted := derive("github", sc.tok, Options{})
			moved := 0
			for _, tt := range emitted(adapted) {
				before := stock.Get(tt).Colour
				after := adapted.Get(tt).Colour
				if !before.IsSet() || before == after {
					continue
				}
				moved++
				_, bc, bh := color.OKLChFromNRGBA(fromChroma(before))
				_, ac, ah := color.OKLChFromNRGBA(fromChroma(after))
				if d := math.Abs(math.Mod(ah-bh+540, 360) - 180); d > 1.0 && bc >= measurableChroma {
					t.Errorf("%s: hue moved %.2f° (%.1f -> %.1f); the re-fit holds hue", tt, d, bh, ah)
				}
				if d := math.Abs(ac - bc); d > 0.01 {
					t.Errorf("%s: chroma moved %.4f (%.3f -> %.3f); the re-fit holds chroma", tt, d, bc, ac)
				}
			}
			if moved == 0 {
				t.Error("no entry moved; the test measures nothing")
			}
		})
	}
}

// TestOneEmphasisPolicyAcrossThePair asserts the two schemes agree, entry for
// entry, on bold and italic. Stock github and github-dark do not: the dark one
// italicises comments and bolds operators, functions and classes where the
// light one asks for neither, so the same note changes its code's emphasis
// when the appearance changes. The derivation settles it on the light member.
func TestOneEmphasisPolicyAcrossThePair(t *testing.T) {
	light := derive("github", tokens.DefaultLight, Options{})
	dark := derive("github", tokens.DefaultDark, Options{})
	if light.Name == dark.Name {
		t.Fatalf("both schemes derived from %q; the pair's two members are not being reached", light.Name)
	}
	types := append(light.Types(), dark.Types()...)
	slices.Sort(types)
	types = slices.Compact(types)
	for _, tt := range types {
		l, d := light.Get(tt), dark.Get(tt)
		if l.Bold != d.Bold || l.Italic != d.Italic {
			t.Errorf("%s: light(bold=%v italic=%v) dark(bold=%v italic=%v); one policy means one answer",
				tt, l.Bold, l.Italic, d.Bold, d.Italic)
		}
	}

	// And the policy is the light member's, verbatim.
	stock := styles.Registry["github"]
	for _, tt := range stock.Types() {
		want, got := stock.Get(tt), light.Get(tt)
		if (want.Bold == chroma.Yes) != (got.Bold == chroma.Yes) ||
			(want.Italic == chroma.Yes) != (got.Italic == chroma.Yes) {
			t.Errorf("%s: policy reads bold=%v italic=%v, light base says bold=%v italic=%v",
				tt, got.Bold, got.Italic, want.Bold, want.Italic)
		}
	}
}

// TestPlainFallbackSurvivesAdaptation asserts the derived style still hands
// back colourless runs for the code the base had no opinion about, so
// Style.CodeColor keeps reaching a highlighted block. The re-fit is a function
// of an ink alone, so entries that shared the plain foreground before share it
// after — this pins that, because a derivation that treated the plain entry
// specially would silently colour every space and bracket in the block.
func TestPlainFallbackSurvivesAdaptation(t *testing.T) {
	const snippet = "func greet(name string) string {\n\treturn name\n}"
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			var zero stdcolor.NRGBA
			plain, coloured := 0, 0
			for _, sp := range Adapt("github", sc.tok)("go", snippet) {
				if sp.Color == zero {
					plain++
				} else {
					coloured++
				}
				switch sp.Text {
				case "(", ")", "{", "}":
					if sp.Color != zero {
						t.Errorf("punctuation %q carries %v; want the zero colour so Style.CodeColor fires", sp.Text, sp.Color)
					}
				case "func", "return":
					if sp.Color == zero {
						t.Errorf("keyword %q lost its colour", sp.Text)
					}
				}
			}
			if plain == 0 || coloured == 0 {
				t.Errorf("snippet split into %d plain and %d coloured runs; want both", plain, coloured)
			}
		})
	}
}

// TestAdaptLeavesTheRegistryAlone asserts adaptation is construction beside the
// stock styles rather than mutation of them. Stock styles are curated artifacts
// and stay selectable unchanged, so this snapshots every entry of every
// embedded style, derives from all of them in both schemes, and compares.
func TestAdaptLeavesTheRegistryAlone(t *testing.T) {
	snapshot := func() map[string]string {
		out := map[string]string{}
		for _, name := range styles.Names() {
			s := styles.Registry[name]
			types := s.Types()
			slices.Sort(types)
			var b strings.Builder
			fmt.Fprintf(&b, "%s|%s|", s.Name, s.Counterpart)
			for _, tt := range types {
				fmt.Fprintf(&b, "%s=%s;", tt, s.Get(tt))
			}
			out[name] = b.String()
		}
		return out
	}
	before := snapshot()
	for _, name := range styles.Names() {
		for _, sc := range schemes() {
			derive(name, sc.tok, Options{AlignToBrand: true})
		}
	}
	after := snapshot()
	if len(before) != len(after) {
		t.Fatalf("registry held %d styles before adaptation and %d after", len(before), len(after))
	}
	for name, was := range before {
		if now := after[name]; now != was {
			t.Errorf("stock style %q changed under adaptation", name)
		}
	}
}

// TestAlignToBrandTurnsThePaletteAsOne asserts the dial does what it claims:
// off, the hues are the base's; on, every chromatic entry turns by the same
// angle — the offsets between them preserved — and the palette's dominant hue
// lands on the theme's primary.
func TestAlignToBrandTurnsThePaletteAsOne(t *testing.T) {
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			plainStyle := derive("github", sc.tok, Options{})
			turned := derive("github", sc.tok, Options{AlignToBrand: true})

			base, ok := dominantHue(stockGithub(sc.name))
			if !ok {
				t.Fatal("the github base reports no dominant hue")
			}
			_, _, brand := color.OKLChFromNRGBA(sc.tok.Primary)
			want := math.Mod(brand-base+540, 360) - 180
			t.Logf("base dominant hue %.1f°, brand hue %.1f°, turn %+.1f°", base, brand, want)

			if math.Abs(want) < 1 {
				t.Skip("this base already sits on the brand hue; the dial has nothing to prove here")
			}
			turnedAny := false
			for _, tt := range emitted(turned) {
				off := plainStyle.Get(tt).Colour
				on := turned.Get(tt).Colour
				_, oc, oh := color.OKLChFromNRGBA(fromChroma(off))
				_, nc, nh := color.OKLChFromNRGBA(fromChroma(on))
				if oc < measurableChroma || nc < measurableChroma {
					continue
				}
				turnedAny = true
				got := math.Mod(nh-oh+540, 360) - 180
				if math.Abs(got-want) > 3 {
					t.Errorf("%s turned %+.1f°, the palette turned %+.1f°; the offsets between entries are not preserved", tt, got, want)
				}
			}
			if !turnedAny {
				t.Error("no entry carried enough chroma to measure a turn; the test proves nothing")
			}

			// The aggregate is checked as a direction, not a landing. sRGB's
			// gamut is not a cylinder: turning a saturated ink onto a hue the
			// display cannot hold that much chroma at costs it chroma, and
			// chroma is what weights the mean — so a palette whose entries
			// each turn by the delta has a dominant hue that turns by
			// somewhat less. Which way it moved is the claim the dial makes.
			offHue, _ := dominantHue(plainStyle)
			onHue, _ := dominantHue(turned)
			was := math.Abs(math.Mod(offHue-brand+540, 360) - 180)
			now := math.Abs(math.Mod(onHue-brand+540, 360) - 180)
			t.Logf("adapted palette's dominant hue %.1f° -> %.1f°, distance to the brand %.1f° -> %.1f°",
				offHue, onHue, was, now)
			if now > was {
				t.Errorf("aligning moved the palette's dominant hue from %.1f° off the brand to %.1f° off it", was, now)
			}
		})
	}
}

// TestAlignToBrandIsOffByDefault asserts the dial's default: Adapt leaves the
// base's hues where their author put them.
func TestAlignToBrandIsOffByDefault(t *testing.T) {
	byDefault := derive("github", tokens.DefaultLight, Options{})
	explicit := derive("github", tokens.DefaultLight, Options{AlignToBrand: false})
	stock := styles.Registry["github"]
	for _, tt := range emitted(byDefault) {
		if byDefault.Get(tt).Colour != explicit.Get(tt).Colour {
			t.Errorf("%s: the zero Options and an explicit off disagree", tt)
		}
		_, chr, was := color.OKLChFromNRGBA(fromChroma(stock.Get(tt).Colour))
		_, _, now := color.OKLChFromNRGBA(fromChroma(byDefault.Get(tt).Colour))
		if chr < measurableChroma {
			continue
		}
		if d := math.Abs(math.Mod(now-was+540, 360) - 180); d > 1 {
			t.Errorf("%s: hue turned %.1f° with the dial off", tt, d)
		}
	}
}

// TestAdaptHoldsUnderAKeptBrand runs the sweep's measurement against a theme
// that is not the default one: an adopted seed moves the accents and leaves
// the code slab where it is, so the fit has to hold for a palette the derived
// style was never seen against.
func TestAdaptHoldsUnderAKeptBrand(t *testing.T) {
	// A seed far from the default primary's violet, so an entry that happened
	// to clear the floor by sitting near the theme's own hue cannot hide.
	light, dark := tokens.FromSeed(stdcolor.NRGBA{R: 0x0E, G: 0x7C, B: 0x66, A: 0xFF})
	for _, sc := range []struct {
		name string
		tok  tokens.ColorTokens
	}{{"light", light}, {"dark", dark}} {
		t.Run(sc.name, func(t *testing.T) {
			surface := codeSurface(sc.tok)
			_, _, brand := color.OKLChFromNRGBA(sc.tok.Primary)
			for _, opt := range []Options{{}, {AlignToBrand: true}} {
				style := derive("github", sc.tok, opt)
				for _, tt := range emitted(style) {
					ink := fromChroma(style.Get(tt).Colour)
					if r := color.ContrastRatio(ink, surface); r < contrastFloor {
						t.Errorf("align=%v: %s renders %v at %.2f:1 on %v", opt.AlignToBrand, tt, ink, r, surface)
					}
				}
				h, _ := dominantHue(style)
				t.Logf("align=%v: brand hue %.1f°, adapted palette's dominant hue %.1f°, surface %v",
					opt.AlignToBrand, brand, h, surface)
			}
		})
	}
}

// TestAdaptUnknownStylePanics asserts a typo fails at construction, as it does
// in New: chroma's silent fallback is a dark style whose near-white runs
// vanish only on the light theme.
func TestAdaptUnknownStylePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Adapt(\"no-such-style\", …) returned; want a panic naming the style")
		}
		if msg := fmt.Sprint(r); !strings.Contains(msg, "no-such-style") {
			t.Errorf("panic message %q does not name the unknown style", msg)
		}
	}()
	Adapt("no-such-style", tokens.DefaultLight)
}

// TestAdaptPicksThePairMemberByScheme asserts one base name reaches both
// members: the light tokens derive from github, the dark ones from
// github-dark, and naming either member gets the same pair.
func TestAdaptPicksThePairMemberByScheme(t *testing.T) {
	for _, base := range []string{"github", "github-dark"} {
		if got := derive(base, tokens.DefaultLight, Options{}).Name; got != "github-adapted" {
			t.Errorf("%s on light tokens derived %q, want the light member", base, got)
		}
		if got := derive(base, tokens.DefaultDark, Options{}).Name; got != "github-dark-adapted" {
			t.Errorf("%s on dark tokens derived %q, want the dark member", base, got)
		}
	}
	// A base with no registered counterpart is derived from on both sides.
	unpaired := ""
	for _, name := range styles.Names() {
		if styles.Registry[name].Counterpart == "" {
			unpaired = name
			break
		}
	}
	if unpaired == "" {
		t.Skip("every embedded style is paired")
	}
	l := derive(unpaired, tokens.DefaultLight, Options{}).Name
	d := derive(unpaired, tokens.DefaultDark, Options{}).Name
	if l != d {
		t.Errorf("unpaired base %q derived %q on light and %q on dark", unpaired, l, d)
	}
}
