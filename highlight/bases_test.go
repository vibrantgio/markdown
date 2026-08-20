package highlight

import (
	stdcolor "image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/styles"

	"github.com/vibrantgio/theme/tokens"
)

// A style is a small XML document. These are the smallest ones that still
// colour something: a ground the mode is read off, and three inks a test can
// tell apart from any embedded style's.
const (
	lanternXML = `<style name="lantern-day" counterpart="lantern-night">
  <entry type="Background" style="bg:#fdf6e3 #586e75"/>
  <entry type="Keyword" style="bold #d33682"/>
  <entry type="LiteralString" style="#2aa198"/>
  <entry type="Comment" style="italic #93a1a1"/>
</style>
`
	lanternNightXML = `<style name="lantern-night" counterpart="lantern-day">
  <entry type="Background" style="bg:#002b36 #93a1a1"/>
  <entry type="Keyword" style="bold #d33682"/>
  <entry type="LiteralString" style="#2aa198"/>
  <entry type="Comment" style="italic #586e75"/>
</style>
`
)

// folder writes files into a fresh temporary directory and returns it. The
// map is name to contents, so a case reads as the folder it is describing.
func folder(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	return dir
}

// forget drops names from the loaded set when the test ends. The set is
// process-wide, like chroma's own registry, so a test that leaves a style in
// it changes what every later test sees Bases return.
func forget(t *testing.T, names ...string) {
	t.Helper()
	t.Cleanup(func() {
		loadedMu.Lock()
		defer loadedMu.Unlock()
		for _, n := range names {
			delete(loaded, strings.ToLower(n))
		}
	})
}

// TestALoadedStyleIsABaseLikeAnyOther: a file dropped in the folder is
// choosable by name, derives like an embedded base, and says where it came
// from — which is the whole of what loading is for.
func TestALoadedStyleIsABaseLikeAnyOther(t *testing.T) {
	forget(t, "lantern-day")
	dir := folder(t, map[string]string{"lantern.xml": lanternXML})
	names, skipped := LoadDir(dir)
	if len(skipped) != 0 {
		t.Fatalf("a well-formed style was skipped: %v", skipped)
	}
	if !slices.Equal(names, []string{"lantern-day"}) {
		t.Fatalf("loaded %v, want the one style in the folder", names)
	}
	if !Known("lantern-day") {
		t.Error("the loaded style does not resolve as a base")
	}
	if !Loaded("lantern-day") {
		t.Error("the loaded style does not report itself as loaded")
	}
	if Loaded(DefaultBase) {
		t.Error("an embedded style reports itself as loaded from a folder")
	}
	if !slices.Contains(Bases(), "lantern-day") {
		t.Error("the loaded style is not among the names a base can be chosen by")
	}
	// The proof it is a base and not just a name: wearing it colours code,
	// and colours it differently from the default.
	const snippet = "// hello\nfunc greet() {}\n"
	mine := worn(t, "lantern-day", tokens.DefaultLight).Highlight("go", snippet)
	theirs := worn(t, DefaultBase, tokens.DefaultLight).Highlight("go", snippet)
	if len(mine) == 0 {
		t.Fatal("wearing the loaded style coloured nothing")
	}
	same := true
	for i := range mine {
		if i >= len(theirs) || mine[i].Color != theirs[i].Color {
			same = false
			break
		}
	}
	if same {
		t.Error("the loaded style coloured the snippet exactly as the default base did")
	}
}

// TestALoadedPairHasTwoSides: a style naming a counterpart that is also in the
// folder behaves the way an embedded pair does — the light member on a light
// theme, the dark one on a dark theme, from the one name. The ground on the
// fence is what says which member arrived, the two fixtures being fitted to
// paper and to slate.
func TestALoadedPairHasTwoSides(t *testing.T) {
	forget(t, "lantern-day", "lantern-night")
	dir := folder(t, map[string]string{"day.xml": lanternXML, "night.xml": lanternNightXML})
	if _, skipped := LoadDir(dir); len(skipped) != 0 {
		t.Fatalf("a well-formed pair was skipped: %v", skipped)
	}
	for _, tc := range []struct {
		name string
		tok  tokens.ColorTokens
		want stdcolor.NRGBA
	}{
		{"light", tokens.DefaultLight, stdcolor.NRGBA{R: 0xfd, G: 0xf6, B: 0xe3, A: 0xff}},
		{"dark", tokens.DefaultDark, stdcolor.NRGBA{R: 0x00, G: 0x2b, B: 0x36, A: 0xff}},
	} {
		if got := worn(t, "lantern-day", tc.tok).CodeBackground; got != tc.want {
			t.Errorf("%s theme put %v under the fence, want the pair's %s member's own ground %v",
				tc.name, got, tc.name, tc.want)
		}
	}
}

