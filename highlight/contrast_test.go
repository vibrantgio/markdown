// The contrast summary's tests are in-package for the same reason the fence's
// are: what is being checked is which of a chroma style's entries were read and
// which were passed over, and chroma stays unexported here.

package highlight

import (
	"fmt"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/styles"

	"github.com/vibrantgio/theme/color"
)

// Palettes written on purpose, so the rule can be checked on cases whose answer
// is known before they are measured rather than on whichever style happens to
// sit near the boundary in this version of chroma's set.
//
// Every fixture is drawn on white, and its inks come off one axis: near-blacks
// around 15:1 and near-whites around 1.2:1. Nothing here turns on a hue, and the
// only thing separating one fixture from another is how many of its reading
// classes got which end.
//
// The inks inside each end differ from one another by a rounding, and they have
// to. An entry drawn in the style's own body colour is a class the style took no
// position on, and is passed over — so a fixture painting four classes in one
// literal colour would be a fixture with one reading in it.
var (
	loudInks  = []string{"#1a1a1a", "#203020", "#301a1a", "#1a1a30", "#2a1a2a"}
	faintInks = []string{"#e8e8e8", "#e4e9e4", "#eae4e4", "#e4e4ea", "#e9e9e2"}
)

// The classes the fixtures paint. They are chosen so that no two of them stand
// in each other's way: chroma resolves an entry a style leaves out by walking up
// to its parent, and a fixture setting Keyword would be measured on KeywordType
// as well, which is right on a real style and a puzzle in a fixture.
const (
	fBody    = `<entry type="Text" style="%s"/>`
	fComment = `<entry type="Comment" style="%s"/>`
	fString  = `<entry type="LiteralString" style="%s"/>`
	fNumber  = `<entry type="LiteralNumber" style="%s"/>`
	fFunc    = `<entry type="NameFunction" style="%s"/>`
)

// ink writes one entry in one colour.
func ink(entry, colour string) string { return fmt.Sprintf(entry, colour) }

// fixture loads a style whose entries are as named, on a white ground, through
// the same folder that a style somebody wrote themselves arrives by. It returns
// the name to ask about.
func fixture(t *testing.T, name string, entries ...string) string {
	t.Helper()
	return ground(t, name, `  <entry type="Background" style="bg:#ffffff"/>`, entries...)
}

// ground is [fixture] with the ground entry handed in, so a case can write a
// style that names none.
func ground(t *testing.T, name, bg string, entries ...string) string {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "<style name=%q>\n", name)
	if bg != "" {
		b.WriteString(bg + "\n")
	}
	for _, e := range entries {
		b.WriteString("  " + e + "\n")
	}
	b.WriteString("</style>\n")
	dir := folder(t, map[string]string{name + ".xml": b.String()})
	names, skipped := LoadDir(dir)
	if len(names) != 1 || len(skipped) != 0 {
		t.Fatalf("fixture %s loaded %v and skipped %v", name, names, skipped)
	}
	forget(t, name)
	return name
}

// TestAFaintPaletteIsNamedFaint: a style whose body ink and whose every reading
// class are drawn a shade off its own ground is the case the summary exists
// for, and it reads below the floor.
func TestAFaintPaletteIsNamedFaint(t *testing.T) {
	name := fixture(t, "fixture-faint",
		ink(fBody, faintInks[0]),
		ink(fComment, faintInks[1]),
		ink(fString, faintInks[2]),
		ink(fFunc, faintInks[3]),
		ink(fNumber, loudInks[0]),
	)
	a, ok := BaseContrast(name)
	if !ok {
		t.Fatal("a style with a ground and inks measured nothing")
	}
	if a.Inks != 5 || a.Below != 4 {
		t.Errorf("measured %d of %d inks under the floor, want 4 of 5", a.Below, a.Inks)
	}
	if !a.BelowFloor() {
		t.Error("a palette drawn four fifths faint does not read as faint")
	}
}

