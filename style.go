package markdown

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/components/richtext"
	"github.com/vibrantgio/theme/tokens"
)

// CodeSpan is one coloured run of a highlighted code block, produced by a
// [Highlighter]. The zero flags describe plain code in [Style].CodeColor.
type CodeSpan struct {
	// Text is the run's content.
	Text string
	// Color is the run colour; the zero value falls back to Style.CodeColor.
	Color color.NRGBA
	// Bold renders the run in the bold monospace weight.
	Bold bool
	// Italic renders the run in the italic monospace style.
	Italic bool
}

// Highlighter syntax-highlights one code block: given the fence's language
// and the block's code it returns the code split into coloured runs, in
// order and concatenating back to the input. Returning nil renders the block
// plain. The hook is a plain func so implementations — like the chroma-backed
// one in markdown/highlight — never enter this package's dependency graph.
type Highlighter func(language, code string) []CodeSpan

// ImageProvider supplies the pixels for a document's [Image] blocks. The
// library performs no I/O of its own: fetching, decoding, and caching policy
// belong to the caller. Returning an error (or a nil image) makes the
// document render the block's alt text instead.
type ImageProvider interface {
	// Image returns the decoded image for a markdown destination URL.
	Image(url string) (image.Image, error)
}

// WidgetImageProvider is the optional vector extension of [ImageProvider]:
// an Images value that also implements it can serve an image as a live
// widget — vector geometry that stays crisp at any scale and pixel density —
// instead of decoded pixels. The document asks ImageWidget first and falls
// back to Image, then to alt text. The hook keeps vector formats out of this
// package's dependency graph the same way [Highlighter] keeps chroma out;
// markdown/svgimage provides an SVG implementation backed by vibrantgio/svg.
type WidgetImageProvider interface {
	// ImageWidget returns a widget rendering the image for a markdown
	// destination URL. Returning an error (or a nil widget) falls through
	// to [ImageProvider].Image.
	ImageWidget(url string) (layout.Widget, error)
}

