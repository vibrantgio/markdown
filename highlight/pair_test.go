package highlight

import (
	stdcolor "image/color"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/styles"

	"github.com/vibrantgio/theme/imageseed"
)

// twinXML is a style whose keyword colour is worn by two token types and
// whose comment colour is its own body colour. It is the smallest fixture
// that can tell a palette reader from a list of entries: the first fact has
// to survive into the list as a repeat, the second has to be dropped.
const twinXML = `<style name="twin-day">
  <entry type="Background" style="bg:#ffffff #333333"/>
  <entry type="Keyword" style="#aa0000"/>
  <entry type="KeywordType" style="#aa0000"/>
  <entry type="LiteralString" style="#0000aa"/>
  <entry type="Comment" style="#333333"/>
</style>
`

func TestBasePaletteReadsInks(t *testing.T) {
	dir := folder(t, map[string]string{"twin.xml": twinXML})
	if _, skipped := LoadDir(dir); len(skipped) != 0 {
		t.Fatalf("loading the fixture: %v", skipped)
	}
	forget(t, "twin-day")

	got := BasePalette("twin-day")
	counts := map[stdcolor.NRGBA]int{}
	for _, c := range got {
		counts[c]++
	}
	var (
		red  = stdcolor.NRGBA{R: 0xaa, A: 0xff}
		blue = stdcolor.NRGBA{B: 0xaa, A: 0xff}
		body = stdcolor.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xff}
	)
	if len(got) != 3 {
		t.Fatalf("palette = %v, want three entries", got)
	}
	if counts[red] != 2 {
		t.Errorf("the keyword red appears %d times, want 2 — one per token type wearing it", counts[red])
	}
	if counts[blue] != 1 {
		t.Errorf("the string blue appears %d times, want 1", counts[blue])
	}
	if counts[body] != 0 {
		t.Errorf("the body colour is in the palette; it is the one colour this package never emits")
	}
}

func TestBasePaletteUnknown(t *testing.T) {
	if got := BasePalette("no such style"); got != nil {
		t.Errorf("BasePalette of an unknown name = %v, want nil", got)
	}
}

func TestBasePaletteStable(t *testing.T) {
	for _, name := range Bases() {
		first, second := BasePalette(name), BasePalette(name)
		if !slices.Equal(first, second) {
			t.Fatalf("%s: two reads disagree:\n %v\n %v", name, first, second)
		}
	}
}