// TestOneFaintInkIsNotAFaintPalette is the rule's whole point. A palette that
// draws its code plainly and lets one class recede — which is what a comment
// colour is for — has not been drawn faint, and saying so about it would be
// saying it about nearly every style there is.
func TestOneFaintInkIsNotAFaintPalette(t *testing.T) {
	name := fixture(t, "fixture-one-quiet-ink",
		ink(fBody, loudInks[0]),
		ink(fString, loudInks[1]),
		ink(fFunc, loudInks[2]),
		ink(fNumber, loudInks[3]),
		ink(fComment, faintInks[0]),
	)
	a, ok := BaseContrast(name)
	if !ok {
		t.Fatal("a style with a ground and inks measured nothing")
	}
	if a.Below != 1 || a.Inks != 5 {
		t.Errorf("measured %d of %d inks under the floor, want 1 of 5", a.Below, a.Inks)
	}
	if a.BelowFloor() {
		t.Error("a palette with one receding class reads as faint, so the rule is the worst ink and not the majority")
	}
}

// TestAMarkerDrawnInTheGroundIsNotMeasured: the quietest entries in chroma's
// set are markers for things that are not code — a deleted line, an error span
// — and several of them are drawn in the ground colour itself, at one to one, on
// purpose. They are not runs anybody reads code in, and counting them would
// call a plainly-drawn palette faint on the strength of a marker its author
// meant nobody to see.
func TestAMarkerDrawnInTheGroundIsNotMeasured(t *testing.T) {
	body := []string{
		ink(fBody, loudInks[0]),
		ink(fString, loudInks[1]),
		ink(fComment, loudInks[2]),
	}
	markers := []string{
		`<entry type="GenericDeleted" style="#ffffff"/>`,
		`<entry type="GenericInserted" style="#ffffff"/>`,
		`<entry type="Error" style="#ffffff"/>`,
		`<entry type="TextWhitespace" style="#ffffff"/>`,
	}
	plain := fixture(t, "fixture-no-markers", body...)
	marked := fixture(t, "fixture-with-markers", append(append([]string{}, body...), markers...)...)
	a, _ := BaseContrast(plain)
	b, ok := BaseContrast(marked)
	if !ok {
		t.Fatal("a style with a ground and inks measured nothing")
	}
	if a != b {
		t.Errorf("four markers drawn in the ground changed the summary from %+v to %+v", a, b)
	}
	if b.BelowFloor() {
		t.Error("a plainly-drawn palette reads as faint once its markers are counted")
	}
}

// TestTheBodyInkIsOneOfTheReadings: most of the characters on screen are the
// ones the style took no position on, so the colour it sets them in is the ink
// a reader spends the most time on. A summary that only walked the classes
// would miss a style that draws its code faint and its keywords loud.
func TestTheBodyInkIsOneOfTheReadings(t *testing.T) {
	name := fixture(t, "fixture-faint-body",
		ink(fBody, faintInks[0]),
		ink(fString, loudInks[0]),
	)
	a, ok := BaseContrast(name)
	if !ok {
		t.Fatal("a style with a ground and inks measured nothing")
	}
	if a.Inks != 2 || a.Below != 1 {
		t.Errorf("measured %d of %d, want the body ink counted beside the one class", a.Below, a.Inks)
	}
}

// TestAnInkResolvingToTheBodyColourIsNotCountedTwice: a class whose entry is
// the style's own plain foreground is a class the style took no position on —
// the highlighter emits it colourless — so it is the body reading and not a
// second one.
func TestAnInkResolvingToTheBodyColourIsNotCountedTwice(t *testing.T) {
	name := fixture(t, "fixture-restated-body",
		ink(fBody, loudInks[0]),
		ink(fString, loudInks[0]),
		ink(fFunc, loudInks[0]),
		ink(fComment, faintInks[0]),
	)
	a, _ := BaseContrast(name)
	if a.Inks != 2 {
		t.Errorf("measured %d inks, want the body colour once and the one class that differs from it", a.Inks)
	}
}

