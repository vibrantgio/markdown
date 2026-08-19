// adapt.go derives a style from a chroma base rather than wearing one.
//
// A stock style is a curated artifact: its inks were chosen against one
// particular background, and the relations between them — this hue for
// keywords, that one for strings, a near-neutral for comments — are the part
// worth keeping. What does not survive the move onto a themed code slab is
// the lightness. github's keyword red measures 3.61:1 on the tinted fill this
// theme puts under a fence, and its comment grey 4.31:1; both read as
// deliberate ink on the near-white the style was fitted to and as washed-out
// ink here. So the derivation holds each entry's hue and chroma and re-fits
// only its lightness, against the actual surface, until it clears the floor.
//
// Nothing in the registry is touched. The result is a new style built beside
// the stock one, which stays selectable and unmodified.

package highlight

import (
	"fmt"
	stdcolor "image/color"
	"math"
	"slices"

	"github.com/alecthomas/chroma/v2"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// contrastFloor is the WCAG 2 contrast ratio every coloured run has to reach
// against the code surface. Code renders at the typography's Code size, which
// is body size rather than large text, so the AA floor for normal text is the
// one that applies; 3:1 would be the large-text floor and does not.
//
// WCAG 2 rather than APCA, which is the design system's own gating metric
// elsewhere: an APCA Lc target is stated per text weight and size, and the
// ink being fitted here arrives from a foreign style at whatever weight that
// style asked for. The ratio is the metric that answers "is this ink legible
// on this fill" without a second opinion about the type.
const contrastFloor = 4.5

// refitSteps is how finely the re-fit walks lightness. The walk stops at the
// first step that clears the floor, so the step size is the most an entry can
// be darkened or lightened beyond what the floor actually demanded.
const refitSteps = 256

// DefaultBase names the syntax palette to derive from when nothing else is
// chosen: chroma's catppuccin-latte, whose registered counterpart
// catppuccin-mocha is the dark member the same name reaches. It is a default
// and not a policy — [Adapt] takes any name in chroma's registry.
//
// The pair is picked for what survives the derivation. Its accents already sit
// in one perceptual-lightness band, so hue is what tells its token types apart
// rather than lightness; re-fitting lightness against a surface therefore
// costs it nothing it was using to carry meaning, where a palette that
// distinguished a keyword from a string by how dark it is would come out of
// the same fit with the two harder to tell apart. Its two members also already
// agree entry for entry on bold and italic, so the one policy across the pair
// changes nothing about it — comments are italic in both appearances rather
// than in one.
const DefaultBase = "catppuccin-latte"

// Options are the dials on [AdaptWith]. The zero value is what [Adapt] uses:
// the surface re-fit alone, which is not a dial — a style that cannot be read
// on the surface it is drawn on is not a style.
type Options struct {
	// AlignToBrand rotates every entry's hue by one angle: the difference
	// between the theme's primary hue and the base's own dominant hue. Every
	// entry turns by the same amount, so the offsets between them — what
	// makes strings and keywords tell apart at a glance — are preserved
	// exactly, and the palette as a whole leans toward the brand instead of
	// standing beside it as a second accent.
	//
	// Off by default. A base's absolute hues are part of what a reader
	// recognises it by, and turning them is a choice about this theme rather
	// than a correction to the style.
	AlignToBrand bool
}

// Adapt returns a Highlighter that colours code with a style derived from the
// named chroma base and fitted to c: every entry keeps its hue and as much of
// its chroma as sRGB still holds, and takes the lightness that clears the
// contrast floor against the code surface these tokens put under a fence. Runs
// the base renders in its plain-text foreground still come back colourless, so
// plain code follows [markdown.Style].CodeColor exactly as it does under [New].
//
// The base names a pair, not a side. Which member is derived from follows the
// tokens: c's own code surface decides light or dark, and chroma's registered
// counterpart supplies the other member, so Adapt([DefaultBase], …) yields the
// catppuccin-latte inks on a light theme and the catppuccin-mocha inks on a
// dark one from the one name. A base with no counterpart is derived from for
// both.
//
// A name missing from chroma's style registry panics, as in [New].
//
// Build a new Highlighter when the theme changes: the style is derived once
// and the returned func closes over it.
func Adapt(base string, c tokens.ColorTokens) markdown.Highlighter {
	return AdaptWith(base, c, Options{})
}

// AdaptWith is [Adapt] with the dials exposed.
func AdaptWith(base string, c tokens.ColorTokens, opt Options) markdown.Highlighter {
	style := derive(base, c, opt)
	return spanner(style, plainForeground(style))
}

// codeSurface is the fill a code block is drawn on under these tokens. It is
// read back off the markdown style rather than from the neutral ramp directly,
// because the question the re-fit asks is "what will this ink actually sit
// on", and the answer is whatever the style constructor decided — one place,
// not two. The typography is irrelevant to it and the default stands in.
func codeSurface(c tokens.ColorTokens) stdcolor.NRGBA {
	return markdown.FromTokens(c, tokens.DefaultTypography).CodeBackground
}

// derive builds the adapted style. It is unexported, and so is every other
// signature in this package that names a chroma type: see the package comment
// in highlight.go for why the dependency is confined here.
func derive(name string, c tokens.ColorTokens, opt Options) *chroma.Style {
	surface := codeSurface(c)
	mode := chroma.Light
	if isDarkSurface(surface) {
		mode = chroma.Dark
	}
	// The member the theme's own appearance calls for, and the member the
	// emphasis policy comes from. They are the same style whenever the base
	// is unpaired.
	member, ok := forMode(name, mode)
	if !ok {
		panic(fmt.Sprintf("highlight: unknown style %q (Bases lists every name that resolves)", name))
	}
	policy, _ := forMode(name, chroma.Light)

	var turn float64
	if opt.AlignToBrand {
		if h, ok := dominantHue(member); ok {
			_, _, brand := color.OKLChFromNRGBA(c.Primary)
			// The signed short way round, so a rotation is never the long
			// way past the far side of the wheel.
			turn = math.Mod(brand-h+540, 360) - 180
		}
	}

	b := chroma.NewStyleBuilder(member.Name + "-adapted")
	for _, tt := range member.Types() {
		e := member.Get(tt)
		if e.Colour.IsSet() {
			ink := fromChroma(e.Colour)
			if turn != 0 {
				ink = rotateHue(ink, turn)
			}
			ink = refit(ink, surface, contrastFloor)
			e.Colour = chroma.NewColour(ink.R, ink.G, ink.B)
		}
		// One emphasis policy across the pair, taken from the light member.
		// Chroma's two github styles disagree on this: the dark one italicises
		// comments and bolds operators, functions, classes and constants where
		// the light one asks for neither, so the same document changes its
		// typographic emphasis when the appearance changes — a difference the
		// reader has no way to read as meaning anything. The light member
		// settles it because a light style is the one fitted to paper, where
		// mono italics and synthetic bold cost the most and get used the least.
		//
		// Written as an explicit yes or no rather than left to inherit, so a
		// token type that takes no position cannot pick one up from its
		// category.
		p := policy.Get(tt)
		e.Bold = pin(p.Bold)
		e.Italic = pin(p.Italic)
		e.Underline = pin(p.Underline)
		b.AddEntry(tt, e)
	}
	// Record the surface the style was fitted to. Nothing in this package
	// paints it — the markdown style fills the slab — but a derived style that
	// carried the base's background would report the wrong appearance and
	// describe a fit it never had.
	bg := b.Get(chroma.Background)
	bg.Background = chroma.NewColour(surface.R, surface.G, surface.B)
	b.AddEntry(chroma.Background, bg)

	style, err := b.Build()
	if err != nil {
		// Every entry came out of chroma's own parser and went back through
		// its own printer, so this is unreachable short of a chroma bug.
		panic(fmt.Sprintf("highlight: deriving from %q: %v", name, err))
	}
	return style
}

// pin turns a trilean into a decision. Pass means "inherit", which is exactly
// what a single policy across a scheme pair must not leave open.
func pin(t chroma.Trilean) chroma.Trilean {
	if t == chroma.Yes {
		return chroma.Yes
	}
	return chroma.No
}

// dominantHue is the chroma-weighted circular mean of the hues a style
// colours with: each entry's OKLCh hue enters as a unit vector scaled by its
// own chroma, and the mean is the direction of the sum. Weighting by chroma is
// what makes it the style's *accent* hue — a near-neutral comment grey carries
// a hue too, and counting it as loudly as a saturated keyword would drag the
// answer toward whatever cast the style's greys happen to have.
//
// The second result is false for a style with no chromatic ink at all, where
// there is no dominant hue to speak of and nothing to align.
func dominantHue(s *chroma.Style) (float64, bool) {
	types := s.Types()
	// Summation order changes the last bits of a float sum, and the type list
	// comes out of a map. Sorted, the answer is the same on every run.
	slices.Sort(types)
	var x, y float64
	for _, tt := range types {
		e := s.Get(tt)
		if !e.Colour.IsSet() {
			continue
		}
		_, chr, h := color.OKLChFromNRGBA(fromChroma(e.Colour))
		rad := h * math.Pi / 180
		x += chr * math.Cos(rad)
		y += chr * math.Sin(rad)
	}
	if x == 0 && y == 0 {
		return 0, false
	}
	h := math.Atan2(y, x) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return h, true
}

// rotateHue turns one ink around the OKLCh hue circle, holding its lightness
// and chroma. A colour with no chroma has no hue to turn.
func rotateHue(c stdcolor.NRGBA, turn float64) stdcolor.NRGBA {
	l, chr, h := color.OKLChFromNRGBA(c)
	if chr <= 0 {
		return c
	}
	return color.NRGBAFromOKLCh(l, chr, math.Mod(h+turn+360, 360))
}

// refit returns c at the lightness that clears floor against surface, holding
// c's hue and chroma. Ink that already clears it is returned untouched — the
// base's own lightness is part of what was curated, and moving it when it
// works would trade a good colour for a compliant one.
//
// Ink that does not clear it walks away from the surface: darker on a light
// fill, lighter on a dark one, one step at a time, stopping at the first step
// that passes. That is the smallest correction the floor demands rather than
// the largest one available, so a keyword that misses by a tenth of a ratio
// point moves about that far and no further.
//
// Chroma is held as an intent, and sRGB has the last word on it. A saturated
// orange at the lightness its author chose is a colour the display has; the
// same orange a third of a lightness darker is not, and the conversion answers
// out-of-gamut by reducing chroma at constant lightness and constant hue. So a
// deeply corrected ink can come back less saturated than it went in — never
// more, and never on another hue.
func refit(c, surface stdcolor.NRGBA, floor float64) stdcolor.NRGBA {
	if color.ContrastRatio(c, surface) >= floor {
		return c
	}
	l, chr, h := color.OKLChFromNRGBA(c)
	target := 1.0
	if !isDarkSurface(surface) {
		target = 0.0
	}
	for i := 1; i <= refitSteps; i++ {
		t := float64(i) / refitSteps
		cand := color.NRGBAFromOKLCh(l+(target-l)*t, chr, h)
		if color.ContrastRatio(cand, surface) >= floor {
			return cand
		}
	}
	// Black on a light fill or white on a dark one is as far as lightness
	// goes. Reaching here means the surface itself is too middling for any
	// ink to clear the floor on, which the theme's own contrast gates would
	// have caught long before a code block did.
	return color.NRGBAFromOKLCh(target, chr, h)
}

// isDarkSurface reports whether a fill reads as dark, on the perceptual
// lightness axis rather than a luma sum: mid-grey is perceptually mid, and a
// luma threshold calls it dark.
func isDarkSurface(c stdcolor.NRGBA) bool {
	l, _, _ := color.OKLChFromNRGBA(c)
	return l < 0.5
}

// fromChroma converts a chroma colour to the standard one. Chroma packs RGB
// into an int32 with zero reserved for "unset", so this is only meaningful for
// a colour that IsSet.
func fromChroma(c chroma.Colour) stdcolor.NRGBA {
	return stdcolor.NRGBA{R: c.Red(), G: c.Green(), B: c.Blue(), A: 0xFF}
}
