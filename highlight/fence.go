// fence.go dresses a fenced code block in a syntax base, as its author drew
// it.
//
// A syntax style is a curated artifact and its parts were curated together: a
// ground, a body ink, and a couple of dozen accents chosen against that
// ground. Which of them reads loudest, which recedes, and by how much is a set
// of relations, and the relations only hold where the whole set is. Re-fitting
// the inks onto some other surface keeps the hues and loses the artifact — and
// on a palette drawn for paper it loses most of it, because nearly every ink
// has to move. Three dark flavours of one family that differ mostly by their
// grounds come out of a re-fit indistinguishable, which is the plainest
// statement of what a re-fit is discarding.
//
// So the block shows the base whole: the author's own background under the
// author's own inks, neither of them touched. The paper around it — the page,
// the prose, the chip an inline span sits on — stays the theme's; the fence is
// content, and content is shown as it was made. What the theme still decides
// is which member of the pair is on screen, and that a block on a page is
// bounded: a ground too near the paper to be seen against it takes an edge.
//
// Contrast here is surfaced, not enforced. A base whose author drew a quiet
// palette is drawn quiet; the sweep in the tests records what every base
// measures on its own ground and names the worst of them, and no ink is moved
// and no style is failed for its author's taste.

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
//
// The pair is picked for what it says about the two appearances. Its members
// are one family drawn twice rather than one palette lit twice: the same token
// types on the same hues, at the two volumes their author set for paper and
// for slate, so a change of appearance changes the whole plate and not just
// the sheet it is on. Its accents also sit in one perceptual-lightness band
// per member, which is what makes hue rather than weight the thing telling a
// keyword from a string — the distinction a reader keeps across a scheme
// switch.
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
// theme's own page. It used to read st.Paper for that comparison; ADR-022
// retired the comparison (see fenceEdge).
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
// A pair is what a person chooses when they choose twice, and the two members
// owe each other nothing: a set of inks balanced against a near-white page is
// not the set anybody would balance against a near-black one, so the light
// member and the dark member are two artifacts and not two views of one.
// Wearing through the pair rather than through one name is therefore what
// makes a change of appearance a change of palette — the whole point of
// keeping two. Each member is drawn as its own author wrote it, italics and
// bold included; a member naming nothing this package can resolve panics
// exactly as [Wear] does, if it is the member the appearance calls for.
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

	// The registry's own style, straight through: the entries the spanner
	// reads are the author's entries, so the inks on screen are theirs to the
	// byte and nothing here can alter one.
	plain := plainForeground(member)
	st.Highlight = spanner(member, plain)
	if plain.IsSet() {
		st.CodeColor = fromChroma(plain)
	}

	// The chip's fill is what a groundless base is drawn on: it is the fill a
	// fence had before any base was chosen, and a style fitted to nothing has
	// no ground of its own to prefer to it. A Style built by hand may carry no
	// chip, and then the theme's own code fill stands in.
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
// It is the theme's own rule, applied to somebody else's fill. This used to
// be a comparison instead — a base whose ground stood off the paper at least
// as far as the theme's own fence did took no line, and one that fell short
// took the theme's divider. The comparison had a floor under it that ADR-022
// removed: the theme's fence separated from the paper by one ramp step,
// 1.13:1 light and 1.12:1 dark, and "as far as ours" was a real bar to clear.
// Since the ladder was re-founded the light fence sits 1.02:1 off its paper,
// so almost any ground would clear that bar while telling a reader nothing,
// and the theme's own fence takes a derived rim in both schemes rather than
// none in either (see markdown's codeRim). A dressed fence is the same
// construct with an author's fill in it, so it takes the same edge — one
// rule, no comparison to keep calibrated, and no scheme named anywhere in
// it.
//
// Deriving against the fence's own fill rather than against the paper is
// what makes that work for a ground this package has never seen. The line's
// harder side is the fill it is drawn on, since a dressed fence lies on the
// page and its rim is read against the block it encloses; and a palette
// fitted to paper and a palette fitted to slate are answered by the same
// call without either being named.
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
// the neutral ramp directly, because the question being asked is "what does
// this theme put under a fence by itself", and the answer is whatever the
// style constructor decided — one place, not two. The typography is
// irrelevant to it and the default stands in.
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
