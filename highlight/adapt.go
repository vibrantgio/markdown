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
// only its lightness against the actual surface.
//
// Two things are kept, not one. Each ink's hue and chroma are the first. The
// second is the order the author drew them in — which token type reads loudest
// against the ground it was fitted to, which recedes, and by how much — and
// keeping it takes more than a floor. A fit that lifted only the inks failing a
// contrast floor kept every ink that passed exactly as drawn, which is most of
// a palette fitted to a near-black ground and almost none of one fitted to
// paper: on a light base nearly every ink fails against a light slab, so
// keyword, string, number and comment all landed within a hundredth of a ratio
// point of the floor and the palette came out flat.
//
// So the fit is a normalisation. Each ink is placed by what it measures against
// its own author's ground, and that ordering is stretched onto the band this
// theme reads code in: the floor at the quiet end, an anchor far above it at
// the loud end. The map only ever pushes ink away from the surface — an ink
// already at or past its place keeps the colour its author chose, so a palette
// drawn for a ground like this one comes through untouched, and no ink is ever
// made quieter than it was drawn.
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

// contrastAnchor is where the loudest ink of a palette lands: the top of the
// band the fit stretches a base onto, as the floor is its bottom.
//
// It is set high deliberately. A band ending just above the floor would pass
// the gate and still read as one weight of ink, which is the defect the
// normalisation answers; and plain code — the uncoloured text between the
// coloured runs, which the theme inks itself — sits about two thirds of the way
// up this band in both appearances, so an anchor much lower would leave a
// keyword quieter than the identifier beside it.
//
// Ten is where it stops rather than higher, because a light ground charges for
// contrast in chroma. An accent walked to 10:1 on a near-white slab keeps most
// of the saturation its author gave it; the same accent walked to 12:1 has
// given up a third of it to the gamut and reads as dark ink with a tint in it
// rather than as a colour. Above the anchor is not forbidden — an ink drawn
// louder than its place stays where it was drawn — it is simply not somewhere
// the fit pushes ink to.
const contrastAnchor = 10.0

// flatSpan is the ratio between a palette's loudest and quietest ink below
// which it has no order to preserve: a base whose loudest reaches less than a
// quarter more contrast than its quietest was drawn as one weight of ink.
//
// The threshold is there because the alternative is inventing a hierarchy out
// of arithmetic. One embedded base sets every token type at one lightness and
// tells them apart by hue alone; its inks measure 4.57:1 and 4.58:1 against its
// own ground, and a stretch that took that hundredth of a point for an author's
// intent would spread it across the whole band. The measured gap either side of
// this value is wide — the flat base spans 1.00x and the next flattest 1.49x —
// so nothing sits near the line.
const flatSpan = 1.25

// refitSteps is how finely the re-fit walks lightness. The walk stops at the
// first step that reaches an ink's target, so the step size is the most an
// entry can be darkened or lightened beyond what its target actually demanded.
const refitSteps = 256

// atTarget is how close to its target an ink counts as being at it. A target is
// a placement in a band rather than a measurement, and a tenth of a ratio point
// is about what one 8-bit step of lightness is worth at these levels — so an
// ink already inside the last step a display can express is left exactly as its
// author drew it instead of being recoloured by a rounding difference.
//
// The floor is not softened by it. An ink under 4.5:1 is re-fitted whatever its
// target says, because the floor is the one number here that is a measurement.
const atTarget = 0.1

// DefaultBase and DefaultDarkBase name the syntax palettes to derive from when
// nothing else is chosen: chroma's catppuccin-latte under a light appearance
// and catppuccin-mocha under a dark one, which are each other's registered
// counterparts. They are defaults and not a policy — [Adapt] takes any name in
// chroma's registry, and [AdaptPair] any two.
//
// The pair is picked for what survives the derivation. Its accents already sit
// in one perceptual-lightness band, so hue is what tells its token types apart
// rather than lightness, and hue is the one thing a change of ground cannot
// touch: re-fitting against a surface costs this pair nothing at all, where a
// palette that tells a keyword from a string by how dark it is has that
// difference re-drawn — preserved in order and in proportion, but at the
// volume this theme reads at rather than at the one its author set.
// Its two members also already
// agree entry for entry on bold and italic, so the one policy across the pair
// changes nothing about it — comments are italic in both appearances rather
// than in one.
const (
	DefaultBase     = "catppuccin-latte"
	DefaultDarkBase = "catppuccin-mocha"
)

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
// its chroma as sRGB still holds, and takes the lightness that puts it where
// its author ranked it, on a band from the contrast floor to an anchor well
// above it, against the code surface these tokens put under a fence. Runs
// the base renders in its plain-text foreground still come back colourless, so
// plain code follows [markdown.Style].CodeColor exactly as it does under [New].
//
// One base names a pair, not a side. Which member is derived from follows the
// tokens: c's own code surface decides light or dark, and chroma's registered
// counterpart supplies the other member, so Adapt([DefaultBase], …) yields the
// catppuccin-latte inks on a light theme and the catppuccin-mocha inks on a
// dark one from the one name. A base with no counterpart is derived from for
// both — and only 22 of the 74 embedded styles name one, so most names are a
// side however they are asked. A caller that has chosen a base for each
// appearance hands both over instead: see [AdaptPair].
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
	return AdaptPairWith(BasePair{Light: base, Dark: base}, c, opt)
}

