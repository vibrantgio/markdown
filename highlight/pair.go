// pair.go — the other half of a base, found rather than asked for.
//
// A person picking one syntax style has picked one side of a theme. The style
// was fitted to a ground — near-white paper or a near-black slab — and drawn
// on the other one it is either unreadable or simply not the thing its author
// made. So a single choice has to be completed into a pair before anything can
// be derived from it, and the question is where the other member comes from.
//
// A minority of styles answer it themselves: chroma records a counterpart on
// twenty-two of the seventy-four it ships, which is eleven author-declared
// pairs. Those are the best answer there is — the same author drew both halves
// for each other — and they win whenever they exist.
//
// The rest are on their own, and guessing from the name is not an option: most
// of the set says nothing about a partner, and the names that look like they
// pair — nord and nordic — are two dark styles rather than two halves of one
// scheme. What is left is the palettes themselves. Two halves of a pair are
// recognisably the same scheme — the same reds and greens and blues, re-fitted
// to the other ground — so the nearest opposite-polarity style by hue is a
// reasonable stand-in for a counterpart nobody declared.
//
// That claim is testable, and it is tested: the metric is run over both halves
// of every declared pair with the declarations hidden from it, and has to find
// them anyway. What it finds and where it is honestly beaten is in pair_test.go.

package highlight

import (
	stdcolor "image/color"
	"math"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/vibrantgio/theme/color"
)

// hueClasses are the token classes two bases are compared across: the kinds of
// run a syntax palette makes a decision about, in the order they are weighed
// (which is no order at all — the comparison is a sum).
//
// Comparing class against class is what makes the comparison mean anything. A
// palette is not a bag of colours, it is an assignment: this hue for keywords,
// that one for strings, a near-neutral for comments. Two styles whose bags of
// colours match while their assignments differ are two different schemes, and
// a metric that sorted both palettes and lined them up would call them the
// same. Token types make the classes reliable — a keyword is a keyword in
// every language chroma lexes — so the classes are read off the style rather
// than inferred from anything.
//
// The list is short deliberately. Every entry here is a class most styles
// actually set; adding rarely-set ones would add classes that only ever match
// by both being absent.
var hueClasses = []chroma.TokenType{
	chroma.Keyword,
	chroma.KeywordType,
	chroma.NameBuiltin,
	chroma.NameClass,
	chroma.NameFunction,
	chroma.NameVariable,
	chroma.LiteralString,
	chroma.LiteralNumber,
	chroma.Comment,
	chroma.Operator,
}

// classFloor is the weight a class carries before its chroma is counted. It is
// the same floor the seed ranking lifts its chroma emphasis off, and it is here
// for the same reason: a comment grey has a hue, and that hue is an artifact of
// rounding rather than a decision, so it must count for little. Little, though,
// and not nothing — a palette whose every ink is near-neutral would otherwise
// have no distance to anything at all, and would compare equal to everything.
const classFloor = 0.020

// hueFamily is the angle past which two inks are simply two different hues:
// a sixth of the circle, about the step between the landmarks a person names
// separately — red, yellow, green, cyan, blue, magenta. Inside it, two inks
// are one hue that two authors, or one author twice, expressed slightly
// differently. Outside it they are two hues, and how far outside is not a
// number about schemes.
//
// The cap it puts on a class's contribution is what makes the comparison a
// question about hue families rather than an average of angles, and the
// difference is not cosmetic. Without it, a base agreeing with another on
// eight classes and contradicting it outright on two loses to one that is
// vaguely off everywhere — two contradictions at 130 degrees drag a mean
// further than eight agreements at 5 pull it back. github and github-dark are
// exactly that shape: the same reds and purples and blues class for class,
// except that the light one draws variables brown and operators blue where the
// dark one draws both in its keyword red. Uncapped, the dark half of the most
// obviously declared pair in the set came third.
//
// The value is the middle of a plateau rather than a guess, and the plateau is
// asserted rather than remembered: a test scores the rediscovery below at
// widths from a twentieth of the circle to a third of it, and requires this
// one to score best with room to spare on either side. Narrower, and moderate
// disagreements start being called total ones; wider, and a single
// contradiction can swing an answer again.
//
// It is not the seed pipeline's own gathering width, which is narrower and
// answers a different question — whether two clusters of a photograph are one
// colour sampled at two depths, where two authors' choices are not involved.
const hueFamily = 60