// TestOneBadFileDoesNotCostTheFolder: the file that will not parse is named,
// with its reason, and every other style in the folder still loads. A folder
// is a place a person edits by hand, so a half-typed file is the ordinary
// case and not the exceptional one.
func TestOneBadFileDoesNotCostTheFolder(t *testing.T) {
	forget(t, "lantern-day")
	dir := folder(t, map[string]string{
		"lantern.xml": lanternXML,
		"broken.xml":  "<style name=\"half-typed\"><entry type=\"Keyword\" sty",
		"notes.txt":   "this is not a style and must not be complained about",
	})
	names, skipped := LoadDir(dir)
	if !slices.Equal(names, []string{"lantern-day"}) {
		t.Errorf("loaded %v, want the one style that parses", names)
	}
	if len(skipped) != 1 {
		t.Fatalf("skipped %v, want exactly the broken file", skipped)
	}
	if skipped[0].File != "broken.xml" {
		t.Errorf("skipped %q, want broken.xml", skipped[0].File)
	}
	if skipped[0].Reason == "" {
		t.Error("the skipped file came back with no reason to show")
	}
	if !strings.Contains(skipped[0].String(), "broken.xml") {
		t.Errorf("the sentence to show is %q, which does not name the file", skipped[0].String())
	}
}

// TestAStyleWithNoNameIsSkipped: a style is chosen by its name, so a file
// that does not carry one has nothing to be chosen by.
func TestAStyleWithNoNameIsSkipped(t *testing.T) {
	dir := folder(t, map[string]string{"anon.xml": `<style><entry type="Keyword" style="#d33682"/></style>`})
	names, skipped := LoadDir(dir)
	if len(names) != 0 {
		t.Errorf("loaded %v from a style with no name", names)
	}
	if len(skipped) != 1 || skipped[0].File != "anon.xml" {
		t.Fatalf("skipped %v, want the unnamed file", skipped)
	}
}

// TestAFileMayNotShadowAnEmbeddedStyle: the embedded set is curated and its
// names mean what they have always meant. A file claiming one of them is
// skipped and says so, rather than quietly replacing a style somebody else
// chose by name.
func TestAFileMayNotShadowAnEmbeddedStyle(t *testing.T) {
	shadow := strings.Replace(lanternXML, `name="lantern-day"`, `name="`+DefaultBase+`"`, 1)
	dir := folder(t, map[string]string{"shadow.xml": shadow})
	names, skipped := LoadDir(dir)
	if len(names) != 0 {
		t.Errorf("loaded %v, want nothing — the name is an embedded style's", names)
	}
	if len(skipped) != 1 || !strings.Contains(skipped[0].Reason, DefaultBase) {
		t.Fatalf("skipped %v, want the shadowing file named with its reason", skipped)
	}
	if Loaded(DefaultBase) {
		t.Fatal("the embedded default was replaced by a file")
	}
}

// TestTwoFilesCannotClaimOneName: which of them wins would be an accident of
// how the folder happens to be ordered, so the second is skipped and named.
func TestTwoFilesCannotClaimOneName(t *testing.T) {
	forget(t, "lantern-day")
	dir := folder(t, map[string]string{"a-lantern.xml": lanternXML, "b-lantern.xml": lanternXML})
	names, skipped := LoadDir(dir)
	if !slices.Equal(names, []string{"lantern-day"}) {
		t.Errorf("loaded %v, want the name once", names)
	}
	if len(skipped) != 1 || skipped[0].File != "b-lantern.xml" {
		t.Fatalf("skipped %v, want the second file to claim the name", skipped)
	}
}

