package markdown

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/components/richtext"
	"github.com/vibrantgio/components/scrollbar"
	themecolor "github.com/vibrantgio/theme/color"
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
//
// # Paper
//
// The surface a Style describes has a name: paper. It is the quiet ground
// running text is read on, and it is not chrome — chrome being the furniture
// a screen is assembled from, the rails and bars and cards and controls that
// answer to the theme directly. Paper answers to the theme too, but through
// roles of its own, and the four that make it paper are:
//
//   - [Style.Paper], the ground the document is read on;
//   - [Style.Text]'s colours, the prose inks — the body, its links, its focus
//     ring;
//   - [Style.HeadingSizes] with [Style.HeadingLineHeights], the heading
//     ladder a document is broken up by;
//   - [Style.CodeChip], the fill under a word of code quoted into a sentence.
//
// The rest of the fields dress the blocks standing on that paper — a fence, a
// quote, a rule, a table, a task box — and they are paper's as well: every
// colour this package draws comes from a field of this struct and from
// nowhere else. The layout code reads the theme for spacing and for radii and
// for no colour at all, so a document looks like what its Style says and
// nothing reaches around it.
//
// The roles are paper's own even where the values are chrome's today.
// [FromTokens] takes the ground from the theme's page, the prose ink from its
// body text pin, the chip from the neutral step a fence is filled with — the
// same numbers a card or a toolbar would reach for, held here in paper's own
// name. Naming them separately costs nothing now and is what lets the reading
// surface move later without the furniture moving with it, or the other way
// about. Deriving a paper role from a theme token is not chrome leaking onto
// the page; drawing a document with a token instead of with a role would be,
// and nothing here does.
type Style struct {
	// Paper is the ground the document is read on: what lies behind the
	// prose, under every block, out to the edges of whatever holds it.
	//
	// Nothing in this package paints it. A document is laid out into a space
	// somebody else owns, and that owner fills the ground — so this is a
	// record rather than a draw, the Style's statement of what the document
	// is lying on. It is what the library measures against when something on
	// the page has to be told apart from the page: a fence wearing a syntax
	// palette takes its edge from exactly that question (see
	// markdown/highlight), and "too near the page to be seen against it" is
	// unanswerable without knowing which page.
	//
	// [FromTokens] sets it to the theme's own background, which is where a
	// document nearly always lies. A holder that mounts one somewhere else
	// says so here, and the measurements follow the ground the reader is
	// actually looking at. The zero value states nothing, and what needs an
	// answer falls back to the theme's background.
	Paper color.NRGBA
	// Text is the paragraph default: body colour and size, link and focus
	// colours, and the link callback (richtext.Style.OnLinkClick). Its
	// colours are paper's prose inks.
	Text richtext.Style
	// HeadingSizes maps heading levels 1..6 (index 0..5) onto text sizes: the
	// ladder paper ranks its sections by, which is a reading ladder and not
	// the roles that size the one big line at the top of a screen.
	HeadingSizes [6]unit.Sp
	// HeadingLineHeights maps the same levels onto the line box each heading's
	// lines occupy, the way [richtext.Style].LineHeight means it. A zero entry
	// — a Style built by hand rather than by [FromTokens] — leaves that level's
	// lines on their shaped metrics.
	HeadingLineHeights [6]unit.Sp
	// Mono is the typeface for inline code and code blocks.
	Mono font.Typeface
	// CodeSize is the text size of code blocks.
	CodeSize unit.Sp
	// CodeColor is the code block text colour: what plain code is set in, and
	// what a highlighted run with no colour of its own falls back to. A fence
	// dressed in a syntax palette takes that palette's own body ink here, so
	// the runs its author left plain are the ones they drew plain.
	CodeColor color.NRGBA
	// CodeBackground is the fenced block's ground. [FromTokens] gives it the
	// theme's own step off the page — a near-white in a light scheme and a
	// near-black in a dark one, because a fence is a panel inset into the page
	// and a panel separates by a step, not by a drop.
	//
	// It is a field rather than a constant because a fence may be dressed in a
	// syntax palette instead, and a palette is a ground and a set of inks
	// together: put the inks on a ground their author never drew them against
	// and the relations between them stop being the ones that were chosen.
	// Then this holds that author's own background, CodeColor their own body
	// ink, and CodeBorder whatever it takes to keep the result an island.
	CodeBackground color.NRGBA
	// CodeBorder strokes a hairline just inside the fence's rounded edge. The
	// zero value — zero alpha — draws none, which is what a ground that stands
	// off the page on its own needs.
	//
	// It is for the ground that does not. A syntax palette fitted to paper is
	// drawn on a near-white, and this page is a near-white too: laid on it
	// unbounded, the block stops being a block and the code reads as a
	// paragraph in a monospace face. The line is what says where the fence is
	// when its fill no longer does.
	CodeBorder color.NRGBA
	// CodeChip fills the rounded chip an inline code span sits on. A zero
	// alpha — a Style built by hand rather than by [FromTokens] — sets inline
	// code on the page itself, as it always was, and the span is still set in
	// Mono at the code size.
	//
	// [FromTokens] gives it and CodeBackground the same value: quoted code is
	// one surface, whether a word of it is set in a sentence or a screenful of
	// it is set apart, which is how the reading surface this library is judged
	// against draws it.
	//
	// The two part company as soon as a fence is dressed in a syntax palette,
	// and deliberately: a page of prose spotted with somebody else's grounds
	// would be a page arguing with itself, so a chip stays on the quiet fill
	// and in the body's own ink while the block down the page shows the
	// palette whole. That is a departure from the reading surface, which puts
	// one fill under both, and it is the chip that keeps the old behaviour.
	CodeChip color.NRGBA
	// CodeScrollbar styles the slim horizontal bar a code block whose widest
	// line overflows the column shows while it scrolls. It sits in the
	// fence's bottom padding, over no code, and it is absent — like every
	// overlay bar on the desktop — while the block fits or rests.
	//
	// The zero value draws no bar. The dissolve at the cut edge is not the
	// bar's and stays either way, so a document that turns the bar off still
	// says that there is more; what it loses is the drag and the sense of how
	// much more.
	CodeScrollbar scrollbar.Style
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
	// ListSpaceAbove is the vertical space at the one seam a list makes with a
	// paragraph directly above it. A paragraph running straight into a list is
	// announcing it — the two are one statement — so an ordinary block gap
	// there reads as a break between them and this tighter space closes it.
	// Like the heading spaces it replaces BlockGap at that seam rather than
	// adding to it, and a zero value falls back to BlockGap, so a hand-built
	// Style spaces a list from the paragraph above it the way it spaces any
	// other pair of blocks.
	//
	// The rule is structural rather than punctuational: every list directly
	// below a paragraph takes it, whether or not that paragraph ends in a
	// colon. A colon test would miss the announcing sentences that carry none
	// and fire on the colons that announce nothing.
	//
	// Nothing else moves. A list below a heading, below another list, first in
	// its container, or nested inside a list item keeps the space it had, and
	// so does the seam below a list back to ordinary blocks: the reference this
	// was measured from shows no list with a paragraph under it, so that side
	// stays at the ordinary gap rather than being guessed at.
	//
	// [FromTokens] derives it from the block rhythm; like the gap it is
	// authored space, and the reader sees a little more than the number, the
	// shaped lines carrying their own leading above and below the ink.
	ListSpaceAbove unit.Dp
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
// typography: the paper is the theme's own background, headings take the six
// stops of the typography's document
// heading scale, body text follows richtext.FromTokens on the BodyLarge
// role, code sits on the Neutral 200 surface — one step off the page in either
// scheme — with the low-contrast Neutral 700 text step, inline code on the same
// step while keeping the body's own ink so a quoted word reads as the
// sentence's, the quote bar is Primary with Neutral 700 text, rules and table
// grid lines are separators and use Divider, and the table header row sits on
// the Neutral 300 tinted fill. Highlight and Images stay nil — both are opt-in.
// Pass tokens.DefaultTypography for the default look.
//
// Every one of those is a role of paper's, derived from the theme rather than
// borrowed from it — see the [Style] doc comment for the distinction and what
// keeping it is worth. The ground is the theme's background because that is
// where a document lies, and a holder that mounts one somewhere else says so
// by setting Paper afterwards: this constructor answers for the theme and not
// for the composition.
//
// The code surface is one step and not three. A fence covers a good deal of
// the column, and area amplifies a fill: the tinted-fill step that reads as a
// tint behind a table's header row reads, spread under a screenful of code, as
// a slab of grey with the page showing white around it — worst in a light
// scheme, where it also leaves the code's own ink barely over its floor. The
// step measured off the reading application this library is judged against is
// gentler still: a code surface 3.4 L* off its page, against the 4.9 and 5.0
// this step gives in the light and dark schemes. What that measurement also
// says is that a fence and an inline chip are one surface there, not two, so
// they are one here.
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
// The block gap, the heading spaces and the announcing seam are set from the
// reading rhythm rather than from the smallest stop that separates two widgets:
// prose read at length wants the openness a typeset page has, which is a good
// deal more air between blocks than a form wants between its rows. See
// blockRhythm, [headingSpacing] and listSeam for the proportions and where they
// came from.
//
// Of each role only Size lands in the Style: headings and paragraphs carry
// their typeface, weight and slant per span (richtext.SpanStyle), so those
// parts of a role reach the shaper through the document's spans rather than
// through this constructor. Mono is the one typeface a Style names outright,
// because code spans are built from it.
func FromTokens(c tokens.ColorTokens, typo tokens.Typography) Style {
	var sizes [6]unit.Sp
	var boxes [6]unit.Sp
	for i, style := range typo.DocumentHeadings {
		sizes[i] = unit.Sp(style.Size)
		boxes[i] = unit.Sp(style.LineHeight)
	}
	gap := blockRhythm - lineLeading
	above, below := headingSpacing(gap, sizes)
	return Style{
		Paper:                 c.Background, // the ground a document lies on
		Text:                  richtext.FromTokens(c, typo.BodyLarge),
		HeadingSizes:          sizes,
		HeadingLineHeights:    boxes,
		HeadingSpaceAbove:     above,
		HeadingSpaceBelow:     below,
		Mono:                  font.Typeface(typo.Code.Typeface),
		CodeSize:              unit.Sp(typo.Code.Size),
		CodeColor:             codeInk(c),                // see codeInk
		CodeBackground:        c.Ramps.Neutral.Step(200), // the step off the page
		CodeChip:              c.Ramps.Neutral.Step(200), // one code surface, not two
		CodeScrollbar:         codeScrollbar(c),
		QuoteBar:              c.Primary,
		QuoteColor:            c.Ramps.Neutral.Step(700), // low-contrast text
		RuleColor:             c.Divider,
		TableBorder:           c.Divider,
		TableHeaderBackground: c.Ramps.Neutral.Step(300), // tinted fill
		CheckboxColor:         c.Primary,
		CheckmarkColor:        c.OnPrimary,
		BlockGap:              gap,
		ListSpaceAbove:        listSeam(gap),
		Indent:                unit.Dp(tokens.Spacing.S6),
	}
}

// codeInk is what plain code is set in: the runs a highlighter leaves
// colourless, and the whole of a block nothing highlights.
//
// It is the one colour in this constructor the two appearances take a different
// ramp step for, and the reason is measured. A ramp's steps are paired scales —
// the same step does the same job in both appearances — but contrast is not
// linear in them, and at the text end of the ramp the pairing stops holding.
// The dark ramp's low-contrast text step inks code at 9.91:1 on the fence's
// fill, 80% of the way from that fill to the weight the same document's prose
// is set at; the light ramp's same step inks it at 5.46:1 and 58%. Code set
// against paper was therefore a third quieter, relative to its own page, than
// the identical document's code set against slate — which is what a screenful
// of light-scheme code reads as: washed.
//
// One step further along the light ramp lands at 8.20:1 and 70%, which is as
// near the dark scheme's relationship as the ramp goes: the step after it is
// the prose colour itself, and code is not prose. So the light appearance takes
// the step below the body text and the dark one keeps the low-contrast text
// step, and the two schemes set code at 70% and 80% of their own prose weight
// where they set it at 58% and 80%.
func codeInk(c tokens.ColorTokens) color.NRGBA {
	if darkScheme(c) {
		return c.Ramps.Neutral.Step(700) // low-contrast text
	}
	return c.Ramps.Neutral.Step(800) // the step below body text
}

// darkScheme reports which appearance these tokens describe. A ColorTokens
// value carries no flag saying which of the two it is, so the page itself is
// the fact — read on the perceptual lightness axis rather than a luma sum,
// because mid-grey is perceptually mid and a luma threshold calls it dark.
func darkScheme(c tokens.ColorTokens) bool {
	l, _, _ := themecolor.OKLChFromNRGBA(c.Background)
	return l < 0.5
}

// codeScrollbar is the design system's bar weighted for the ground a fence
// puts it on. Everything else about it — the width, the radius, the minimum
// thumb, the fade a second after the content stops — is the shared style's.
//
// Two things change, and the same fact drives both. scrollbar.FromTokens tunes
// its thumb for a scrolling column, which rests on the page: the
// low-contrast-text step at about 40%, three ramp stops from the ground it
// lies on, reads clearly there. A fence's bar rests on the code surface
// instead, one step off that page, and over so little separation the
// translucency all but disappears — about 1.5:1 in the light scheme, which
// leaves the one affordance that can be dragged effectively invisible.
//
// So the thumb is opaque, and it is the ramp's low-contrast text step: as
// present against the fence's fill as text on it would be, and it darkens to
// the ramp's far end while hovered or dragged. That is a weight against a
// ground and not a match to the code's own ink, which is why it stays on this
// step in both appearances while the light appearance's code sits one step past
// it (see codeInk) — the bar lies on the code surface, it is not a run of code,
// and a fence's one draggable affordance does not get heavier because the
// reading got heavier. The translucency it gives up is the
// shared bar's identity because a column's bar lies over the column's own
// text; a fence's lies over the fence's bottom padding, where nothing shows
// through it either way.
func codeScrollbar(c tokens.ColorTokens) scrollbar.Style {
	s := scrollbar.FromTokens(c)
	s.ThumbColor = c.Ramps.Neutral.Step(700)      // the code's own step
	s.ThumbHoverColor = c.Ramps.Neutral.Step(900) // the ramp's far end
	return s
}

// The reading rhythm is measured in what the reader sees — the blank run
// between one block's ink and the next's — while a [Style] is written in
// authored space. The difference is the leading the shaped lines carry
// between their ink and the edges of their line boxes, and these constants are
// that difference, read off rendered blank runs at the document heading scale
// against a 16 dp body.
//
// None of them is exact, because none can be. A line closing on a descender
// and one opening on an ascender leave several pixels less blank than two that
// do neither, so the same authored space measures anywhere across a range of
// about five pixels depending on the words. Each constant is read where the
// resulting runs centre on the measured reference across mixed prose, not
// where any one pair of lines happens to fall.
//
// They are the size they are because a line box is the type role's, not the
// glyphs'. A 16 dp body line inks 16 px inside a 24 px box, and the 8 px left
// over is what a pair of ordinary blocks shows the reader on top of the space
// between them. Under a box left to the shaped metrics the same pair showed
// three, which is the whole of why every number here moved when the boxes
// arrived.
const (
	// lineLeading is what a pair of ordinary blocks contributes.
	lineLeading = unit.Dp(8)
	// headingLeadingAbove is the same for the transition into a heading: a
	// body line's leading below its ink, then a heading line's above its own.
	headingLeadingAbove = unit.Dp(6)
	// headingLeadingBelow is the transition out of one, which is the other two
	// halves — a heading's leading below its ink and a body line's above its
	// own. It is the wider of the pair, a heading's box being the taller and
	// the deeper of its two halves the one that faces this way.
	headingLeadingBelow = unit.Dp(8)
)

// blockRhythm is the blank an ordinary pair of blocks shows the reader: the
// spacing scale's S8 stop plus an S1, which is 2.25 body sizes at the default
// 16 dp body. It is the reader-visible number rather than the authored one
// precisely because that is the quantity a typeset reading surface is set in;
// [FromTokens] authors it less lineLeading, which lands the rendered run
// within a pixel of it across mixed prose — and that is where the reference's
// own 37 px sits. The swing is small now that a line occupies its role's box
// rather than its glyphs': the box edges are fixed, so only where the ink sits
// inside them still varies with the words.
var blockRhythm = unit.Dp(tokens.Spacing.S8 + tokens.Spacing.S1)

// The proportions a heading holds against that rhythm: the space above it is
// a little over a block gap and the space below it about half of the space
// above, so a heading separates from the section it closes and clings to the
// one it opens. Both are proportions of visible blank, not of authored space.
const (
	headingAboveRhythm = 1.35
	headingBelowAbove  = 0.5
)

// listSeamRhythm is the share of an ordinary block rhythm the seam between a
// paragraph and the list it announces keeps: two thirds. It is read off the
// same reference reading surface as the rest of the rhythm, where an ordinary
// pair of blocks shows about 37 px of blank at a 16 dp body and a paragraph
// with a list under it shows 25 — a seam a reader sees as a join rather than
// as a break, without the list crowding the line that introduces it.
const listSeamRhythm = 2.0 / 3.0

// listSeam derives the announcing seam from the block gap. Like
// [headingSpacing] it works in visible blank throughout — the seam is
// listSeamRhythm of an ordinary block rhythm — and turns the visible number
// into an authored one only at the end, by subtracting the leading the shaped
// lines carry. Taking the proportion of the authored gap instead would land on
// the measurement at this gap and nowhere else, the leading being a constant
// few pixels while the rhythm is not.
func listSeam(gap unit.Dp) unit.Dp {
	return (gap+lineLeading)*listSeamRhythm - lineLeading
}

// headingSpacing derives the space around every heading level from the block
// gap and the type scale. It works in visible blank throughout — the space
// above a heading is headingAboveRhythm ordinary block rhythms, the space
// below it headingBelowAbove of that — and subtracts the shaped lines'
// leading only at the end, when the visible number becomes an authored one.
// Deriving it the other way round is what would break at a different gap: the
// leading is a constant few pixels and the rhythm is not, so a formula that
// mixes them lands on the proportions at one gap and nowhere else.
//
// The two sides subtract different leadings because they are different pairs
// of halves. Above a heading the reader sees a body line's leading below its
// ink and then a heading line's above its own; below one it is the heading's
// lower half over a body line's upper. Those are four different quantities,
// and one number for both sides lands on the reference on one side only.
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
		above[i] = visible - headingLeadingAbove
		below[i] = visible*headingBelowAbove - headingLeadingBelow
	}
	return above, below
}