// CompletePair returns the pair one chosen base stands for: the base itself on
// the side its author fitted it to, and on the other side the best answer
// available for what should be drawn there instead.
//
// That answer is looked for in two places, in order. A counterpart the style's
// own author declared wins outright, when the style declares one and it is
// resolvable and it really is fitted to the other side. Otherwise the pair is
// completed by measurement: of every base this build can resolve that suits the
// other side, the one whose inks fall nearest this one's, class by class, on
// the hue circle — see [BaseDistance].
//
// A base fitted to no ground at all is a pair by itself. It was drawn against
// nothing, so it is not the wrong choice under either appearance, and returning
// it for both sides is the honest reading of what its author left. Four of the
// embedded styles are like this.
//
// A name that resolves to nothing yields [DefaultBases], which is what a
// caller holding a name from a settings file written by an older build needs:
// the same fallback [BaseOrDefault] makes, for both members at once.
//
// The result is deterministic. Nothing here reads a map in map order, the
// candidates are weighed in the order [Bases] lists them, and a tie — two
// candidates equally near — goes to whichever of them that list holds first,
// which is alphabetical.
func CompletePair(name string) BasePair {
	s, ok := lookup(name)
	if !ok {
		return DefaultBases()
	}
	self, _ := listed(name)
	dark, grounded := polarity(s)
	if !grounded {
		return BasePair{Light: self, Dark: self}
	}
	other := counterpart(s, !dark)
	if dark {
		return BasePair{Light: other, Dark: self}
	}
	return BasePair{Light: self, Dark: other}
}

// listed is the spelling [Bases] uses for a name that resolves, which is the
// spelling every member of a completed pair comes back in: a chooser marking
// the row it is on compares strings, and a style whose author capitalised its
// name is registered under a lower-cased key that the list shows and the style
// itself does not carry.
func listed(name string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	loadedMu.RLock()
	s, ok := loaded[key]
	loadedMu.RUnlock()
	if ok {
		return s.Name, true
	}
	if _, ok := styles.Registry[key]; ok {
		return key, true
	}
	return "", false
}

// polarity measures which appearance a style was fitted to, and reports
// whether it was fitted to one at all. It is [BaseSuits]'s own measurement,
// reachable from a style rather than from a name.
func polarity(s *chroma.Style) (dark, grounded bool) {
	bg := s.Get(chroma.Background).Background
	if !bg.IsSet() {
		return false, false
	}
	return isDarkSurface(fromChroma(bg)), true
}

// counterpart is the member of s's pair that suits the given appearance: the
// declared one where there is one that fits, the nearest measured one
// otherwise, and the default for that appearance when there is no candidate to
// measure at all.
func counterpart(s *chroma.Style, dark bool) string {
	if cp, ok := lookup(s.Counterpart); ok {
		if cpDark, grounded := polarity(cp); grounded && cpDark == dark {
			declared, _ := listed(s.Counterpart)
			return declared
		}
	}
	if near, ok := nearest(s, dark); ok {
		return near
	}
	if dark {
		return DefaultDarkBase
	}
	return DefaultBase
}

// nearest is the base suiting the given appearance whose palette falls closest
// to s's, or false when nothing this build holds can be compared to s.
//
// The candidates are the grounded bases of the wanted appearance. A groundless
// base is not among them: it suits both sides because it was fitted to
// neither, and a style fitted to nothing is nobody's opposite — offering it as
// the counterpart of a style that does have a ground would answer "what was
// this scheme's other half" with a style that has no halves.
//
// The walk is over [Bases], which is sorted, and takes a strictly smaller
// distance to displace the leader, so the first-listed of two equally near
// candidates wins and the answer does not depend on anything but the styles.
func nearest(s *chroma.Style, dark bool) (string, bool) {
	best, bestAt := "", math.Inf(1)
	for _, name := range Bases() {
		c, ok := lookup(name)
		if !ok || c == s {
			continue
		}
		if cDark, grounded := polarity(c); !grounded || cDark != dark {
			continue
		}
		d, ok := distance(s, c)
		if !ok || d >= bestAt {
			continue
		}
		best, bestAt = name, d
	}
	return best, best != ""
}