// Style holds the themed rendering defaults for a document. Derive the
// token-themed default with [FromTokens], then set Text.OnLinkClick.
type Style struct {
	// Text is the paragraph default: body colour and size, link and focus
	// colours, and the link callback (richtext.Style.OnLinkClick).
	Text richtext.Style
	// HeadingSizes maps heading levels 1..6 (index 0..5) onto text sizes.
	HeadingSizes [6]unit.Sp
	// Mono is the typeface for inline code and code blocks.
	Mono font.Typeface
	// CodeSize is the text size of code blocks.
	CodeSize unit.Sp
	// CodeColor is the code block text colour.
	CodeColor color.NRGBA
	// CodeBackground fills the code block surface.
	CodeBackground color.NRGBA
	// QuoteBar is the colour of the bar leading a blockquote.
	QuoteBar color.NRGBA
	// QuoteColor is the text colour inside blockquotes.
	QuoteColor color.NRGBA
	// RuleColor is the thematic-break line colour.
	RuleColor color.NRGBA
	// TableBorder is the table grid line colour.
	TableBorder color.NRGBA
	// TableHeaderBackground fills the table header row behind its emphasised
	// cells.
	TableHeaderBackground color.NRGBA
	// CheckboxColor strokes the unchecked task checkbox and, filled, backs
	// the checked one.
	CheckboxColor color.NRGBA
	// CheckmarkColor draws the check mark inside a checked checkbox.
	CheckmarkColor color.NRGBA
	// BlockGap is the vertical space between sibling blocks. It is authored
	// space, not what the reader sees: the shaped lines put their own leading
	// between their ink and the edges of their line boxes, so the blank run in
	// a rendered document is this plus a few pixels. [FromTokens] sizes it so
	// that the sum, not the field, lands on the reading rhythm.
	BlockGap unit.Dp
	// HeadingSpaceAbove is the vertical space above a heading of level 1..6
	// (index 0..5) and HeadingSpaceBelow the space below it. They replace
	// BlockGap on their side rather than adding to it, which is what lets a
	// heading take more air above than an ordinary block gap and less below:
	// a heading then separates from the section it closes and clings to the
	// one it opens. A zero entry falls back to BlockGap, so a hand-built
	// Style that sets neither spaces headings evenly, as it always did.
	//
	// The space above is suppressed for the document's first block, which has
	// nothing to separate from, and halved for a heading directly under
	// another heading, a pair being one announcement rather than two
	// sections.
	//
	// [FromTokens] derives both from the block gap and the type scale; like
	// the gap they are authored space, and the reader sees a little more than
	// these numbers on each side, the shaped lines carrying their own leading
	// above and below the ink.
	HeadingSpaceAbove [6]unit.Dp
	// HeadingSpaceBelow is the vertical space below a heading; see
	// HeadingSpaceAbove.
	HeadingSpaceBelow [6]unit.Dp
	// Indent is the per-level indentation of list items and the inset of
	// blockquote content.
	Indent unit.Dp
	// Gutter is a trailing inset on every top-level block: the prose stops
	// short of the viewport's edge by this much, and nothing is drawn in
	// the strip left over. It is what lets a scrollbar sit where the
	// platform puts one — hard against the pane's edge — while the reading
	// measure keeps its own margin, which would otherwise have to be the
	// same number. Zero, the default, gives the blocks the full width.
	Gutter unit.Dp
	// EndSpace is the space a scrolling document keeps below its last block,
	// on top of whatever that block closes with. It is Gutter's vertical
	// twin, and it is spent at one place rather than on every frame: the
	// viewport stays full while the reader is in the middle of the document,
	// so a line half-way off the trailing edge is the window cutting it and
	// not a margin, and the document scrolled to its end comes to rest with
	// its last line clear of that edge instead of sitting on it.
	//
	// Only [Document.Layout] and [Document.LayoutScrollbar] spend it, because
	// only they own a viewport for the document to rest against. A document
	// laid out with [Document.LayoutColumn] takes exactly its content's
	// height, and the space below its end belongs to whoever is scrolling it.
	//
	// It also fixes where the ends are for everything that moves the document
	// without the pointer: a page move, [Document.ScrollToEnd] and the keys
	// that reach for it all stop at the resting position, and the scrollbar's
	// indicator reaches the end of its track exactly there.
	//
	// Zero, the default, ends the document flush with the viewport.
	EndSpace unit.Dp
	// StartSpace is EndSpace at the other end: the space a scrolling document
	// keeps above its first block, and the reason a viewport may begin hard
	// against whatever chrome stands over it.
	//
	// The document's first block opens with nothing of its own — a heading's
	// space above is suppressed there, having no section to separate from — on
	// the understanding that whatever holds the document puts the air above it.
	// A holder that puts that air outside the viewport buys it at the price of
	// a strip of empty ground over every half-cut line the reader scrolls past,
	// which reads as a clipping fault rather than as scrolling. Spent here
	// instead, the air belongs to the document's start: the viewport reaches
	// the chrome's own edge, a line leaving the top is cut by that edge, and
	// the document scrolled back to its start comes to rest clear of it.
	//
	// Like EndSpace it is content, so the scroll bounds, the page moves and the
	// scrollbar's track all account for it without being told, and only
	// [Document.Layout] and [Document.LayoutScrollbar] spend it — a document
	// laid out with [Document.LayoutColumn] leaves the space above its first
	// block to whoever is scrolling it.
	//
	// Zero, the default, starts the document flush with the viewport.
	StartSpace unit.Dp
	// Highlight, when non-nil, syntax-highlights fenced code blocks.
	// markdown/highlight provides a chroma-backed implementation.
	Highlight Highlighter
	// Images, when non-nil, supplies the pixels for [Image] blocks; without
	// it every image falls back to its alt text. A value that also
	// implements [WidgetImageProvider] can serve vector images as widgets.
	Images ImageProvider
}