func TestCompletePairDeclared(t *testing.T) {
	for _, tc := range []struct {
		name string
		want BasePair
	}{
		{"github", BasePair{Light: "github", Dark: "github-dark"}},
		{"github-dark", BasePair{Light: "github", Dark: "github-dark"}},
		{"catppuccin-latte", BasePair{Light: "catppuccin-latte", Dark: "catppuccin-mocha"}},
		{"catppuccin-mocha", BasePair{Light: "catppuccin-latte", Dark: "catppuccin-mocha"}},
		{"solarized-light", BasePair{Light: "solarized-light", Dark: "solarized-dark"}},
		{"xcode-dark", BasePair{Light: "xcode", Dark: "xcode-dark"}},
	} {
		if got := CompletePair(tc.name); got != tc.want {
			t.Errorf("CompletePair(%q) = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// TestCompletePairGroundless holds the rule for a base fitted to no ground: it
// is its own pair, because there is no appearance it is the wrong choice for
// and none it is the right one for either.
func TestCompletePairGroundless(t *testing.T) {
	var found []string
	for _, name := range Bases() {
		if !BaseSuits(name, true) || !BaseSuits(name, false) {
			continue
		}
		found = append(found, name)
		want := BasePair{Light: name, Dark: name}
		if got := CompletePair(name); got != want {
			t.Errorf("CompletePair(%q) = %+v, want %+v", name, got, want)
		}
	}
	if len(found) == 0 {
		t.Fatal("no groundless base in the embedded set; the rule is untested")
	}
	t.Logf("bases fitted to no ground: %v", found)
}

func TestCompletePairUnknown(t *testing.T) {
	if got := CompletePair("no such style"); got != DefaultBases() {
		t.Errorf("CompletePair of an unknown name = %+v, want %+v", got, DefaultBases())
	}
}

// TestCompletePairLoaded holds that a style read from a folder is completed
// like any other — by its own declaration where it makes one, and by
// measurement where it does not.
func TestCompletePairLoaded(t *testing.T) {
	dir := folder(t, map[string]string{
		"day.xml":   lanternXML,
		"night.xml": lanternNightXML,
		"twin.xml":  twinXML,
	})
	if _, skipped := LoadDir(dir); len(skipped) != 0 {
		t.Fatalf("loading the fixtures: %v", skipped)
	}
	forget(t, "lantern-day", "lantern-night", "twin-day")

	want := BasePair{Light: "lantern-day", Dark: "lantern-night"}
	if got := CompletePair("lantern-day"); got != want {
		t.Errorf("CompletePair(lantern-day) = %+v, want %+v", got, want)
	}
	if got := CompletePair("lantern-night"); got != want {
		t.Errorf("CompletePair(lantern-night) = %+v, want %+v", got, want)
	}

	// twin-day declares nothing, so its dark member is measured. What it
	// measures to is the metric's business; that it is a resolvable dark base
	// is the contract.
	got := CompletePair("twin-day")
	if got.Light != "twin-day" {
		t.Errorf("CompletePair(twin-day).Light = %q, want the style itself", got.Light)
	}
	if !Known(got.Dark) || !BaseSuits(got.Dark, true) {
		t.Errorf("CompletePair(twin-day).Dark = %q, which is not a dark base this build has", got.Dark)
	}
	t.Logf("twin-day completed to %+v", got)
}

// TestCompletePairDeterministic is the hard requirement stated on the
// function: two runs cannot disagree. The metric walks a sorted list and
// breaks ties by it, and the palettes it reads come out of sorted token type
// lists, so nothing here can be perturbed by a map.
func TestCompletePairDeterministic(t *testing.T) {
	for _, name := range Bases() {
		first := CompletePair(name)
		for i := 0; i < 4; i++ {
			if got := CompletePair(name); got != first {
				t.Fatalf("CompletePair(%q) = %+v on run %d, %+v on run 1", name, got, i+2, first)
			}
		}
	}
}

func TestBaseDistance(t *testing.T) {
	if d, ok := BaseDistance("github", "github"); !ok || d != 0 {
		t.Errorf("a style against itself = %v (ok=%v), want 0", d, ok)
	}
	if _, ok := BaseDistance("github", "no such style"); ok {
		t.Error("an unknown name compared as if it resolved")
	}
	forward, ok := BaseDistance("github", "monokai")
	back, ok2 := BaseDistance("monokai", "github")
	if !ok || !ok2 || forward != back {
		t.Errorf("the measure is not symmetric: %v vs %v", forward, back)
	}
	// bw colours nothing at all — every entry it has is the body colour, in
	// bold or italic — so there is no class to compare it on and it is
	// honestly incomparable rather than distance zero from everything.
	if _, ok := BaseDistance("bw", "github"); ok {
		t.Error("a style that colours nothing was compared anyway")
	}
}

// declaredPairs is every author-declared counterpart in the embedded set, as
// (style, its counterpart) with both names spelled the way [Bases] lists them.
func declaredPairs(t *testing.T) [][2]string {
	t.Helper()
	names := styles.Names()
	slices.Sort(names)
	var out [][2]string
	for _, name := range names {
		s := styles.Registry[name]
		if s.Counterpart == "" {
			continue
		}
		cp, ok := listed(s.Counterpart)
		if !ok {
			t.Fatalf("%s declares %q, which resolves to nothing", name, s.Counterpart)
		}
		out = append(out, [2]string{name, cp})
	}
	return out
}

// ranked is the measured ordering of every base suiting the given appearance,
// nearest first — the search [nearest] makes, opened up so a test can read
// where an answer came in rather than only whether it won. Nothing here reads
// a declaration, which is the point: this is the metric with the answers
// hidden from it.
func ranked(t *testing.T, name string, dark bool) []string {
	t.Helper()
	return rankedWith(t, name, dark, hueFamily)
}

// rankedWith is [ranked] at a stated hue-family width, so the width itself can
// be scored rather than assumed.
func rankedWith(t *testing.T, name string, dark bool, family float64) []string {
	t.Helper()
	s, ok := lookup(name)
	if !ok {
		t.Fatalf("%s resolves to nothing", name)
	}
	type row struct {
		name string
		at   float64
	}
	var rows []row
	for _, candidate := range Bases() {
		c, ok := lookup(candidate)
		if !ok || c == s {
			continue
		}
		if cDark, grounded := polarity(c); !grounded || cDark != dark {
			continue
		}
		if d, ok := distanceWith(s, c, family); ok {
			rows = append(rows, row{candidate, d})
		}
	}
	// Stable, so equally near candidates keep the order Bases listed them in
	// — the same tie-break the search itself makes.
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].at < rows[j].at })
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.name
	}
	return out
}