// TestABaseFittedToNoGroundHasNoAuthoredContrast: the ratios are a fact about
// the pairing an author made, and an author who named no ground made no
// pairing. Measuring such a style against a surface somebody else chose would
// report a number about that surface.
func TestABaseFittedToNoGroundHasNoAuthoredContrast(t *testing.T) {
	inks := []string{ink(fBody, faintInks[0]), ink(fComment, faintInks[1])}
	groundless := ground(t, "fixture-groundless", "", inks...)
	control := fixture(t, "fixture-grounded", inks...)
	if _, ok := BaseContrast(groundless); ok {
		t.Error("a style fitted to no ground reports an authored contrast anyway")
	}
	a, ok := BaseContrast(control)
	if !ok || !a.BelowFloor() {
		t.Errorf("the control measured %+v ok=%v, so the case above proves nothing", a, ok)
	}
}

// TestABaseColouringNothingHasNothingToMeasure: one embedded style takes no
// position on any run at all. It has not drawn code faint; it has not drawn it.
func TestABaseColouringNothingHasNothingToMeasure(t *testing.T) {
	if _, ok := BaseContrast("bw"); ok {
		t.Error("a style that colours nothing reports an authored contrast")
	}
	if _, ok := BaseContrast("no-such-base"); ok {
		t.Error("a name that resolves to nothing reports an authored contrast")
	}
	var zero AuthoredContrast
	if zero.BelowFloor() {
		t.Error("nothing measured reads as a finding")
	}
}

// TestTheSummaryIsMeasuredOnTheAuthorsOwnGround checks the arithmetic against
// the ratios themselves on a style whose ground is nothing like the page: every
// ink it counted below the floor is one that measures below the floor there,
// and the count is the whole of what it walked.
func TestTheSummaryIsMeasuredOnTheAuthorsOwnGround(t *testing.T) {
	for _, name := range []string{"catppuccin-mocha", "solarized-light", "github"} {
		s, ok := lookup(name)
		if !ok {
			t.Fatalf("%s does not resolve", name)
		}
		ground, ok := authored(s)
		if !ok {
			t.Fatalf("%s names no ground", name)
		}
		plain := plainForeground(s)
		want := AuthoredContrast{}
		if plain.IsSet() {
			want.Inks++
			if color.ContrastRatio(fromChroma(plain), ground) < ContrastFloor {
				want.Below++
			}
		}
		for _, tt := range readingClasses {
			ink, ok := classInk(s, plain, tt)
			if !ok {
				continue
			}
			want.Inks++
			if color.ContrastRatio(ink, ground) < ContrastFloor {
				want.Below++
			}
		}
		got, _ := BaseContrast(name)
		if got != want {
			t.Errorf("%s summarised %+v, want %+v", name, got, want)
		}
	}
}

// TestTheShippedSetSplitsOnTheRule is the sanity check that the rule says
// something about real palettes: the styles with the plainest reputations for
// being drawn faint are named, the ones drawn plainly are not, and the set as a
// whole is neither all one nor all the other.
//
// The counts are logged rather than pinned. Which styles chroma ships is
// chroma's business, and a test asserting a total would fail on somebody else's
// upgrade for a reason that is not about this package.
func TestTheShippedSetSplitsOnTheRule(t *testing.T) {
	faint := map[string]bool{"solarized-light": true, "paraiso-light": true, "tokyonight-day": true}
	plain := map[string]bool{"github": true, "github-dark": true, "monokai": true, "catppuccin-mocha": true}
	named, measured := 0, 0
	for _, name := range styles.Names() {
		a, ok := BaseContrast(name)
		if !ok {
			continue
		}
		measured++
		if a.BelowFloor() {
			named++
		}
		if faint[name] && !a.BelowFloor() {
			t.Errorf("%s reads plain at %d of %d inks under the floor", name, a.Below, a.Inks)
		}
		if plain[name] && a.BelowFloor() {
			t.Errorf("%s reads faint at %d of %d inks under the floor", name, a.Below, a.Inks)
		}
	}
	if named == 0 || named == measured {
		t.Errorf("%d of %d measurable bases read faint, which is a rule that says nothing", named, measured)
	}
	t.Logf("%d of %d measurable bases read faint on their own grounds", named, measured)
}