// AdaptPair is [Adapt] for a caller holding a base per appearance: c's own
// code surface says which appearance is being drawn, and the pair's member for
// that appearance is what the code is coloured from. Everything else is
// [Adapt] — each entry keeps its hue, takes the lightness its place in the
// base's own ordering asks for, and comes back legible on the surface these
// tokens put under a fence.
//
// A pair is what a person chooses when they choose twice, and the two members
// owe each other nothing: a set of inks balanced against a near-white page is
// not the set anybody would balance against a near-black one, so the light
// member and the dark member are two artifacts and not two views of one.
// Deriving through the pair rather than through one name is therefore what
// makes a change of appearance a change of palette — the whole point of
// keeping two.
//
// Each member settles its own emphasis, unlike the pair one name reaches: both
// of these were chosen, so a member is drawn as its author wrote it rather than
// re-typeset from the other half of somebody's pair. See [Options] for the
// dials; a member naming nothing this package can resolve panics exactly as
// [Adapt] does, if it is the member the appearance calls for.
func AdaptPair(p BasePair, c tokens.ColorTokens) markdown.Highlighter {
	return AdaptPairWith(p, c, Options{})
}

// AdaptPairWith is [AdaptPair] with the dials exposed.
func AdaptPairWith(p BasePair, c tokens.ColorTokens, opt Options) markdown.Highlighter {
	style := derivePair(p, c, opt)
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

// derive builds the adapted style from one name, which is the pair whose two
// members are that name: chroma's own counterpart rule inside [forMode] is
// what reaches the other appearance, exactly as it did before a caller could
// name each appearance itself.
//
// It is unexported, and so is every other signature in this package that names
// a chroma type: see the package comment in highlight.go for why the
// dependency is confined here.
func derive(name string, c tokens.ColorTokens, opt Options) *chroma.Style {
	return derivePair(BasePair{Light: name, Dark: name}, c, opt)
}

// derivePair builds the adapted style from a base per appearance.
func derivePair(p BasePair, c tokens.ColorTokens, opt Options) *chroma.Style {
	surface := codeSurface(c)
	mode, name := chroma.Light, p.Light
	if isDarkSurface(surface) {
		mode, name = chroma.Dark, p.Dark
	}
	// The member the theme's own appearance calls for.
	member, ok := forMode(name, mode)
	if !ok {
		panic(fmt.Sprintf("highlight: unknown style %q (Bases lists every name that resolves)", name))
	}
	policy := member
	if p.Light == p.Dark {
		// One name standing for both appearances. The other member is
		// chroma's counterpart and not a choice, so the pair's light member
		// settles the emphasis for both — see the entry loop below.
		if s, ok := forMode(p.Light, chroma.Light); ok {
			policy = s
		}
	}

	var turn float64
	if opt.AlignToBrand {
		if h, ok := dominantHue(member); ok {
			_, _, brand := color.OKLChFromNRGBA(c.Primary)
			// The signed short way round, so a rotation is never the long
			// way past the far side of the wheel.
			turn = math.Mod(brand-h+540, 360) - 180
		}
	}

	fit := bandFor(member, surface)
	b := chroma.NewStyleBuilder(member.Name + "-adapted")
	for _, tt := range member.Types() {
		e := member.Get(tt)
		if e.Colour.IsSet() {
			ink := fromChroma(e.Colour)
			// The band places the ink the author drew, before any rotation:
			// the ordering being preserved is theirs, and turning a palette
			// toward the brand is not meant to re-rank it. Two hues at one
			// lightness measure slightly different contrasts, and letting that
			// difference decide the order would make the dial a reordering.
			target := fit.target(ink)
			if turn != 0 {
				ink = rotateHue(ink, turn)
			}
			ink = refit(ink, surface, target)
			e.Colour = chroma.NewColour(ink.R, ink.G, ink.B)
		}
		// The emphasis comes from the style that was chosen.
		//
		// Where one name stands for both appearances, the second style is
		// chroma's counterpart rather than anybody's choice, and the two can
		// disagree: chroma's dark github italicises comments and bolds
		// operators, functions, classes and constants where its light one asks
		// for neither, so the same document would change its typographic
		// emphasis on a switch nobody made — a difference the reader has no way
		// to read as meaning anything. The light member settles it for both,
		// because a light style is the one fitted to paper, where mono italics
		// and synthetic bold cost the most and get used the least.
		//
		// Where the pair is two names, both were chosen, for the ground each
		// was fitted to. Then the member being drawn settles its own emphasis:
		// a style rewritten to another style's idea of what leans is not the
		// style that was picked, and the appearance on screen must be a
		// function of the choice made for it and nothing else — otherwise a
		// name chosen under one appearance quietly re-typesets the other.
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

// band is a base's own contrast ordering, ready to be read as a target for
// each of its inks: the ground its author fitted the palette to, and the
// quietest and loudest contrast any of its coloured inks reaches against that
// ground.
type band struct {
	ground stdcolor.NRGBA
	lo, hi float64
}

// bandFor measures a base against its own ground.
//
// Its own, and not the surface it is about to be drawn on, because the
// ordering being preserved is the author's: a comment is the quietest ink in a
// palette because its author drew it quietest against the paper or the slate
// they had in front of them, and that fact is only legible against that ground.
// Measured against ours instead, a dark base's near-black comment would
// outrank its keyword on a light fill — the palette read upside down.
//
// A style that names no ground was fitted to nothing there is any record of, so
// the surface it is about to be drawn on stands in: its inks then rank against
// the fill they will actually sit on, which is the only ground there is to rank
// them by. Four of the embedded styles are like this, and it is the same
// measurement that decides which appearance a base is offered under.
//
// The base's plain foreground is left out of the span. It is the colour the
// theme inks itself — every run resolving to it comes back colourless — so
// counting it would let ink this package never emits set where the emitted ink
// lands.
func bandFor(s *chroma.Style, surface stdcolor.NRGBA) band {
	ground := surface
	if bg := s.Get(chroma.Background).Background; bg.IsSet() {
		ground = fromChroma(bg)
	}
	b := band{ground: ground, lo: math.Inf(1)}
	plain := plainForeground(s)
	types := s.Types()
	// Summation order is irrelevant to a min and a max, but the type list comes
	// out of a map and a sorted walk keeps the numbers in any log identical
	// between runs.
	slices.Sort(types)
	for _, tt := range types {
		e := s.Get(tt)
		if !e.Colour.IsSet() || e.Colour == plain {
			continue
		}
		r := color.ContrastRatio(fromChroma(e.Colour), ground)
		b.lo, b.hi = math.Min(b.lo, r), math.Max(b.hi, r)
	}
	return b
}

// target is the contrast ink is fitted to: its place in its author's own
// ordering, read onto the band between the floor and the anchor.
//
// The map is affine in the contrast ratio, so an ink a third of the way up its
// palette's own range comes out a third of the way up the band: the ordering is
// preserved exactly and the spacing proportionally, which is what makes the
// result the author's palette at this theme's volume rather than a different
// palette at the right volume.
//
// A flat base has no order to stretch and every ink targets the floor, which
// leaves the loud ones exactly where they were drawn (see refit) and lifts only
// the ones that cannot be read. An ink outside the span — the plain foreground,
// which the span deliberately skips — clamps to the band's ends.
func (b band) target(ink stdcolor.NRGBA) float64 {
	if !(b.hi >= b.lo*flatSpan) {
		return contrastFloor
	}
	r := color.ContrastRatio(ink, b.ground)
	t := contrastFloor + (r-b.lo)/(b.hi-b.lo)*(contrastAnchor-contrastFloor)
	return math.Max(contrastFloor, math.Min(contrastAnchor, t))
}

// refit returns c at the lightness that reaches target against surface, holding
// c's hue and chroma. Ink already at its target is returned untouched — the
// base's own lightness is part of what was curated, and moving it when it is
// already where the band would put it would trade a chosen colour for an
// arithmetic one. Whole palettes come back this way: a base fitted to a ground
// much like the one it is being drawn on is mostly already where it belongs.
//
// Ink short of its target walks away from the surface: darker on a light fill,
// lighter on a dark one, one step at a time, stopping at the first step that
// reaches it. That is the smallest correction the target demands rather than
// the largest one available, so an ink that misses by a tenth of a ratio point
// moves about that far and no further.
//
// The walk runs in one direction only, which is what makes the guarantee a
// guarantee: ink is never made quieter than its author drew it. A target under
// what an ink already reaches is a target already met, so the band lifts a
// palette and never flattens one — a base drawn louder at its top than the
// anchor keeps its own top.
//
// Chroma is held as an intent, and sRGB has the last word on it. A saturated
// orange at the lightness its author chose is a colour the display has; the
// same orange a third of a lightness darker is not, and the conversion answers
// out-of-gamut by reducing chroma at constant lightness and constant hue. So a
// deeply corrected ink can come back less saturated than it went in — never
// more, and never on another hue.
func refit(c, surface stdcolor.NRGBA, target float64) stdcolor.NRGBA {
	if r := color.ContrastRatio(c, surface); r >= target-atTarget && r >= contrastFloor {
		return c
	}
	l, chr, h := color.OKLChFromNRGBA(c)
	end := 1.0
	if !isDarkSurface(surface) {
		end = 0.0
	}
	for i := 1; i <= refitSteps; i++ {
		t := float64(i) / refitSteps
		cand := color.NRGBAFromOKLCh(l+(end-l)*t, chr, h)
		if color.ContrastRatio(cand, surface) >= target {
			return cand
		}
	}
	// Black on a light fill or white on a dark one is as far as lightness
	// goes. Reaching here means the surface itself is too middling for any
	// ink to reach the target on, which for the floor is a surface the theme's
	// own contrast gates would have caught long before a code block did.
	return color.NRGBAFromOKLCh(end, chr, h)
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