// FromTokens derives the default document style from colour tokens and a
// typography: headings take the six stops of the typography's document
// heading scale, body text follows richtext.FromTokens on the BodyLarge
// role, code sits on the Neutral 300
// tinted fill with the low-contrast Neutral 700 text step, the quote bar is
// Primary with Neutral 700 text, rules and table grid lines are separators
// and use Divider, and the table header row sits on the Neutral 300 tinted
// fill. Highlight and Images stay nil — both are opt-in. Pass
// tokens.DefaultTypography for the default look.
//
// Mono and CodeSize come from typo's own Code role — the sixteenth style,
// which sits outside the MD3 grid. Until F3.4 this constructor took a
// tokens.TypeScale, which had no code stop, and so had to read
// tokens.DefaultTypography.Code no matter what typography the theme carried
// (C2.8); a Style shaped for a non-default one had to reset Mono and CodeSize
// afterwards. Taking the whole typography retires that workaround.
//
// The heading sizes come from tokens.DocumentHeadingScale rather than from
// the Headline and Title roles this constructor used to borrow. Those roles
// size the one big line at the top of a screen: against a 16 dp body they run
// 32 down to 14, which inks a document's title a quarter again taller than a
// typeset reading surface inks one — enough to wrap a title that should fit a
// line — while crowding levels three and four onto nearly the same size and
// then dropping a third of the ladder between levels four and five. The
// document scale is stepped off the body role instead, evenly, so six levels
// are six levels.
//
// The block gap and the heading spaces are set from the reading rhythm rather
// than from the smallest stop that separates two widgets: prose read at length
// wants the openness a typeset page has, which is a good deal more air between
// blocks than a form wants between its rows. See blockRhythm and
// [headingSpacing] for the proportions and where they came from.
//
// Of each role only Size lands in the Style: headings and paragraphs carry
// their typeface, weight and slant per span (richtext.SpanStyle), so those
// parts of a role reach the shaper through the document's spans rather than
// through this constructor. Mono is the one typeface a Style names outright,
// because code spans are built from it.
func FromTokens(c tokens.ColorTokens, typo tokens.Typography) Style {
	var sizes [6]unit.Sp
	for i, style := range typo.DocumentHeadings {
		sizes[i] = unit.Sp(style.Size)
	}
	gap := blockRhythm - lineLeading
	above, below := headingSpacing(gap, sizes)
	return Style{
		Text:                  richtext.FromTokens(c, typo.BodyLarge),
		HeadingSizes:          sizes,
		HeadingSpaceAbove:     above,
		HeadingSpaceBelow:     below,
		Mono:                  font.Typeface(typo.Code.Typeface),
		CodeSize:              unit.Sp(typo.Code.Size),
		CodeColor:             c.Ramps.Neutral.Step(700), // low-contrast text
		CodeBackground:        c.Ramps.Neutral.Step(300), // tinted fill
		QuoteBar:              c.Primary,
		QuoteColor:            c.Ramps.Neutral.Step(700), // low-contrast text
		RuleColor:             c.Divider,
		TableBorder:           c.Divider,
		TableHeaderBackground: c.Ramps.Neutral.Step(300), // tinted fill
		CheckboxColor:         c.Primary,
		CheckmarkColor:        c.OnPrimary,
		BlockGap:              gap,
		Indent:                unit.Dp(tokens.Spacing.S6),
	}
}

// The reading rhythm is measured in what the reader sees — the blank run
// between one block's ink and the next's — while a [Style] is written in
// authored space. The difference is the leading the shaped lines carry
// between their ink and the edges of their line boxes, and these two
// constants are that difference, read off rendered blank runs at the document
// heading scale against a 16 dp body.
//
// Neither is exact, because neither can be. A line closing on a descender and
// one opening on an ascender leave several pixels less blank than two that do
// neither, so the same authored space measures anywhere across a range of
// about five pixels depending on the words. Each constant is read where the
// resulting runs centre on the measured reference across mixed prose, not
// where any one pair of lines happens to fall.
const (
	// lineLeading is what a pair of ordinary blocks contributes.
	lineLeading = unit.Dp(3)
	// headingLeading is the same for a transition with a heading on one side.
	// It is the larger of the two because a heading's line box is taller and
	// its leading with it — and because that box swings wider, a heading being
	// a few words rather than a full line of prose.
	headingLeading = unit.Dp(6)
)

// blockRhythm is the blank an ordinary pair of blocks shows the reader: the
// spacing scale's S8 stop plus an S1, which is 2.25 body sizes at the default
// 16 dp body. It is the reader-visible number rather than the authored one
// precisely because that is the quantity a typeset reading surface is set in;
// [FromTokens] authors it less lineLeading, which puts the rendered run on it
// where the facing lines are tallest and a pixel or two over it on average —
// and the average is where the reference's own 37 px sits.
var blockRhythm = unit.Dp(tokens.Spacing.S8 + tokens.Spacing.S1)

// The proportions a heading holds against that rhythm: the space above it is
// a little over a block gap and the space below it about half of the space
// above, so a heading separates from the section it closes and clings to the
// one it opens. Both are proportions of visible blank, not of authored space.
const (
	headingAboveRhythm = 1.35
	headingBelowAbove  = 0.5
)