// family is the scheme a base belongs to: the part of its name before the
// first dash. Flavours of one scheme ship under one family name — the four
// catppuccins, the four tokyonights, the three rose-pines — and the
// rediscovery rule below needs to be able to say so.
func family(name string) string {
	if at := strings.IndexByte(name, '-'); at >= 0 {
		return name[:at]
	}
	return name
}

// TestRediscoversDeclaredPairs is what keeps the measured completion honest.
//
// Eleven of the embedded schemes ship as author-declared pairs. The metric
// never reads those declarations — it compares palettes — so running it over
// the declared halves is a test with an answer key that the thing being tested
// cannot see. If it cannot find pairs whose authors drew both halves for each
// other, it has no business proposing one for a scheme whose author drew only
// half.
//
// The acceptance rule is not "always first", and pretending otherwise would be
// dishonest about what was measured. Some schemes ship several flavours of one
// side — three dark catppuccins against one light one, two dark rose-pines
// against one light one — and those flavours differ mostly in their grounds,
// which is exactly the axis a palette comparison ignores. So a declared
// counterpart may be beaten, but only by a sibling of its own family, and it
// must still be inside the top three. Beaten by an unrelated scheme is a
// failure.
func TestRediscoversDeclaredPairs(t *testing.T) {
	const topN = 3
	for _, pair := range declaredPairs(t) {
		name, declared := pair[0], pair[1]
		dark, grounded := func() (bool, bool) { s, _ := lookup(name); return polarity(s) }()
		if !grounded {
			t.Fatalf("%s declares a counterpart but was fitted to no ground", name)
		}
		order := ranked(t, name, !dark)
		at := slices.Index(order, declared)
		if at < 0 {
			t.Errorf("%s: %s was not among the candidates at all", name, declared)
			continue
		}
		if at >= topN {
			t.Errorf("%s: the declared counterpart %s came %d of %d; the metric found %v",
				name, declared, at+1, len(order), order[:topN])
			continue
		}
		for _, ahead := range order[:at] {
			if family(ahead) != family(declared) {
				t.Errorf("%s: %s came ahead of the declared counterpart %s and is not of its family",
					name, ahead, declared)
			}
		}
		if at == 0 {
			t.Logf("%-22s -> %-22s (found first)", name, declared)
		} else {
			t.Logf("%-22s -> %-22s (found %d, behind %v)", name, declared, at+1, order[:at])
		}
	}
}

// TestHueFamilySitsInThePlateau is what makes the one chosen number in the
// measure a measurement.
//
// The family width could have been anything, so it is scored: for every width
// from a twentieth of the hue circle to a third of it, how many of the
// twenty-two declared counterparts does the masked metric find outright? The
// width the package
// ships has to be a best score, and it has to still be a best score twenty
// degrees narrower and twenty degrees wider — a value perched on a cliff edge
// would be fitted to the styles this build happens to embed rather than to
// anything about palettes.
func TestHueFamilySitsInThePlateau(t *testing.T) {
	found := func(family float64) int {
		hits := 0
		for _, pair := range declaredPairs(t) {
			s, _ := lookup(pair[0])
			dark, _ := polarity(s)
			order := rankedWith(t, pair[0], !dark, family)
			if len(order) > 0 && order[0] == pair[1] {
				hits++
			}
		}
		return hits
	}
	best, at := 0, 0.0
	for family := 18.0; family <= 120; family += 6 {
		if got := found(family); got > best {
			best, at = got, family
		}
	}
	mine := found(hueFamily)
	if mine < best {
		t.Errorf("the shipped family width of %d finds %d of the declared pairs; %.0f finds %d",
			hueFamily, mine, at, best)
	}
	for _, edge := range []float64{hueFamily - 20, hueFamily + 20} {
		if got := found(edge); got < best {
			t.Errorf("%d is on a cliff: %.0f degrees finds %d where the best is %d",
				hueFamily, edge, got, best)
		}
	}
	t.Logf("family width %d finds %d of %d declared counterparts outright",
		hueFamily, mine, len(declaredPairs(t)))
}

