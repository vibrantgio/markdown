// The fence's own tests live inside the package: what they check is that the
// entries a base was written with arrive on screen unaltered, and the entries
// are chroma's, which stays unexported here. A black-box test could see the
// spans and never what they came from.

package highlight

import (
	"fmt"
	stdcolor "image/color"
	"math"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// contrastFloor is the yardstick the sweep reports against, and it is the
// package's own — the same number [BaseContrast] measures a base against, so
// the sweep and the summary cannot drift into two floors.
const contrastFloor = ContrastFloor

// specimen is the code every measurement here reads: enough kinds of run —
// comment, keyword, type, string, number, call — that a base which colours
// anything colours several of these.
const specimen = "// greet returns a greeting.\n" +
	"func greet(name string, times int) string {\n" +
	"\treturn fmt.Sprintf(\"hello, %s\", name)\n" +
	"}\n"

// schemes are the two appearances a fence has to hold up in.
func schemes() []struct {
	name string
	tok  tokens.PlatformColors
} {
	return []struct {
		name string
		tok  tokens.PlatformColors
	}{
		{"light", tokens.PlatformLight},
		{"dark", tokens.PlatformDark},
	}
}

// worn is a token-themed Style with the named base on its fences.
func worn(t *testing.T, base string, c tokens.PlatformColors) markdown.Style {
	t.Helper()
	st := markdown.FromTokens(c, tokens.DefaultTypography, stdcolor.NRGBA{})
	Wear(&st, base, c)
	return st
}

// member is the style an appearance actually draws from.
func member(t *testing.T, base string, dark bool) *chroma.Style {
	t.Helper()
	mode := chroma.Light
	if dark {
		mode = chroma.Dark
	}
	s, ok := forMode(base, mode)
	if !ok {
		t.Fatalf("%s does not resolve", base)
	}
	return s
}

// authored is a member's own background, or the zero colour where it names
// none.
func authored(s *chroma.Style) (stdcolor.NRGBA, bool) {
	bg := s.Get(chroma.Background).Background
	if !bg.IsSet() {
		return stdcolor.NRGBA{}, false
	}
	return fromChroma(bg), true
}

// TestTheFenceWearsTheBasesOwnBackgroundAndColors is what the whole file is
// about: the plate on screen is the artifact its author made. The background
// under the code is the background they fitted their colours against, byte
// for byte; the runs they left plain are set in their own body colour; and
// every coloured run carries a colour that is in their own palette and was
// not derived from it.
func TestTheFenceWearsTheBasesOwnBackgroundAndColors(t *testing.T) {
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			st := worn(t, DefaultBase, sc.tok)
			m := member(t, DefaultBase, sc.name == "dark")

			bg, ok := authored(m)
			if !ok {
				t.Fatalf("%s names no background; this case cannot show one is worn", m.Name)
			}
			if st.CodeBackground != bg {
				t.Errorf("the fence's background is %v, %s was drawn on %v", st.CodeBackground, m.Name, bg)
			}
			if plain := plainForeground(m); plain.IsSet() && st.CodeColor != fromChroma(plain) {
				t.Errorf("plain code is set in %v, %s sets its body in %v",
					st.CodeColor, m.Name, fromChroma(plain))
			}

			colors := palette(m)
			spans := st.Highlight("go", specimen)
			if len(spans) == 0 {
				t.Fatal("the fence coloured nothing")
			}
			coloured := 0
			for _, sp := range spans {
				if sp.Color.A == 0 {
					continue
				}
				coloured++
				if !slices.Contains(colors, sp.Color) {
					t.Errorf("a run is drawn in %v, which is not a colour %s holds", sp.Color, m.Name)
				}
			}
			if coloured == 0 {
				t.Fatal("no run carries a colour, so matching proves nothing")
			}
			t.Logf("%s: background %v, body %v, %d of %d runs in %s's own colours",
				m.Name, st.CodeBackground, st.CodeColor, coloured, len(spans), m.Name)
		})
	}
}