// TestNoFolderIsNotAProblem: a person who has added no styles has no folder,
// and that is the ordinary state rather than something to report.
func TestNoFolderIsNotAProblem(t *testing.T) {
	names, skipped := LoadDir(filepath.Join(t.TempDir(), "nothing-here"))
	if len(names) != 0 || len(skipped) != 0 {
		t.Errorf("a missing folder gave %v and %v, want nothing at all", names, skipped)
	}
}

// TestLoadingLeavesChromasRegistryAlone: the curated set is what this package
// promises not to touch, and a folder full of styles is exactly the pressure
// that promise exists to survive.
func TestLoadingLeavesChromasRegistryAlone(t *testing.T) {
	forget(t, "lantern-day")
	before := styles.Names()
	dir := folder(t, map[string]string{"lantern.xml": lanternXML})
	if _, skipped := LoadDir(dir); len(skipped) != 0 {
		t.Fatalf("a well-formed style was skipped: %v", skipped)
	}
	if after := styles.Names(); !slices.Equal(before, after) {
		t.Errorf("chroma's registry went from %d names to %d — loading put a style in it", len(before), len(after))
	}
	if _, in := styles.Registry["lantern-day"]; in {
		t.Error("the loaded style is in chroma's registry")
	}
}

// TestAnUnknownBaseFallsBackToTheDefault: a name kept by an older build, or
// one whose file has since left the folder, leaves the code coloured the way
// it is for somebody who never chose at all.
func TestAnUnknownBaseFallsBackToTheDefault(t *testing.T) {
	for _, name := range []string{"", "  ", "a-style-nobody-wrote"} {
		if got := BaseOrDefault(name); got != DefaultBase {
			t.Errorf("BaseOrDefault(%q) = %q, want the default %q", name, got, DefaultBase)
		}
		if Known(name) {
			t.Errorf("%q was reported as a base that resolves", name)
		}
	}
	if got := BaseOrDefault("github"); got != "github" {
		t.Errorf("BaseOrDefault(github) = %q, want it kept", got)
	}
}

// TestEveryEmbeddedStyleIsChoosable: the list a chooser is built from covers
// the whole embedded set, so a base browsed elsewhere can be found here.
func TestEveryEmbeddedStyleIsChoosable(t *testing.T) {
	names := Bases()
	for _, n := range styles.Names() {
		if !slices.Contains(names, n) {
			t.Errorf("the embedded style %q is not choosable", n)
		}
	}
	if !slices.IsSorted(names) {
		t.Error("the names came back unsorted — a chooser built from them would not stand still")
	}
}

// TestEveryBaseIsOfferedUnderOneAppearanceOrBoth: the split is a partition of
// the whole set — a base offered under neither is a style that has become
// unreachable, which is the one outcome a filter must not produce. The counts
// are logged rather than pinned: chroma's set grows, and a test that failed
// when it did would be reporting on chroma rather than on this.
func TestEveryBaseIsOfferedUnderOneAppearanceOrBoth(t *testing.T) {
	var light, dark, both int
	for _, n := range Bases() {
		l, d := BaseSuits(n, false), BaseSuits(n, true)
		switch {
		case l && d:
			both++
		case l:
			light++
		case d:
			dark++
		default:
			t.Errorf("the base %q is offered under neither appearance — nothing can reach it", n)
		}
	}
	t.Logf("%d bases: %d light, %d dark, %d fitted to no ground and offered under both",
		len(Bases()), light, dark, both)
	if light == 0 || dark == 0 {
		t.Errorf("the split came out %d light and %d dark — one half of the chooser would be empty", light, dark)
	}
}