// TestRediscoveryMatchesTheSearch keeps the reading above honest about the
// code below it: the ordering the test builds has to start where the search
// the package actually makes ends up.
func TestRediscoveryMatchesTheSearch(t *testing.T) {
	for _, name := range Bases() {
		s, _ := lookup(name)
		dark, grounded := polarity(s)
		if !grounded {
			continue
		}
		order := ranked(t, name, !dark)
		got, ok := nearest(s, !dark)
		if len(order) == 0 {
			if ok {
				t.Errorf("%s: nothing was comparable, yet the search answered %q", name, got)
			}
			continue
		}
		if !ok || got != order[0] {
			t.Errorf("%s: the search answered %q, the ordering leads with %q", name, got, order[0])
		}
	}
}

// TestSweepEveryBase is the exit criterion: every base this build ships gets a
// completed pair and a leading seed candidate, and nothing about either is
// left to chance.
//
// It asserts the contract rather than the answers. Which light base a dark one
// without a declaration ends up beside is a measurement, and pinning
// seventy-odd of those would be pinning the metric's output rather than
// testing it — the declared pairs above are where the metric is held to an
// answer key. What is asserted here is that the answer exists, resolves, sits
// on the side it was asked for, and that the seed candidate is a colour the
// style genuinely draws with.
func TestSweepEveryBase(t *testing.T) {
	var inkless []string
	for _, name := range styles.Names() {
		pair := CompletePair(name)
		for _, member := range []struct {
			name string
			dark bool
		}{{pair.Light, false}, {pair.Dark, true}} {
			if !Known(member.name) {
				t.Errorf("%s: completed to %q, which resolves to nothing", name, member.name)
				continue
			}
			if !BaseSuits(member.name, member.dark) {
				t.Errorf("%s: completed to %q on the %s side, which it was not fitted to",
					name, member.name, appearance(member.dark))
			}
		}
		if dark, grounded := func() (bool, bool) { s, _ := lookup(name); return polarity(s) }(); grounded {
			if own := pair.Base(dark); own != name {
				t.Errorf("%s: its own side of the pair is %q", name, own)
			}
		}

		ink := BasePalette(name)
		candidates := imageseed.ExtractPalette(ink)
		if len(ink) == 0 {
			// A style that colours nothing has no seed in it. It is not a
			// failure, it is a style drawn in one colour, and the sweep says
			// which ones they are rather than pretending they extracted.
			inkless = append(inkless, name)
			if len(candidates) != 0 {
				t.Errorf("%s: colours nothing, yet %d candidates came out", name, len(candidates))
			}
			continue
		}
		if len(candidates) == 0 {
			t.Errorf("%s: %d inks and no candidate", name, len(ink))
			continue
		}
		if !slices.Contains(ink, candidates[0].Color) {
			t.Errorf("%s: the leading candidate %v is not a colour the style draws with",
				name, candidates[0].Color)
		}
		t.Logf("%-22s -> light %-22s dark %-22s  seed #%02x%02x%02x (chroma %.3f, share %.2f of %d inks)",
			name, pair.Light, pair.Dark,
			candidates[0].Color.R, candidates[0].Color.G, candidates[0].Color.B,
			candidates[0].Chroma, candidates[0].Share, len(ink))
	}
	t.Logf("bases that colour nothing, and so have no seed: %v", inkless)
}

func appearance(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}