// headingSpacing derives the space around every heading level from the block
// gap and the type scale. It works in visible blank throughout — the space
// above a heading is headingAboveRhythm ordinary block rhythms, the space
// below it headingBelowAbove of that — and subtracts the shaped lines'
// leading only at the end, when the visible number becomes an authored one.
// Deriving it the other way round is what would break at a different gap: the
// leading is a constant few pixels and the rhythm is not, so a formula that
// mixes them lands on the proportions at one gap and nowhere else.
//
// Level two is the section heading the rhythm was measured at, so it sits
// exactly on the proportions above and the other levels lean off it by their
// own size — which is what makes a deep heading earn slightly less air than a
// shallow one without any level leaving the measured range.
func headingSpacing(gap unit.Dp, sizes [6]unit.Sp) (above, below [6]unit.Dp) {
	if sizes[1] == 0 {
		// No scale to lean off: leave the spaces zero, which is a fall back to
		// the plain block gap on both sides of every heading.
		return above, below
	}
	rhythm := gap + lineLeading
	for i, size := range sizes {
		// Only the part of the space that exceeds an ordinary block gap scales
		// with the level. Scaling the whole of it would run the ladder from
		// half again the rhythm down to well under it, which is a level six
		// that binds to the section above rather than to the one it opens.
		level := float32(size) / float32(sizes[1])
		visible := rhythm * unit.Dp(1+(headingAboveRhythm-1)*level)
		above[i] = visible - headingLeading
		below[i] = visible*headingBelowAbove - headingLeading
	}
	return above, below
}

// compact returns the style with its block rhythm reset to gap: the gap
// itself, and the heading spaces re-derived so they hold their proportions
// against it rather than against the document's own, much wider, rhythm. It is
// what a container whose contents are one block of the reading flow — a list —
// lays its inner blocks out with. A style whose heading spaces were left zero
// keeps them zero, so a hand-built Style still spaces every pair by its gap.
func (s Style) compact(gap unit.Dp) Style {
	s.BlockGap = gap
	if s.HeadingSpaceAbove != ([6]unit.Dp{}) {
		s.HeadingSpaceAbove, s.HeadingSpaceBelow = headingSpacing(gap, s.HeadingSizes)
	}
	return s
}

// headingSpace returns the space above and below a heading of the given level.
// A zero entry — a Style built by hand rather than by [FromTokens] — falls
// back to the ordinary block gap, and the space above never falls below it.
func (s Style) headingSpace(level int) (above, below unit.Dp) {
	above, below = s.BlockGap, s.BlockGap
	if level >= 1 && level <= len(s.HeadingSpaceAbove) {
		if v := s.HeadingSpaceAbove[level-1]; v > 0 {
			above = v
		}
		if v := s.HeadingSpaceBelow[level-1]; v > 0 {
			below = v
		}
	}
	return max(above, s.BlockGap), below
}

// heading returns the richtext paragraph style for a heading of the given
// level: the level's type-scale size with body colours.
func (s Style) heading(level int) richtext.Style {
	st := s.Text
	if level >= 1 && level <= len(s.HeadingSizes) {
		st.Size = s.HeadingSizes[level-1]
	}
	return st
}

// codeSpans maps a code block's content onto richtext spans: highlighted
// runs when the style's Highlighter recognises the language, one plain run
// otherwise. Newlines opening a highlighted run (chroma's whitespace tokens
// lead with them) are moved to the previous run's tail: richtext treats a
// trailing newline as a clean line end, while a leading one would skew the
// line's metrics.
func (s Style) codeSpans(cb *CodeBlock) []richtext.SpanStyle {
	if s.Highlight != nil {
		if hl := s.Highlight(cb.Language, cb.Code); len(hl) > 0 {
			out := make([]richtext.SpanStyle, 0, len(hl))
			for _, h := range hl {
				content := h.Text
				if len(out) > 0 {
					i := 0
					for i < len(content) && content[i] == '\n' {
						i++
					}
					out[len(out)-1].Content += content[:i]
					content = content[i:]
				}
				if content == "" {
					continue
				}
				rs := richtext.SpanStyle{Content: content, Color: h.Color, Typeface: s.Mono}
				if h.Bold {
					rs.Weight = font.Bold
				}
				if h.Italic {
					rs.Style = font.Italic
				}
				out = append(out, rs)
			}
			return out
		}
	}
	return []richtext.SpanStyle{{Content: cb.Code, Typeface: s.Mono}}
}

// spanStyles maps model spans onto richtext spans against the style's
// typefaces. defWeight is the run weight for spans without their own bold
// flag (font.Bold for headings).
func (s Style) spanStyles(spans []Span, defWeight font.Weight) []richtext.SpanStyle {
	out := make([]richtext.SpanStyle, 0, len(spans))
	for _, sp := range spans {
		rs := richtext.SpanStyle{
			Content:       sp.Text,
			URL:           sp.URL,
			Strikethrough: sp.Strikethrough,
			Weight:        defWeight,
		}
		if sp.Bold {
			rs.Weight = font.Bold
		}
		if sp.Italic {
			rs.Style = font.Italic
		}
		if sp.Code {
			rs.Typeface = s.Mono
		}
		out = append(out, rs)
	}
	return out
}