// TestNoColorIsAltered is the same claim made across the whole registry rather
// than on the default: whatever base is chosen, in whichever appearance, the
// colours reaching the renderer are colours its author wrote down. A ratio is
// never consulted, so a palette drawn faint stays faint and a palette drawn
// boldly stays bold.
func TestNoColorIsAltered(t *testing.T) {
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			checked, runs := 0, 0
			for _, name := range styles.Names() {
				st := worn(t, name, sc.tok)
				colors := palette(member(t, name, sc.name == "dark"))
				for _, sp := range st.Highlight("go", specimen) {
					if sp.Color.A == 0 {
						continue
					}
					runs++
					if !slices.Contains(colors, sp.Color) {
						t.Errorf("%s: a run is drawn in %v, which its author never wrote", name, sp.Color)
					}
				}
				checked++
			}
			t.Logf("%d bases, %d coloured runs, every one of them a colour off the base itself", checked, runs)
		})
	}
}

// TestTheBackgroundIsTheAuthorsOrTheChips sweeps the backgrounds: a base that
// names a background is drawn on it exactly, and one that names none — four
// of the embedded styles — is drawn on the fill an inline chip sits on, which
// is what a fence had before any base was chosen.
func TestTheBackgroundIsTheAuthorsOrTheChips(t *testing.T) {
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			var noBackground []string
			for _, name := range styles.Names() {
				st := worn(t, name, sc.tok)
				m := member(t, name, sc.name == "dark")
				bg, ok := authored(m)
				if !ok {
					noBackground = append(noBackground, name)
					chip := markdown.FromTokens(sc.tok, tokens.DefaultTypography, stdcolor.NRGBA{}).CodeChip
					if st.CodeBackground != chip {
						t.Errorf("%s names no background and is drawn on %v, want the chip's fill %v",
							name, st.CodeBackground, chip)
					}
					continue
				}
				if st.CodeBackground != bg {
					t.Errorf("%s was drawn on %v and its fence is filled with %v", name, bg, st.CodeBackground)
				}
			}
			t.Logf("%d bases fitted to no background, each on the chip's fill: %v", len(noBackground), noBackground)
		})
	}
}

// TestAFenceIsBoundedOnItsPage: a block has to look like a block, and the fill
// is not what says so. The fence's fill is a small step off the page whichever
// base it wears, so the theme edges its own fence and every dressed one takes
// the same edge — the platform's separator laid over the background it
// encloses. Every base, with no exceptions and no comparison.
func TestAFenceIsBoundedOnItsPage(t *testing.T) {
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			for _, name := range styles.Names() {
				st := worn(t, name, sc.tok)
				if st.CodeBorder.A == 0 {
					t.Errorf("%s is drawn on %v and takes no edge at all", name, st.CodeBackground)
					continue
				}
				seam := tokens.PlatformLight.Separator
				if color.BestOn(st.CodeBackground, tokens.PlatformLight.Text, tokens.PlatformDark.Text) == tokens.PlatformDark.Text {
					seam = tokens.PlatformDark.Separator
				}
				seam.A = sc.tok.Separator.A
				if want := color.Flatten(seam, st.CodeBackground); st.CodeBorder != want {
					t.Errorf("%s: edge %v on the background %v, want the separator of the appearance whose text reads on that fill, %v",
						name, st.CodeBorder, st.CodeBackground, want)
				}
				if st.CodeBorder == st.CodeBackground {
					t.Errorf("%s: edge and background are both %v, so nothing says where the fence ends", name, st.CodeBackground)
				}
			}
			st := worn(t, DefaultBase, sc.tok)
			t.Logf("%s: %d bases, every one edged. The default's background %v stands |Lc| %.3f off the page and its edge %v measures |Lc| %.3f against that background",
				sc.name, len(styles.Names()), st.CodeBackground,
				color.Magnitude(st.CodeBackground, sc.tok.TextBackground), st.CodeBorder,
				color.Magnitude(st.CodeBorder, st.CodeBackground))
		})
	}
}