// BaseDistance is how much of two bases' palettes falls into different hue
// families: 0 when every class they both colour is the same colour in both,
// and 1 when none of them is. The second result is false for a pair that
// cannot be compared — a name that resolves to nothing, or two styles with no
// coloured class in common.
//
// It is a chroma-weighted mean over the classes both styles colour. Each class
// contributes the angle between its two inks on the OKLCh hue circle, capped
// at one hue family and read as a fraction of one — so a class the two draw in
// the same colour contributes nothing, a class they draw in two unrelated
// colours contributes its full weight, and how unrelated stops mattering past
// the point where the answer is already "not the same colour". The weight is
// the smaller of the two chromas lifted off a floor: the smaller rather than
// the mean, because a grey compared against a saturated ink is a comparison of
// one hue that means something against one that is rounding noise, and the
// floor because a palette of near-neutrals must still be able to differ from
// something.
//
// Hue is the axis because it is the one a change of ground does not touch. The
// two halves of a declared pair differ in lightness everywhere by construction
// — that is what the two grounds are for — and in hue almost nowhere.
//
// Classes only one of the two styles colours are left out rather than counted
// as a mismatch. A style that takes no position on numbers has not disagreed
// about numbers with anybody.
//
// The space is the one seed extraction reads a palette in, and the chroma
// floor is the one it lifts its own emphasis off, so "how much colour has this
// ink got" is one question across the two and not two.
func BaseDistance(a, b string) (float64, bool) {
	sa, ok := lookup(a)
	if !ok {
		return 0, false
	}
	sb, ok := lookup(b)
	if !ok {
		return 0, false
	}
	return distance(sa, sb)
}

// distance is [BaseDistance] on two styles already resolved.
func distance(a, b *chroma.Style) (float64, bool) {
	return distanceWith(a, b, hueFamily)
}

// distanceWith is distance with the family width handed in. The width is the
// one number in the measure that was chosen rather than derived, so it is a
// parameter here: the test that picks it scores the whole rediscovery at a
// range of widths, which it could not do against a constant nailed into the
// arithmetic.
func distanceWith(a, b *chroma.Style, family float64) (float64, bool) {
	plainA, plainB := plainForeground(a), plainForeground(b)
	var sum, weight float64
	for _, tt := range hueClasses {
		inkA, okA := classInk(a, plainA, tt)
		inkB, okB := classInk(b, plainB, tt)
		if !okA || !okB {
			continue
		}
		_, chromaA, hueA := color.OKLChFromNRGBA(inkA)
		_, chromaB, hueB := color.OKLChFromNRGBA(inkB)
		w := classFloor + math.Min(chromaA, chromaB)
		sum += w * math.Min(hueGap(hueA, hueB), family) / family
		weight += w
	}
	if weight == 0 {
		return 0, false
	}
	return sum / weight, true
}

// classInk is the colour a style draws one class of token in, or false when it
// has no colour for that class.
//
// "No colour" includes resolving to the style's plain foreground, which is the
// same reading the highlighter itself makes: a run that comes out in the body
// colour is one the style had no opinion about, and this package emits it
// colourless so the theme's own text colour shows through. Counting it as a
// decision would have every style that declares a body colour agreeing with
// every other about every class neither of them colours.
func classInk(s *chroma.Style, plain chroma.Colour, tt chroma.TokenType) (stdcolor.NRGBA, bool) {
	e := s.Get(tt)
	if !e.Colour.IsSet() || e.Colour == plain {
		return stdcolor.NRGBA{}, false
	}
	return fromChroma(e.Colour), true
}

// hueGap is the angle between two OKLCh hues, in degrees, the short way round
// the circle.
func hueGap(a, b float64) float64 {
	gap := math.Abs(a - b)
	if gap > 180 {
		gap = 360 - gap
	}
	return gap
}