// TestTheGroundDecidesTheAppearance, and not the name: a style whose title says
// one thing and whose background says the other is offered under what it draws
// on. Both fixtures are loaded from a folder, which is also the assertion that
// a style somebody wrote themselves is classified exactly like one that ships.
func TestTheGroundDecidesTheAppearance(t *testing.T) {
	const misnamed = `<style name="lantern-night-in-name-only">
  <entry type="Background" style="bg:#fdf6e3 #586e75"/>
  <entry type="Keyword" style="bold #d33682"/>
</style>
`
	const groundless = `<style name="lantern-nowhere">
  <entry type="Keyword" style="bold #d33682"/>
</style>
`
	dir := folder(t, map[string]string{
		"day.xml": lanternXML, "night.xml": lanternNightXML,
		"misnamed.xml": misnamed, "groundless.xml": groundless,
	})
	if _, skipped := LoadDir(dir); len(skipped) > 0 {
		t.Fatalf("the folder skipped %v", skipped)
	}
	forget(t, "lantern-day", "lantern-night", "lantern-night-in-name-only", "lantern-nowhere")
	for _, tc := range []struct {
		name        string
		light, dark bool
	}{
		{"lantern-day", true, false},
		{"lantern-night", false, true},
		{"lantern-night-in-name-only", true, false},
		{"lantern-nowhere", true, true},
	} {
		if got := BaseSuits(tc.name, false); got != tc.light {
			t.Errorf("BaseSuits(%q, light) = %v, want %v", tc.name, got, tc.light)
		}
		if got := BaseSuits(tc.name, true); got != tc.dark {
			t.Errorf("BaseSuits(%q, dark) = %v, want %v", tc.name, got, tc.dark)
		}
	}
	// A name nothing resolves is offered nowhere: a chooser asking about a base
	// that has left the folder must not be handed a row for it.
	if BaseSuits("a-style-nobody-wrote", false) || BaseSuits("a-style-nobody-wrote", true) {
		t.Error("a name that resolves to nothing was offered under an appearance")
	}
}

// TestTheDefaultPairSitsOnOppositeSides: the base the window opens on is light
// and the one it reaches in the dark is dark, so the two halves of the chooser
// each hold one member of the pair rather than both or neither.
func TestTheDefaultPairSitsOnOppositeSides(t *testing.T) {
	if !BaseSuits(DefaultBase, false) || BaseSuits(DefaultBase, true) {
		t.Errorf("the default base %q is not offered as a light base and only as one", DefaultBase)
	}
	if BaseSuits(DefaultDarkBase, false) || !BaseSuits(DefaultDarkBase, true) {
		t.Errorf("the default dark base %q is not offered as a dark base and only as one", DefaultDarkBase)
	}
	if got := DefaultBases(); got.Base(false) != DefaultBase || got.Base(true) != DefaultDarkBase {
		t.Errorf("the default pair reads %+v, want %q under the sun and %q under the moon", got, DefaultBase, DefaultDarkBase)
	}
}

// TestAKeptPairResolvesByMeasurement is the whole of what a reader does with a
// pair somebody kept: a member stands where it was fitted to stand, and falls
// back to that appearance's default everywhere else. The last two cases are the
// migration — a file naming one base with no appearance attached arrives with
// that name in both members, and comes out with the name on the half it was
// measured to belong on and the default on the other.
func TestAKeptPairResolvesByMeasurement(t *testing.T) {
	for _, tc := range []struct {
		name         string
		light, dark  string
		wantL, wantD string
	}{
		{"a pair chosen for each appearance", "github", "dracula", "github", "dracula"},
		{"nothing kept", "", "", DefaultBase, DefaultDarkBase},
		{"names this build cannot resolve", "a-style-nobody-wrote", "another", DefaultBase, DefaultDarkBase},
		{"members on the wrong halves", "dracula", "github", DefaultBase, DefaultDarkBase},
		{"one light base, no appearance attached", "github", "github", "github", DefaultDarkBase},
		{"one dark base, no appearance attached", "dracula", "dracula", DefaultBase, "dracula"},
		{"one base fitted to no ground at all", "pygments", "pygments", "pygments", "pygments"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := BasesOrDefault(tc.light, tc.dark)
			if got.Light != tc.wantL || got.Dark != tc.wantD {
				t.Errorf("BasesOrDefault(%q, %q) = %+v, want light %q and dark %q",
					tc.light, tc.dark, got, tc.wantL, tc.wantD)
			}
			// Whatever comes out is drawable under the appearance it came out
			// for, which is the property a chooser leans on: the applied base
			// is always on the list the scheme is showing.
			if !BaseSuits(got.Light, false) || !BaseSuits(got.Dark, true) {
				t.Errorf("%+v holds a member fitted to the other ground", got)
			}
		})
	}
}
