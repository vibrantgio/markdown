// contrast.go — how loudly a base draws, measured on its own ground.
//
// Contrast in content is surfaced and not enforced. Nothing here moves an ink
// or refuses a style: a base is shown as its author drew it, and what this file
// adds is the ability to say, quietly and beside the thing, that a particular
// palette was drawn faint. A chooser can pass that on; a renderer never has to
// ask.
//
// The measurement is deliberately not the whole entry table. A syntax style is
// mostly a set of decisions about code, plus a handful of markers for things
// that are not code — deleted lines, error spans, trailing whitespace — and
// several of those are drawn in the ground colour on purpose, at a ratio of one
// to one, because being invisible until something goes wrong is the job. A
// count over everything would call a careful palette faint on the strength of
// one marker its author meant nobody to see. So the reading is over the runs a
// person's eye is actually on while reading code, and the verdict is a majority
// of them rather than the worst of them.

package highlight

import (
	stdcolor "image/color"

	"github.com/alecthomas/chroma/v2"

	"github.com/vibrantgio/theme/color"
)

// ContrastFloor is WCAG 2's AA ratio for normal text, which is the size code is
// set at. Nothing in this package enforces it — no ink is moved and no style is
// refused for falling under it. It is the yardstick [BaseContrast] reports
// against, so that "this base is drawn faint" comes out as a measurement
// somebody can act on rather than as an opinion about somebody's palette.
const ContrastFloor = 4.5

// readingClasses are the runs measured: the kinds of thing a person reading
// code is actually looking at. They are the classes the palette comparison
// walks (see hueClasses) and for the same reason — every one of them is a class
// most styles genuinely set, so a shortlist of them says something about a
// palette where a walk over every entry type would mostly say which markers an
// author bothered to define.
var readingClasses = hueClasses

// AuthoredContrast is what a base's own inks measure against the ground its own
// author drew them on: how many reading inks there were, and how many of them
// fall under [ContrastFloor].
//
// Both numbers are counts and not a score. A caller wanting a verdict asks
// [AuthoredContrast.BelowFloor]; a caller wanting to say how much of a palette
// is faint has the fraction.
type AuthoredContrast struct {
	// Inks is how many reading inks were measured — the body colour, and each
	// reading class the base colours differently from it.
	Inks int
	// Below is how many of those measure under [ContrastFloor].
	Below int
}

// BelowFloor reports whether this base reads faint: whether most of what it
// draws code in falls under [ContrastFloor] on its author's own ground.
//
// A majority and not a single ink. One faint ink in a palette is ordinary and
// often deliberate — comments are meant to recede, and a marker drawn in the
// ground colour is meant to disappear — so a rule that fired on the quietest
// reading would fire on nearly every style there is, which is a rule that says
// nothing. A majority fires when the palette as a whole was drawn faint, which
// is the thing a reader would notice and the thing worth mentioning.
//
// The zero value reads false: nothing measured is not a finding.
func (a AuthoredContrast) BelowFloor() bool { return a.Inks > 0 && 2*a.Below > a.Inks }

// BaseContrast measures the base named against the ground its author fitted it
// to: the colour it sets plain code in, and the colour it gives each of the
// reading classes it takes a position on, each read as a contrast ratio against
// that ground.
//
// The ground is the author's own and there is no substitute for it. A palette's
// ratios are a fact about the pairing its author made, so a base fitted to no
// ground at all — four of the embedded styles are like this — has no authored
// contrast to report, and comes back false rather than measured against a
// surface somebody else chose. So does a name that resolves to nothing, and so
// does a base that colours no reading run at all: a style taking no position on
// code has not drawn it faint.
//
// Nothing is measured that the renderer would not draw, and everything it draws
// is measured as drawn. A class a style leaves out inherits its parent's ink,
// which is the ink a run of it comes out in, so that is the ink read here. An
// entry resolving to the style's own plain foreground is the other side of the
// same rule: it is a run the highlighter emits colourless rather than a decision
// about that class, so it is left out, its colour having already been counted
// once as the body ink.
func BaseContrast(name string) (AuthoredContrast, bool) {
	s, ok := lookup(name)
	if !ok {
		return AuthoredContrast{}, false
	}
	bg := s.Get(chroma.Background).Background
	if !bg.IsSet() {
		return AuthoredContrast{}, false
	}
	ground := fromChroma(bg)

	var a AuthoredContrast
	read := func(ink stdcolor.NRGBA) {
		a.Inks++
		if color.ContrastRatio(ink, ground) < ContrastFloor {
			a.Below++
		}
	}
	plain := plainForeground(s)
	if plain.IsSet() {
		read(fromChroma(plain))
	}
	for _, tt := range readingClasses {
		if ink, ok := classInk(s, plain, tt); ok {
			read(ink)
		}
	}
	return a, a.Inks > 0
}
