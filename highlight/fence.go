// fence.go dresses a fenced code block in a syntax highlighter style, as
// its author drew it.
//
// A style's background, body colour and accents were curated together, so
// the block shows the style whole: the author's own background under the
// author's own colours, neither of them touched. The content around it — the
// page, the prose, the chip an inline span sits on — stays the theme's. What
// the theme decides is which member of the pair is on screen, and that a block
// on a page is bounded: a background too near the page to be seen against it
// takes an edge.
//
// Contrast is surfaced, not enforced: no colour is moved and no style is
// failed for its author's taste. The sweep in the tests records what every
// style measures on its own background and names the worst of them.

package highlight

import (
	"fmt"
	stdcolor "image/color"

	"github.com/alecthomas/chroma/v2"

	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// DefaultStyle and DefaultDarkStyle name the syntax highlighter styles to
// draw code in when nothing else is chosen: chroma's catppuccin-latte under
// a light appearance and catppuccin-mocha under a dark one, which are each
// other's registered counterparts. They are defaults and not a policy — [Wear] takes
// any name in chroma's registry, and [WearPair] any two.
const (
	DefaultStyle     = "catppuccin-latte"
	DefaultDarkStyle = "catppuccin-mocha"
)

// Wear dresses st's fenced code blocks in the style named: its author's own
// background under the fence, their own colours in the runs they coloured, and
// their own body colour in the runs they left plain. Nothing else on the page
// moves — the prose and the chip an inline code span sits on stay the theme's.
// The bar of a wide block is the theme's bar, flattened onto the author's
// background.
//
// One style names a pair, not a side. Which member is worn follows the tokens:
// c's own appearance decides light or dark, and chroma's registered
// counterpart supplies the other member, so Wear(&st, [DefaultStyle], c) puts
// the catppuccin-latte plate on a light theme and the catppuccin-mocha one on
// a dark theme from the one name. A style with no counterpart is worn under
// both — and only 22 of the 74 embedded styles name one, so most names are a
// side however they are asked. A caller that has chosen a style for each
// appearance hands both over instead: see [WearPair].
//
// It is st's five code fields that are written: Highlight, CodeColor,
// CodeBackground, CodeBorder and the colours of CodeScrollbar. The bar keeps
// its sizes, its fade and the caller's Show scroll bars answer; only its
// thumb and its match fills are flattened onto the new background. A bar
// worn before or after this call is the same bar. Everything a Style says
// about anything else
// is left exactly as the caller had it, so the ordinary shape of this is
// [markdown.FromTokens] followed by one call. Nothing else is read: the
// block's edge is decided against the background the style itself names, so a
// document mounted on some other surface takes the same fence it takes on the
// theme's own page.
//
// A name missing from chroma's style registry panics, as in [New].
//
// Dress the Style again when the theme changes: the style is resolved once and
// the highlighter closes over it, so nothing here can follow a theme
// observable.
func Wear(st *markdown.Style, style string, p tokens.PlatformColors) {
	WearPair(st, StylePair{Light: style, Dark: style}, p)
}

// WearPair is [Wear] for a caller holding a style per appearance: c's own
// appearance says which member is drawn, and that member's background, colours
// and body colour are what the fence takes.
//
// The two members are independent artifacts, not two views of one: each is
// drawn as its own author wrote it, italics and bold included. A member naming
// nothing this package can resolve panics exactly as [Wear] does, if it is the
// member the appearance calls for.
func WearPair(st *markdown.Style, pair StylePair, p tokens.PlatformColors) {
	surface := codeSurface(p)
	mode, name := chroma.Light, pair.Light
	if isDarkSurface(surface) {
		mode, name = chroma.Dark, pair.Dark
	}
	member, ok := forMode(name, mode)
	if !ok {
		panic(fmt.Sprintf("highlight: unknown style %q (Styles lists every name that resolves)", name))
	}

	// The registry's own style, straight through: the colours on screen are the
	// author's to the byte and nothing here alters one.
	plain := plainForeground(member)
	st.Highlight = spanner(member, plain)
	if plain.IsSet() {
		st.CodeColor = fromChroma(plain)
	}

	// The chip's fill is what a style naming no background of its own is drawn on.
	// A Style built by hand may carry no chip, and then the theme's own code fill
	// stands in.
	fallback := st.CodeChip
	if fallback.A == 0 {
		fallback = surface
	}
	st.CodeBackground = fenceBackground(member, fallback)
	st.CodeBorder = fenceEdge(st.CodeBackground, p)
	st.CodeScrollbar = fenceBar(st.CodeScrollbar, st.CodeBackground, p)
}

// fenceBar is the code block's bar flattened onto the fence's fill. The
// platform's knob is a coverage of black or white, and [scrollbar.FromTokens]
// flattens that coverage onto the fill the bar is drawn over. A thumb
// flattened onto the theme's code fill is a neutral grey on the author's
// tinted background. The bar takes its colours from the fill it is drawn
// over, and keeps every other field the caller set. A zero bar draws
// nothing and stays zero.
func fenceBar(bar scrollbar.Style, fence stdcolor.NRGBA, p tokens.PlatformColors) scrollbar.Style {
	if bar.Width() <= 0 {
		return bar
	}
	over := scrollbar.FromTokens(p, fence)
	if bar.FadeDelay == 0 {
		over = over.Always()
		bar.TrackColor = over.TrackColor
	}
	bar.ThumbColor = over.ThumbColor
	bar.MatchFill = over.MatchFill
	bar.CurrentMatchFill = over.CurrentMatchFill
	return bar
}

// fenceBackground is the fence's fill under one member: the background its
// author fitted their colours against, or fallback for the four embedded
// styles that name no background at all.
func fenceBackground(member *chroma.Style, fallback stdcolor.NRGBA) stdcolor.NRGBA {
	if bg := member.Get(chroma.Background).Background; bg.IsSet() {
		return fromChroma(bg)
	}
	return fallback
}

// fenceEdge is the hairline a dressed fence draws to read as a block: the
// platform's separator laid over the background the author fitted their
// colours to.
//
// The seam lands on the fence's own fill rather than on the content, which is
// what makes it work for a background this package has never seen: a dressed
// fence lies on the page and its edge is inset into the block it encloses, so
// a palette fitted to a light page and one fitted to a dark one are answered
// by the same call without either being named.
//
// Which separator is a question about that fill and not about the appearance
// the document is read in. The platform draws its separator dark on a light
// fill and light on a dark one, and a style fitted to a dark page worn on a
// light theme is an ordinary thing to ask for — so the fill is asked which
// appearance's text reads on it, and it takes that appearance's separator, at
// the live set's own coverage. [markdown.FromTokens] needs none of this: its
// fence is the platform's own step off the page, so the set's own separator is
// already the right way round.
func fenceEdge(fence stdcolor.NRGBA, p tokens.PlatformColors) stdcolor.NRGBA {
	seam := tokens.PlatformLight.Separator
	if color.BestOn(fence, tokens.PlatformLight.Text, tokens.PlatformDark.Text) == tokens.PlatformDark.Text {
		seam = tokens.PlatformDark.Separator
	}
	seam.A = p.Separator.A
	return color.Flatten(seam, fence)
}

// codeSurface is the fill a code block is drawn on under this colour set
// before any style is worn. It is read back off the markdown style rather than
// being spelled again here, so the answer stays in one place: whatever the
// style constructor decided. The typography is irrelevant to it and the
// default stands in, and so does the page — the fence's own fill is what is
// wanted, not the surface under it.
func codeSurface(p tokens.PlatformColors) stdcolor.NRGBA {
	return markdown.FromTokens(p, tokens.DefaultTypography, stdcolor.NRGBA{}).CodeBackground
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