// TestTheEdgeFollowsTheBackground: what the edge answers is where the block
// ends, so the answer moves when the fill it is inset into moves and not when
// anything else does. The two extremes of the registry worn on one theme — the
// palest background and the deepest — take two different lines, because the
// separator carries a coverage and composites over whatever it lands on.
//
// The extremes are found rather than named, so the registry can gain and lose
// bases without this test going stale.
//
// The content surface is not read here at all, so a document inset into a panel
// takes the same edge it takes on the page.
func TestTheEdgeFollowsTheBackground(t *testing.T) {
	c := tokens.PlatformLight
	var pale, deep markdown.Style
	var paleName, deepName string
	lightest, darkest := -1.0, 999.0
	for _, name := range styles.Names() {
		st := worn(t, name, c)
		// Black's |Lc| against the fill ranks it by lightness without a
		// colour space of its own: the paler the fill, the further black
		// stands from it.
		l := color.Magnitude(stdcolor.NRGBA{A: 0xff}, st.CodeBackground)
		if l > lightest {
			lightest, pale, paleName = l, st, name
		}
		if l < darkest {
			darkest, deep, deepName = l, st, name
		}
	}
	t.Logf("palest %s on %v (black at |Lc| %.1f) edged %v; deepest %s on %v (black at |Lc| %.1f) edged %v",
		paleName, pale.CodeBackground, lightest, pale.CodeBorder,
		deepName, deep.CodeBackground, darkest, deep.CodeBorder)

	if pale.CodeBorder == deep.CodeBorder {
		t.Errorf("a fence on %v and one on %v take the same edge %v, so the edge is not following the background",
			pale.CodeBackground, deep.CodeBackground, pale.CodeBorder)
	}

	onPage := worn(t, DefaultBase, c)
	inset := markdown.FromTokens(c, tokens.DefaultTypography, stdcolor.NRGBA{})
	inset.ContentSurface = tokens.PlatformDark.TextBackground
	Wear(&inset, DefaultBase, c)
	if inset.CodeBorder != onPage.CodeBorder {
		t.Errorf("the same base inset onto a dark panel takes the edge %v where on the page it takes %v — the content surface is being read again",
			inset.CodeBorder, onPage.CodeBorder)
	}
}

// TestAStyleNamingNoSurfaceTakesTheSameEdge: a Style built by hand names no
// surface, and since the edge is derived against the fence's own background rather
// than against the page, that costs it nothing. A caller who never heard of
// the field sees exactly the fence a constructor-built Style sees.
func TestAStyleNamingNoSurfaceTakesTheSameEdge(t *testing.T) {
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			stated := worn(t, DefaultBase, sc.tok)

			silent := markdown.FromTokens(sc.tok, tokens.DefaultTypography, stdcolor.NRGBA{})
			silent.ContentSurface = stdcolor.NRGBA{}
			Wear(&silent, DefaultBase, sc.tok)

			if silent.CodeBorder != stated.CodeBorder {
				t.Errorf("a Style naming no surface takes the edge %v where one on the theme's page takes %v",
					silent.CodeBorder, stated.CodeBorder)
			}
		})
	}
}

// TestThreeFlavoursShowThreeBackgrounds is the case that says what a verbatim
// fence is worth. Three dark flavours of one family differ from each other
// mostly by the background they are drawn on: re-fitted onto one surface they
// came out very nearly the same plate three times, and worn they are three.
func TestThreeFlavoursShowThreeBackgrounds(t *testing.T) {
	flavours := []string{"catppuccin-frappe", "catppuccin-macchiato", "catppuccin-mocha"}
	seen := map[stdcolor.NRGBA]string{}
	for _, name := range flavours {
		st := worn(t, name, tokens.PlatformDark)
		if first, dup := seen[st.CodeBackground]; dup {
			t.Errorf("%s and %s are drawn on the same background %v", first, name, st.CodeBackground)
		}
		seen[st.CodeBackground] = name
		t.Logf("%-22s background %v, |Lc| %.3f off the page, edge %v",
			name, st.CodeBackground,
			color.Magnitude(st.CodeBackground, tokens.PlatformDark.TextBackground), st.CodeBorder)
	}
	if len(seen) != len(flavours) {
		t.Errorf("%d flavours came out on %d backgrounds", len(flavours), len(seen))
	}
}

