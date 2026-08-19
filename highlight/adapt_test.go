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

// bases are the pairs the before/after measurements run over: the default
// one, which is what ships, and github, which is where the defect was first
// measured. Two bases rather than one because the derivation's claims are
// about any base, and a pair whose members already agree on emphasis cannot
// on its own show that the policy is being imposed.
var bases = []string{DefaultBase, "github"}

// stock is the member of a pair a scheme derives from, unadapted — the before
// half of every before/after measurement here.
func stock(base, scheme string) *chroma.Style {
	mode := chroma.Light
	if scheme == "dark" {
		mode = chroma.Dark
	}
	return styles.GetForMode(base, mode)
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
// difference is on the record rather than assumed. Both bases fall short on
// this theme's code fill, and the default one falls furthest: it was fitted to
// a near-white page, and the fill a fence sits on here is three steps darker.
func TestAdaptationLiftsWhatStockLeavesShort(t *testing.T) {
	for _, base := range bases {
		for _, sc := range schemes() {
			surface := codeSurface(sc.tok)
			t.Run(base+"/"+sc.name, func(t *testing.T) {
				was := stock(base, sc.name)
				adapted := derive(base, sc.tok, Options{})
				stockPlain := plainForeground(was)

				short, worstBefore := 0, math.Inf(1)
				for _, tt := range emitted(adapted) {
					before := was.Get(tt).Colour
					if !before.IsSet() || before == stockPlain {
						continue
					}
					b := color.ContrastRatio(fromChroma(before), surface)
					a := color.ContrastRatio(fromChroma(adapted.Get(tt).Colour), surface)
					if b < contrastFloor {
						short++
						worstBefore = math.Min(worstBefore, b)
					}
					if a < b-0.01 {
						t.Errorf("%s: adaptation lowered contrast, %.2f:1 -> %.2f:1", tt, b, a)
					}
				}
				if short == 0 {
					t.Errorf("no stock %s entry measured under the floor on %v; the defect this derivation answers is not reproduced", base, surface)
				}
				t.Logf("%d stock entries were under the %.1f:1 floor on %v, the worst at %.2f:1; none are after adaptation",
					short, contrastFloor, surface, worstBefore)
			})
		}
	}
}

// measurableChroma is the chroma below which an 8-bit colour's hue is not a
// fact about it. #57606A and #555D67 are both "a grey with a hint of blue" and
// their hues differ by three degrees; the quantisation, not the colour, is what
// moved. Hue assertions skip below this, chroma assertions do not.
const measurableChroma = 0.03

// gamutSafeChroma is a chroma sRGB holds at any lightness the re-fit is
// likely to land on, whatever the hue. Below it a moved entry must come back
// at the chroma it went in with; above it the gamut has the last word.
const gamutSafeChroma = 0.09

// TestAdaptationHoldsHueAndChroma asserts the re-fit moves lightness and
// nothing else it has a choice about: the hue of a corrected ink is the
// base's, and so is its chroma wherever sRGB can still hold it.
//
// Where it cannot, chroma falls, and the test says so rather than pretending
// otherwise. A saturated orange at the lightness its author chose is a colour
// sRGB has; the same orange a third of a lightness darker is not, and the
// conversion answers by reducing chroma at constant lightness and constant
// hue. That is the gamut's doing, not the derivation's, and the invariant that
// survives it is one-directional: chroma may be given up, never added.
//
// The tolerances are the 8-bit round trip. Two degrees of hue and a hundredth
// of chroma sit well under a just-noticeable difference, and a derivation that
// recoloured rather than re-fitted would miss by tens of degrees.
func TestAdaptationHoldsHueAndChroma(t *testing.T) {
	for _, base := range bases {
		for _, sc := range schemes() {
			t.Run(base+"/"+sc.name, func(t *testing.T) {
				was := stock(base, sc.name)
				adapted := derive(base, sc.tok, Options{})
				moved, clipped := 0, 0
				for _, tt := range emitted(adapted) {
					before := was.Get(tt).Colour
					after := adapted.Get(tt).Colour
					if !before.IsSet() || before == after {
						continue
					}
					moved++
					_, bc, bh := color.OKLChFromNRGBA(fromChroma(before))
					_, ac, ah := color.OKLChFromNRGBA(fromChroma(after))
					if d := math.Abs(math.Mod(ah-bh+540, 360) - 180); d > 2.0 && bc >= measurableChroma {
						t.Errorf("%s: hue moved %.2f° (%.1f -> %.1f); the re-fit holds hue", tt, d, bh, ah)
					}
					if ac > bc+0.01 {
						t.Errorf("%s: chroma rose %.3f -> %.3f; the re-fit never adds chroma", tt, bc, ac)
					}
					if bc <= gamutSafeChroma && bc-ac > 0.01 {
						t.Errorf("%s: chroma fell %.3f -> %.3f at a chroma sRGB holds throughout; the re-fit holds chroma", tt, bc, ac)
					}
					if bc-ac > 0.01 {
						clipped++
					}
				}
				if moved == 0 {
					t.Error("no entry moved; the test measures nothing")
				}
				t.Logf("%d entries re-fitted on their own hue; %d gave up chroma sRGB could not hold at the new lightness", moved, clipped)
			})
		}
	}
}

// TestOneEmphasisPolicyAcrossThePair asserts the two schemes agree, entry for
// entry, on bold and italic, and that what they agree on is the light member's
// answer. Stock github and github-dark do not agree: the dark one italicises
// comments and bolds operators, functions and classes where the light one asks
// for neither, so the same note changes its code's emphasis when the
// appearance changes. The default pair already agrees, which is a reason to
// prefer it and not a reason to stop checking — the policy has to be imposed
// rather than inherited, or the next base chosen brings the split back.
func TestOneEmphasisPolicyAcrossThePair(t *testing.T) {
	for _, base := range bases {
		t.Run(base, func(t *testing.T) {
			light := derive(base, tokens.DefaultLight, Options{})
			dark := derive(base, tokens.DefaultDark, Options{})
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
			was := stock(base, "light")
			for _, tt := range was.Types() {
				want, got := was.Get(tt), light.Get(tt)
				if (want.Bold == chroma.Yes) != (got.Bold == chroma.Yes) ||
					(want.Italic == chroma.Yes) != (got.Italic == chroma.Yes) {
					t.Errorf("%s: policy reads bold=%v italic=%v, light base says bold=%v italic=%v",
						tt, got.Bold, got.Italic, want.Bold, want.Italic)
				}
			}
		})
	}
}

// TestDefaultPairComments asserts the concrete outcome the emphasis policy was
// imposed for: a comment is italic in both appearances under the default base,
// where the pair the highlighter shipped with before italicised one and not
// the other.
func TestDefaultPairComments(t *testing.T) {
	for _, sc := range schemes() {
		style := derive(DefaultBase, sc.tok, Options{})
		if e := style.Get(chroma.CommentSingle); e.Italic != chroma.Yes {
			t.Errorf("%s: a line comment reads italic=%v under %s", sc.name, e.Italic, style.Name)
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
	for _, base := range bases {
		for _, sc := range schemes() {
			t.Run(base+"/"+sc.name, func(t *testing.T) {
				var zero stdcolor.NRGBA
				plain, coloured := 0, 0
				for _, sp := range Adapt(base, sc.tok)("go", snippet) {
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
			plainStyle := derive(DefaultBase, sc.tok, Options{})
			turned := derive(DefaultBase, sc.tok, Options{AlignToBrand: true})

			base, ok := dominantHue(stock(DefaultBase, sc.name))
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
	byDefault := derive(DefaultBase, tokens.DefaultLight, Options{})
	explicit := derive(DefaultBase, tokens.DefaultLight, Options{AlignToBrand: false})
	was := stock(DefaultBase, "light")
	for _, tt := range emitted(byDefault) {
		if byDefault.Get(tt).Colour != explicit.Get(tt).Colour {
			t.Errorf("%s: the zero Options and an explicit off disagree", tt)
		}
		_, chr, before := color.OKLChFromNRGBA(fromChroma(was.Get(tt).Colour))
		_, _, now := color.OKLChFromNRGBA(fromChroma(byDefault.Get(tt).Colour))
		if chr < measurableChroma {
			continue
		}
		if d := math.Abs(math.Mod(now-before+540, 360) - 180); d > 1 {
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
				style := derive(DefaultBase, sc.tok, opt)
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
	for _, pair := range []struct{ light, dark string }{
		{"catppuccin-latte", "catppuccin-mocha"},
		{"github", "github-dark"},
	} {
		for _, base := range []string{pair.light, pair.dark} {
			if got := derive(base, tokens.DefaultLight, Options{}).Name; got != pair.light+"-adapted" {
				t.Errorf("%s on light tokens derived %q, want the light member", base, got)
			}
			if got := derive(base, tokens.DefaultDark, Options{}).Name; got != pair.dark+"-adapted" {
				t.Errorf("%s on dark tokens derived %q, want the dark member", base, got)
			}
		}
	}
	if DefaultBase != "catppuccin-latte" {
		t.Errorf("the default base is %q; the pair checked above is no longer the default one", DefaultBase)
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

// TestAPairDerivesThroughTheAppearancesOwnMember: two names that are nothing to
// do with each other, and the appearance on screen decides which one the code
// is coloured from. This is what a base per appearance buys — a scheme change
// is a palette change — and it is asserted on two unrelated members precisely
// because chroma's counterpart rule could never have reached either from the
// other.
func TestAPairDerivesThroughTheAppearancesOwnMember(t *testing.T) {
	p := BasePair{Light: "solarized-light", Dark: "dracula"}
	if got := derivePair(p, tokens.DefaultLight, Options{}).Name; got != p.Light+"-adapted" {
		t.Errorf("the light appearance derived %q, want the pair's light member", got)
	}
	if got := derivePair(p, tokens.DefaultDark, Options{}).Name; got != p.Dark+"-adapted" {
		t.Errorf("the dark appearance derived %q, want the pair's dark member", got)
	}
	// And the spans a fence comes back as follow the member, not the name that
	// was passed first.
	const src = "// greet.\nfunc greet(name string) string { return name }\n"
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
		member string
	}{
		{"light", tokens.DefaultLight, p.Light},
		{"dark", tokens.DefaultDark, p.Dark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := AdaptPair(p, tc.colors)("go", src)
			alone := Adapt(tc.member, tc.colors)("go", src)
			if len(got) == 0 || len(got) != len(alone) {
				t.Fatalf("the fence split into %d runs, the member alone gives %d", len(got), len(alone))
			}
			coloured := 0
			for i := range got {
				if got[i].Color != alone[i].Color {
					t.Fatalf("run %d is %v, the member alone gives %v", i, got[i].Color, alone[i].Color)
				}
				if got[i].Color.A != 0 {
					coloured++
				}
			}
			if coloured == 0 {
				t.Fatal("no run carries a colour, so matching proves nothing")
			}
			t.Logf("%d runs, %d coloured, ink for ink the %s member's", len(got), coloured, tc.name)
		})
	}
}

// TestAChosenMemberKeepsItsOwnEmphasis: where the two members were chosen,
// each is drawn as its author wrote it. The single-name rule above is for a
// counterpart nobody picked; a pair is two picks, and imposing one member's
// italics on the other would mean the appearance on screen depended on a
// choice made for the appearance that is not — a base picked under the sun
// quietly re-typesetting the moon's.
//
// github-dark is the fixture because it is where the two rules part company: it
// italicises comments and bolds four token categories where chroma's light
// github asks for neither.
func TestAChosenMemberKeepsItsOwnEmphasis(t *testing.T) {
	p := BasePair{Light: "solarized-light", Dark: "github-dark"}
	stockDark, ok := lookup(p.Dark)
	if !ok {
		t.Fatalf("the fixture pair's dark member %q does not resolve", p.Dark)
	}
	dark := derivePair(p, tokens.DefaultDark, Options{})
	types := slices.Clone(stockDark.Types())
	slices.Sort(types)
	emphasised := 0
	for _, tt := range types {
		want, got := stockDark.Get(tt), dark.Get(tt)
		if (want.Bold == chroma.Yes) != (got.Bold == chroma.Yes) ||
			(want.Italic == chroma.Yes) != (got.Italic == chroma.Yes) {
			t.Errorf("%s: the derived dark member reads bold=%v italic=%v, %s itself says bold=%v italic=%v",
				tt, got.Bold, got.Italic, p.Dark, want.Bold, want.Italic)
		}
		if got.Bold == chroma.Yes || got.Italic == chroma.Yes {
			emphasised++
		}
	}
	if emphasised == 0 {
		t.Fatalf("%s emphasises nothing at all — this fixture cannot show whose emphasis was used", p.Dark)
	}
	t.Logf("%d of %s's own entries keep the emphasis its author gave them", emphasised, p.Dark)
	// And the choice is still a choice: another light member picked beside it
	// moves not one of them.
	other := derivePair(BasePair{Light: "github", Dark: p.Dark}, tokens.DefaultDark, Options{})
	for _, tt := range types {
		if a, b := dark.Get(tt), other.Get(tt); a.Bold != b.Bold || a.Italic != b.Italic {
			t.Errorf("%s: changing the light member changed the dark member's emphasis", tt)
		}
	}
}

// TestTheDefaultPairAgreesWhicheverRuleApplies: the pair that ships is two
// names, so each member settles its own emphasis — and the concrete outcome
// the single-name policy was imposed for holds anyway, because catppuccin's two
// members already agree. A comment leans in both appearances, and the pair
// derives exactly what the one name does, entry for entry.
func TestTheDefaultPairAgreesWhicheverRuleApplies(t *testing.T) {
	p := DefaultBases()
	for _, sc := range schemes() {
		style := derivePair(p, sc.tok, Options{})
		if e := style.Get(chroma.CommentSingle); e.Italic != chroma.Yes {
			t.Errorf("%s: a line comment reads italic=%v under %s", sc.name, e.Italic, style.Name)
		}
		one := derive(DefaultBase, sc.tok, Options{})
		if one.Name != style.Name {
			t.Fatalf("%s: the pair derived %q and the single name %q", sc.name, style.Name, one.Name)
		}
		for _, tt := range style.Types() {
			if a, b := style.Get(tt), one.Get(tt); a != b {
				t.Errorf("%s/%s: the pair gives %+v and the single name %+v", sc.name, tt, a, b)
			}
		}
	}
}

// TestAPairWithANameThisBuildLacks: a pair whose light member has left the
// styles folder still colours a dark window. What it costs is the emphasis
// policy that member was carrying, not the fence.
func TestAPairWithANameThisBuildLacks(t *testing.T) {
	p := BasePair{Light: "a-style-nobody-wrote", Dark: "dracula"}
	got := derivePair(p, tokens.DefaultDark, Options{})
	if want := p.Dark + "-adapted"; got.Name != want {
		t.Errorf("derived %q, want %q", got.Name, want)
	}
}