// compact returns the style with its block rhythm reset to gap: the gap
// itself, and the heading spaces re-derived so they hold their proportions
// against it rather than against the document's own, much wider, rhythm. It is
// what a container whose contents are one block of the reading flow — a list —
// lays its inner blocks out with. A style whose heading spaces were left zero
// keeps them zero, so a hand-built Style still spaces every pair by its gap.
//
// The announcing seam does not survive the compacting, and unlike the heading
// spaces it is not re-derived either. It is a correction to the reading rhythm
// — the air between two blocks a reader takes in one after the other — and
// inside a container that rhythm has already been spent: the compact stop binds
// an item's own blocks tightly enough that the line above a sub-list needs
// nothing done to it. Carrying the number across would leave the seam wider
// than the gap it is meant to tighten; re-deriving it would close a seam
// nothing was measured at.
func (s Style) compact(gap unit.Dp) Style {
	s.BlockGap = gap
	if s.HeadingSpaceAbove != ([6]unit.Dp{}) {
		s.HeadingSpaceAbove, s.HeadingSpaceBelow = headingSpacing(gap, s.HeadingSizes)
	}
	s.ListSpaceAbove = 0
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

// listSpace returns the space at the seam between a paragraph and the list it
// announces. A zero field — a Style built by hand rather than by [FromTokens] —
// falls back to the ordinary block gap, which is what that seam always had.
func (s Style) listSpace() unit.Dp {
	if s.ListSpaceAbove > 0 {
		return s.ListSpaceAbove
	}
	return s.BlockGap
}

// heading returns the richtext paragraph style for a heading of the given
// level: the level's type-scale size and line box with body colours. A level
// with no line box of its own falls back to the body's, which is the smallest
// box any of them asks for and so cannot squeeze a heading's own metrics.
func (s Style) heading(level int) richtext.Style {
	st := s.Text
	if level >= 1 && level <= len(s.HeadingSizes) {
		st.Size = s.HeadingSizes[level-1]
		st.LineHeight = s.HeadingLineHeights[level-1]
		if st.LineHeight == 0 {
			st.LineHeight = s.Text.LineHeight
		}
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

// The chip an inline code span sits on, read off a reference reading surface
// set at a 16 px body: 4 px of clear space between the code and each end of
// the fill, and a 4 px radius on a chip 20 px tall. Both land on the theme's
// own first stops, so the chip is drawn in the design system's units rather
// than in the measurement's, and both scale with the reader's density because
// they are dp.
var (
	codeChipPad    = unit.Dp(tokens.Spacing.S1)
	codeChipRadius = unit.Dp(tokens.Radius.Base)
)

// codeSize is the size inline code takes in a line set at size.
//
// Code is set below the prose around it — the reference reading surface sets
// an inline span at the same size as a fence, seven eighths of its body — and
// that is what keeps a line holding code the height of a line without it: at
// the body's own size the monospace face asks for more ascent than the body
// face does, which drops the line's shared baseline out from under everything
// hung beside it, a list's markers first of all.
//
// The proportion travels rather than the number: a code span in a heading
// takes the same fraction of the heading's size. Setting it at the fence's 14
// there would read as a footnote dropped into a title, and one line of a
// document would be sized by another line's face.
//
// A style carrying no code size — built by hand rather than by [FromTokens] —
// leaves the span at the line's own size, which is what it always had.
func (s Style) codeSize(size unit.Sp) unit.Sp {
	if s.CodeSize == 0 || s.Text.Size == 0 {
		return size
	}
	return size * s.CodeSize / s.Text.Size
}

// spanStyles maps model spans onto richtext spans against the style's
// typefaces. defWeight is the run weight for spans without their own bold
// flag (font.Bold for headings), and size is the size the line is set at,
// which inline code is sized against.
func (s Style) spanStyles(spans []Span, defWeight font.Weight, size unit.Sp) []richtext.SpanStyle {
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
			rs.Size = s.codeSize(size)
			rs.Chip = richtext.Chip{
				Color:   s.CodeChip,
				Padding: codeChipPad,
				Radius:  codeChipRadius,
			}
		}
		out = append(out, rs)
	}
	return out
}