// TestWearTakesThePairMemberForTheScheme asserts one base name reaches both
// members: the light tokens wear github, the dark ones github-dark, and naming
// either member gets the same pair. The background is what says which arrived.
func TestWearTakesThePairMemberForTheScheme(t *testing.T) {
	for _, pair := range []struct{ light, dark string }{
		{"catppuccin-latte", "catppuccin-mocha"},
		{"github", "github-dark"},
	} {
		lightBackground, _ := authored(member(t, pair.light, false))
		darkBackground, _ := authored(member(t, pair.dark, true))
		for _, base := range []string{pair.light, pair.dark} {
			if got := worn(t, base, tokens.PlatformLight).CodeBackground; got != lightBackground {
				t.Errorf("%s on light tokens drew on %v, want the light member's %v", base, got, lightBackground)
			}
			if got := worn(t, base, tokens.PlatformDark).CodeBackground; got != darkBackground {
				t.Errorf("%s on dark tokens drew on %v, want the dark member's %v", base, got, darkBackground)
			}
		}
	}
	if DefaultBase != "catppuccin-latte" {
		t.Errorf("the default base is %q; the pair checked above is no longer the default one", DefaultBase)
	}
	// A base with no registered counterpart is worn on both sides.
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
	l := worn(t, unpaired, tokens.PlatformLight)
	d := worn(t, unpaired, tokens.PlatformDark)
	if bg, ok := authored(member(t, unpaired, false)); ok && (l.CodeBackground != bg || d.CodeBackground != bg) {
		t.Errorf("unpaired base %q drew on %v under light and %v under dark, want its own %v",
			unpaired, l.CodeBackground, d.CodeBackground, bg)
	}
}

// TestAPairWearsTheAppearancesOwnMember: two names that are nothing to do with
// each other, and the appearance on screen decides which one is on the fence.
// This is what a base per appearance buys — a scheme change is a palette
// change — and it is asserted on two unrelated members precisely because
// chroma's counterpart rule could never have reached either from the other.
func TestAPairWearsTheAppearancesOwnMember(t *testing.T) {
	p := BasePair{Light: "solarized-light", Dark: "dracula"}
	for _, tc := range []struct {
		name   string
		tok    tokens.PlatformColors
		member string
	}{
		{"light", tokens.PlatformLight, p.Light},
		{"dark", tokens.PlatformDark, p.Dark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := markdown.FromTokens(tc.tok, tokens.DefaultTypography, stdcolor.NRGBA{})
			WearPair(&st, p, tc.tok)
			alone := worn(t, tc.member, tc.tok)
			if st.CodeBackground != alone.CodeBackground || st.CodeColor != alone.CodeColor {
				t.Errorf("the pair drew on %v in %v; %s alone gives %v in %v",
					st.CodeBackground, st.CodeColor, tc.member, alone.CodeBackground, alone.CodeColor)
			}
			got, want := st.Highlight("go", specimen), alone.Highlight("go", specimen)
			if len(got) == 0 || len(got) != len(want) {
				t.Fatalf("the fence split into %d runs, the member alone gives %d", len(got), len(want))
			}
			coloured := 0
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("run %d is %+v, the member alone gives %+v", i, got[i], want[i])
				}
				if got[i].Color.A != 0 {
					coloured++
				}
			}
			if coloured == 0 {
				t.Fatal("no run carries a colour, so matching proves nothing")
			}
		})
	}
}

