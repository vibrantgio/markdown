// fence.go dresses a fenced code block in a syntax base, as its author drew
// it.
//
// A syntax style's ground, body ink and accents were curated together, so the
// block shows the base whole: the author's own background under the author's
// own inks, neither of them touched. The paper around it — the page, the
// prose, the chip an inline span sits on — stays the theme's. What the theme
// decides is which member of the pair is on screen, and that a block on a page
// is bounded: a ground too near the paper to be seen against it takes an edge.
//
// Contrast is surfaced, not enforced: no ink is moved and no style is failed
// for its author's taste. The sweep in the tests records what every base
// measures on its own ground and names the worst of them.

package highlight

import (
	"fmt"
	stdcolor "image/color"

	"github.com/alecthomas/chroma/v2"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// DefaultBase and DefaultDarkBase name the syntax palettes to draw code in
// when nothing else is chosen: chroma's catppuccin-latte under a light
// appearance and catppuccin-mocha under a dark one, which are each other's
// registered counterparts. They are defaults and not a policy — [Wear] takes
// any name in chroma's registry, and [WearPair] any two.
const (
	DefaultBase     = "catppuccin-latte"
	DefaultDarkBase = "catppuccin-mocha"
)

// Wear dresses st's fenced code blocks in the base named: its author's own
// background under the fence, their own inks in the runs they coloured, and
// their own body ink in the runs they left plain. Nothing else on the page
// moves — the prose, the chip an inline code span sits on, and the bar a wide
// block scrolls under all stay the theme's.
//
// One base names a pair, not a side. Which member is worn follows the tokens:
// c's own appearance decides light or dark, and chroma's registered
// counterpart supplies the other member, so Wear(&st, [DefaultBase], c) puts
// the catppuccin-latte plate on a light theme and the catppuccin-mocha one on
// a dark theme from the one name. A base with no counterpart is worn under
// both — and only 22 of the 74 embedded styles name one, so most names are a
// side however they are asked. A caller that has chosen a base for each
// appearance hands both over instead: see [WearPair].
//
// It is st's four code fields that are written: Highlight, CodeColor,
// CodeBackground and CodeBorder. Everything a Style says about anything else
// is left exactly as the caller had it, so the ordinary shape of this is
// [markdown.FromTokens] followed by one call. Nothing else is read: the
// block's edge is decided against the ground the base itself names, so a
// document mounted on some other paper takes the same fence it takes on the
// theme's own page.
//
// A name missing from chroma's style registry panics, as in [New].
//
// Dress the Style again when the theme changes: the base is resolved once and
// the highlighter closes over it, so nothing here can follow a theme
// observable.
func Wear(st *markdown.Style, base string, c tokens.ColorTokens) {
	WearPair(st, BasePair{Light: base, Dark: base}, c)
}

// WearPair is [Wear] for a caller holding a base per appearance: c's own
// appearance says which member is drawn, and that member's ground, inks and
// body colour are what the fence takes.
//
// The two members are independent artifacts, not two views of one: each is
// drawn as its own author wrote it, italics and bold included. A member naming
// nothing this package can resolve panics exactly as [Wear] does, if it is the
// member the appearance calls for.
func WearPair(st *markdown.Style, p BasePair, c tokens.ColorTokens) {
	surface := codeSurface(c)
	mode, name := chroma.Light, p.Light
	if isDarkSurface(surface) {
		mode, name = chroma.Dark, p.Dark
	}
	member, ok := forMode(name, mode)
	if !ok {
		panic(fmt.Sprintf("highlight: unknown style %q (Bases lists every name that resolves)", name))
	}

	// The registry's own style, straight through: the inks on screen are the
	// author's to the byte and nothing here alters one.
	plain := plainForeground(member)
	st.Highlight = spanner(member, plain)
	if plain.IsSet() {
		st.CodeColor = fromChroma(plain)
	}

	// The chip's fill is what a groundless base is drawn on. A Style built by
	// hand may carry no chip, and then the theme's own code fill stands in.
	fallback := st.CodeChip
	if fallback.A == 0 {
		fallback = surface
	}
	st.CodeBackground = fenceGround(member, fallback)
	st.CodeBorder = fenceEdge(st.CodeBackground, c)
}

// fenceGround is the fence's fill under one member: the background its author
// fitted their inks against, or fallback for the four embedded styles that
// name no background at all.
func fenceGround(member *chroma.Style, fallback stdcolor.NRGBA) stdcolor.NRGBA {
	if bg := member.Get(chroma.Background).Background; bg.IsSet() {
		return fromChroma(bg)
	}
	return fallback
}

// fenceEdge is the hairline a dressed fence draws to read as a block: the
// neutral rung nearest the ramp's mid-value step that reaches WCAG 1.4.11's
// 3:1 against the ground the author fitted their inks to.
//
// The rim is derived against the fence's own fill rather than against the
// paper, which is what makes it work for a ground this package has never seen:
// a dressed fence lies on the page and its rim is read against the block it
// encloses, so a palette fitted to paper and one fitted to slate are answered
// by the same call without either being named.
func fenceEdge(fence stdcolor.NRGBA, c tokens.ColorTokens) stdcolor.NRGBA {
	return c.MarkOn(tokens.RoleNeutral, fence, edgeFloor)
}

// edgeFloor is WCAG 1.4.11's contrast floor for a graphic that carries
// meaning without being text — 3:1. A fence's rim is exactly such a graphic:
// it is the whole of what says where the code begins once the fill is a
// whisper off the page.
const edgeFloor = 3.0

// codeSurface is the fill a code block is drawn on under these tokens before
// any base is worn. It is read back off the markdown style rather than from
// the neutral ramp directly, so the answer stays in one place: whatever the
// style constructor decided. The typography is irrelevant to it and the
// default stands in.
func codeSurface(c tokens.ColorTokens) stdcolor.NRGBA {
	return markdown.FromTokens(c, tokens.DefaultTypography).CodeBackground
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