// TestAWornMemberKeepsItsOwnEmphasis: the leaning and the weight are the
// author's too. github-dark is the fixture because it is where a pair's two
// members part company — it italicises comments and bolds four token
// categories where chroma's light github asks for neither — so a policy
// imposed from the other member would show here and nowhere else.
func TestAWornMemberKeepsItsOwnEmphasis(t *testing.T) {
	p := BasePair{Light: "solarized-light", Dark: "github-dark"}
	st := markdown.FromTokens(tokens.PlatformDark, tokens.DefaultTypography, stdcolor.NRGBA{})
	WearPair(&st, p, tokens.PlatformDark)

	got := st.Highlight("go", specimen)
	mine := worn(t, p.Dark, tokens.PlatformDark).Highlight("go", specimen)
	theirs := worn(t, p.Light, tokens.PlatformLight).Highlight("go", specimen)
	if len(got) != len(mine) || len(got) != len(theirs) {
		t.Fatalf("the specimen split into %d, %d and %d runs; the three have to line up to be compared",
			len(got), len(mine), len(theirs))
	}

	leaning, apart := 0, 0
	for i := range got {
		if got[i].Bold != mine[i].Bold || got[i].Italic != mine[i].Italic {
			t.Errorf("run %d reads bold=%t italic=%t; %s itself sets bold=%t italic=%t",
				i, got[i].Bold, got[i].Italic, p.Dark, mine[i].Bold, mine[i].Italic)
		}
		if got[i].Bold || got[i].Italic {
			leaning++
		}
		if got[i].Bold != theirs[i].Bold || got[i].Italic != theirs[i].Italic {
			apart++
		}
	}
	if leaning == 0 {
		t.Fatalf("%s emphasises nothing in the specimen — this fixture cannot show whose emphasis was used", p.Dark)
	}
	if apart == 0 {
		t.Fatalf("%s and %s emphasise the specimen identically — this fixture cannot tell the two apart", p.Dark, p.Light)
	}
	t.Logf("%d runs of the specimen lean or thicken; %d runs carry an emphasis %s would have set differently",
		leaning, apart, p.Light)
}

// TestAPairWithANameThisBuildLacks: a pair whose light member has left the
// styles folder still colours a dark window.
func TestAPairWithANameThisBuildLacks(t *testing.T) {
	p := BasePair{Light: "a-style-nobody-wrote", Dark: "dracula"}
	st := markdown.FromTokens(tokens.PlatformDark, tokens.DefaultTypography, stdcolor.NRGBA{})
	WearPair(&st, p, tokens.PlatformDark)
	if want, _ := authored(member(t, p.Dark, true)); st.CodeBackground != want {
		t.Errorf("the fence is filled with %v, want %s's own background %v", st.CodeBackground, p.Dark, want)
	}
}

// TestWearUnknownBasePanics asserts a typo fails where it is made rather than
// silently colouring with a style nobody asked for.
func TestWearUnknownBasePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("wearing an unknown base returned; want a panic naming it")
		}
		if msg := fmt.Sprint(r); !strings.Contains(msg, "no-such-style") {
			t.Errorf("panic message %q does not name the unknown base", msg)
		}
	}()
	st := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, stdcolor.NRGBA{})
	Wear(&st, "no-such-style", tokens.PlatformLight)
}

// TestWearLeavesTheRegistryAlone: the styles chroma ships are a curated set
// this package promises not to touch. Wearing one reads it and builds nothing,
// which is the easiest version of that promise to keep and still worth
// holding: an entry changed here would change it for every later reader in the
// process.
func TestWearLeavesTheRegistryAlone(t *testing.T) {
	before := map[chroma.TokenType]chroma.StyleEntry{}
	s := styles.Registry[DefaultBase]
	for _, tt := range s.Types() {
		before[tt] = s.Get(tt)
	}
	for _, sc := range schemes() {
		worn(t, DefaultBase, sc.tok)
	}
	for tt, want := range before {
		if got := s.Get(tt); got != want {
			t.Errorf("%s: the registry now says %+v, it said %+v", tt, got, want)
		}
	}
}

// TestWearTouchesOnlyTheCodeFields: the fence is dressed and the rest of the
// document is not, so a base reaches four fields and no others. The chip an
// inline code span sits on is the one this is most about — a page of prose
// spotted with somebody else's backgrounds is the thing not being built.
func TestWearTouchesOnlyTheCodeFields(t *testing.T) {
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			plain := markdown.FromTokens(sc.tok, tokens.DefaultTypography, stdcolor.NRGBA{})
			got := worn(t, DefaultBase, sc.tok)
			if got.CodeChip != plain.CodeChip {
				t.Errorf("the inline chip is filled with %v, the theme fills it with %v", got.CodeChip, plain.CodeChip)
			}
			if !reflect.DeepEqual(got.Text, plain.Text) {
				t.Error("the prose style moved when a base was worn")
			}
			if got.CodeBackground == plain.CodeBackground {
				t.Error("the fence's background did not move at all, so this proves nothing")
			}
			// Every remaining field, compared by putting the four that are
			// allowed to move back and dropping the highlighter, a func being
			// the one thing here that cannot be compared to anything.
			a, b := got, plain
			a.Highlight, b.Highlight = nil, nil
			a.CodeColor, a.CodeBackground, a.CodeBorder = b.CodeColor, b.CodeBackground, b.CodeBorder
			if !reflect.DeepEqual(a, b) {
				t.Errorf("a field outside the four moved:\n got %+v\nwant %+v", a, b)
			}
		})
	}
}

// TestAuthoredContrastSweep records what every base measures on the background
// its author drew it on, and names the worst of them. It fails nothing.
//
// The floor is a fact about a base and not a bar it has to clear. Contrast in
// content is surfaced rather than enforced — a style shows as its author drew
// it, and a reader who finds one unreadable picks another — so what a gate can
// honestly do here is keep the number where somebody looking for it will find
// it. A third of all authored colours across the embedded set sit under the
// normal-text floor, most of them on token types real code rarely reaches
// (diff markers, error highlights, whitespace), and the faintest of them are
// drawn deliberately: a marker for deleted text drawn in the background colour
// itself measures 1.00:1 and is meant to.
//
// What it would take to fail here is structural: a base that resolves to
// nothing, or an appearance that reaches no base at all.
func TestAuthoredContrastSweep(t *testing.T) {
	type reading struct {
		base, tt string
		r        float64
	}
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			chip := markdown.FromTokens(sc.tok, tokens.DefaultTypography, stdcolor.NRGBA{}).CodeChip
			var worst []reading
			var colourless []string
			entries, below := 0, 0
			for _, name := range styles.Names() {
				m := member(t, name, sc.name == "dark")
				background, ok := authored(m)
				if !ok {
					background = chip
				}
				plain := plainForeground(m)
				types := slices.Clone(m.Types())
				slices.Sort(types)
				w := reading{base: name, r: math.Inf(1)}
				for _, tt := range types {
					e := m.Get(tt)
					if !e.Colour.IsSet() || e.Colour == plain {
						continue
					}
					entries++
					r := color.Magnitude(fromChroma(e.Colour), background)
					if r < contrastFloor {
						below++
					}
					if r < w.r {
						w.r, w.tt = r, tt.String()
					}
				}
				if math.IsInf(w.r, 1) {
					colourless = append(colourless, name)
					continue
				}
				worst = append(worst, w)
			}
			if entries == 0 {
				t.Fatal("the sweep measured nothing")
			}
			sort.Slice(worst, func(i, j int) bool { return worst[i].r < worst[j].r })
			t.Logf("%d bases, %d authored colours, %d under |Lc| %.1f (%.0f%%), on the background each was drawn on",
				len(worst), entries, below, contrastFloor, 100*float64(below)/float64(entries))
			for _, w := range worst[:8] {
				t.Logf("  faintest colour: %-24s %-28s |Lc| %.2f", w.base, w.tt, w.r)
			}
			for _, w := range worst[len(worst)-3:] {
				t.Logf("  faintest colour: %-24s %-28s |Lc| %.2f  (the most pronounced of the faint)", w.base, w.tt, w.r)
			}
			if len(colourless) > 0 {
				t.Logf("bases colouring nothing at all: %v", colourless)
			}
		})
	}
}
