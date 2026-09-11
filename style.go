package markdown

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/components/paragraph"
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
// [layout.Widget] — vector geometry that stays crisp at any scale and pixel
// density — instead of decoded pixels. The document asks ImageWidget first
// and falls back to Image, then to alt text. The hook keeps vector formats
// out of this package's dependency graph the same way [Highlighter] keeps
// chroma out;
// markdown/svgimage provides an SVG implementation backed by vibrantgio/svg.
type WidgetImageProvider interface {
	// ImageWidget returns a [layout.Widget] rendering the image for a markdown
	// destination URL. Returning an error (or a nil one) falls through
	// to [ImageProvider].Image.
	ImageWidget(url string) (layout.Widget, error)
}

// Style holds the themed rendering defaults for a document. Derive the
// token-themed default with [FromTokens], then set Text.OnLinkClick and,
// for a reader that writes GFM task markers, OnTaskClick.
//
// # The content
//
// A Style describes a document standing on the content: the plain surface
// running text is read on, distinct from the chrome — the rails, bars and
// controls the window is framed with. Four fields make it that:
//
//   - [Style.ContentSurface], the surface the document is read on;
//   - [Style.Text]'s colours, the prose foregrounds — the body, its links
//     and its focus ring;
//   - [Style.HeadingSizes] with [Style.HeadingLineHeights], the heading
//     scale a document is broken up by;
//   - [Style.CodeChip], the fill under a word of code quoted into a sentence.
//
// The rest of the fields dress the blocks standing on the content — a fence, a
// quote, a rule, a table, a task box.
//
// The invariant: every colour this package draws comes from a field of this
// struct and from nowhere else. The layout code reads the theme for spacing
// and for radii and for no colour at all, so a document looks like what its
// Style says and nothing reaches around it.
type Style struct {
	// ContentSurface is the surface the document is read on: what lies behind
	// the prose, under every block, out to the edges of whatever holds it.
	//
	// Nothing in this package paints it. A document is laid out into a space
	// somebody else owns, and that owner fills it — so this is a
	// record rather than a draw, the Style's statement of what the document
	// is lying on.
	//
	// Nothing in the library reads this field: a code surface's edge is
	// derived against the fill it encloses rather than against the page. It
	// stays a record because a holder that mounts a document somewhere
	// unusual should still be able to say so.
	//
	// [FromTokens] sets it to the theme's own background, which is where a
	// document nearly always lies.
	ContentSurface color.NRGBA
	// Text is the paragraph default: body colour and size, link and focus
	// colours, and the link callback (paragraph.Style.OnLinkClick). Its
	// colours are the content's prose foregrounds.
	Text paragraph.Style
	// HeadingSizes maps heading levels 1..6 (index 0..5) onto text sizes: the
	// scale the content ranks its sections by, which is a reading scale and not
	// the roles that size the one big line at the top of a screen.
	HeadingSizes [6]unit.Sp
	// HeadingLineHeights maps the same levels onto the line box each heading's
	// lines occupy, the way [paragraph.Style].LineHeight means it. A zero entry
	// — a Style built by hand rather than by [FromTokens] — leaves that level's
	// lines on their shaped metrics.
	HeadingLineHeights [6]unit.Sp
	// Mono is the typeface for inline code and code blocks.
	Mono font.Typeface
	// CodeSize is the text size of code blocks.
	CodeSize unit.Sp
	// CodeColor is the code block text colour: what plain code is set in, and
	// what a highlighted run with no colour of its own falls back to. A fence
	// dressed in a syntax palette takes that palette's own body colour here, so
	// the runs its author left plain are the ones they drew plain.
	CodeColor color.NRGBA
	// CodeBackground is the fenced block's fill. [FromTokens] gives it the
	// platform's alternating content fill over the page — the one small step
	// the platform gives content off its own plane, darker than a light page
	// and lighter than a dark one.
	//
	// It is a field rather than a constant because a fence may be dressed in a
	// syntax palette instead, and a palette is a background and a set of
	// colours together: put the colours on a background their author never drew
	// them against and the relations between them stop being the ones that were
	// chosen.
	// Then this holds that author's own background, CodeColor their own body
	// colour, and CodeBorder whatever it takes to keep the result an island.
	CodeBackground color.NRGBA
	// CodeBorder strokes a hairline just inside the fence's rounded edge. The
	// zero value — zero alpha — draws none, which is what a fill that stands
	// off the page on its own needs.
	//
	// It is for the fill that does not, which is every fill a fence
	// takes. A step of fill says a fence stands apart; it does not say where
	// the fence ENDS, and a block laid unbounded on a page a whisper away
	// from it stops being a block — the code reads as a paragraph in a
	// monospace face. A syntax palette fitted to a light page puts its own
	// near-white in that position. The line is what says where the fence is,
	// and [FromTokens] lays the platform's seam over whatever fill it is
	// edging (see codeRim).
	CodeBorder color.NRGBA
	// CodeChip fills the rounded chip an inline code span sits on. A zero
	// alpha — a Style built by hand rather than by [FromTokens] — sets inline
	// code on the page itself, and the span is still set in Mono at the code
	// size.
	//
	// [FromTokens] gives it and CodeBackground the same value: quoted code is
	// one surface, whether a word of it is set in a sentence or a screenful of
	// it is set apart, which is how the reading surface this library is judged
	// against draws it.
	//
	// The two part company as soon as a fence is dressed in a syntax palette,
	// and deliberately: a page of prose spotted with somebody else's
	// backgrounds would be a page arguing with itself, so a chip stays on the
	// theme's own fill and in the body's own colour while the block down the
	// page shows the palette whole.
	CodeChip color.NRGBA
	// CodeChipBorder strokes a hairline just inside the chip's rounded edge,
	// as [Style.CodeBorder] does for the fence. A zero alpha draws none.
	//
	// The chip and the fence are one construct at two sizes, so they take one
	// fill, and that fill is a whisper off the content — which a fence
	// survives, having a rim and a radius and a screenful of area to be
	// recognised by, and a word of code does not. A hue is not available as
	// an answer: the platform's coloured names all say something — the accent
	// says this is the action, the system colours say what the state is — and
	// code says none of them. So the chip takes the fence's answer at the
	// chip's own size: the same fill and the same seam.
	//
	// It is separate from CodeBorder for the same reason CodeChip is separate
	// from CodeBackground: a fence dressed in a syntax palette takes that
	// palette's background and the edge that background calls for, while the
	// chip stays on the theme's own fill and keeps the theme's own rim.
	CodeChipBorder color.NRGBA
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
	// CheckboxBorder strokes the box of an unchecked task item. Nothing is
	// painted inside it, so the stroke lies straight on [Style.ContentSurface] and is
	// the whole of what says there is a task here and it is open — a graphic
	// carrying meaning without being text, owing its page WCAG 1.4.11's 3:1.
	//
	// CheckboxFill is the same box in the other state and is a separate
	// field because the two are drawn on opposite surfaces: this one on the
	// page, that one over it. One colour can only serve both while it happens
	// to read on both, which is a property of the brand a Style was derived
	// from and not of this package. See [FromTokens].
	CheckboxBorder color.NRGBA
	// CheckboxFill fills the box of a checked task item, wall to wall, with
	// [Style.CheckmarkColor]'s tick drawn over it. It is a filled mark and
	// not a foreground on the page: what it owes contrast to is the tick it
	// carries, not the surface it covers, so it is entitled to be the brand's
	// own colour at the brand's own depth.
	CheckboxFill color.NRGBA
	// CheckmarkColor draws the check mark inside a checked checkbox. Its
	// surface is CheckboxFill rather than [Style.ContentSurface] — the fill covers the
	// box before the tick goes on — so it is a colour chosen against that
	// fill, and a Style that moves the fill has to move this with it.
	CheckmarkColor color.NRGBA
	// MatchFill marks a match of the document's find query, painted behind
	// the matched glyphs on the line's box and under the text, in every kind
	// of block the query is searched in. Nothing is marked until a query is
	// set; see [Document.Find].
	//
	// [FromTokens] takes the theme's highlight laid over
	// [Style.ContentSurface], so a Style that moves that surface has to move
	// this with it.
	MatchFill color.NRGBA
	// CurrentMatchFill marks the one match the caller is on, so the reader
	// can tell it from the others while stepping through them.
	//
	// [FromTokens] takes the same yellow over the same surface at a higher
	// coverage: the current match is the fill the rest wear laid on more
	// strongly, not a second colour.
	CurrentMatchFill color.NRGBA
	// ArrivalFill marks the heading a followed heading word took the reader
	// to: the block's own box filled under its content, showing at once and
	// fading by itself moments later. It is the document's own marking, not
	// the caller's — see [Document.Highlight] for that one — because the
	// document made the move.
	//
	// [FromTokens] takes the theme's highlight laid over
	// [Style.ContentSurface], the same fill a match wears: one highlight,
	// whatever brought the reader.
	ArrivalFill color.NRGBA
	// BlockGap is the vertical space between sibling blocks. It is authored
	// space, not what the reader sees: the shaped lines put their own leading
	// between their glyphs and the edges of their line boxes, so the blank run in
	// a rendered document is this plus a few pixels. [FromTokens] sizes it so
	// that the sum, not the field, lands on the reading rhythm.
	BlockGap unit.Dp
	// HeadingSpaceAbove is the vertical space above a heading of level 1..6
	// (index 0..5) and HeadingSpaceBelow the space below it. They replace
	// BlockGap on their side rather than adding to it, which is what lets a
	// heading take more air above than an ordinary block gap and less below:
	// a heading then separates from the section it closes and clings to the
	// one it opens. A zero entry falls back to BlockGap, so a hand-built
	// Style that sets neither spaces headings evenly.
	//
	// The space above is suppressed for the document's first block, which has
	// nothing to separate from, and halved for a heading directly under
	// another heading, a pair being one announcement rather than two
	// sections.
	//
	// [FromTokens] derives both from the block gap and the type scale; like
	// the gap they are authored space, and the reader sees a little more than
	// these numbers on each side, the shaped lines carrying their own leading
	// above and below the glyphs.
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
	// shaped lines carrying their own leading above and below the glyphs.
	ListSpaceAbove unit.Dp
	// Indent is the per-level indentation of list items and the inset of
	// blockquote content.
	Indent unit.Dp
	// Measure is the width a top-level block may reach. When the region is
	// wider, every block lays out at this width and the run of them is
	// centred in it: nothing widens to fill, and content that keeps its own
	// size — a code block, a table — scrolls inside the measure rather than
	// past it. A region narrower than the measure gives the blocks what
	// there is, which is what every document had before this field existed.
	//
	// Long lines tire the reader, which is the whole of it: a window dragged
	// out to the width of a screen should give the reader more of the
	// document, not longer lines to walk back along.
	//
	// Gutter is not part of it and is not centred with it. The gutter is the
	// strip a scrollbar sits in at the viewport's own trailing edge, so it
	// comes off first and the measure is centred in what the viewport shows
	// rather than in what the gutter leaves. The two rules only meet when
	// the measure is within a gutter's width of the viewport itself: centring
	// would then run a block under the bar, so the run is seated as close to
	// centre as the gutter allows and no closer.
	//
	// Only [Document.Layout] and [Document.LayoutScrollbar] spend it, as they
	// alone spend Gutter, StartSpace and EndSpace: a document laid out with
	// [Document.LayoutColumn] is a passage inside somebody else's
	// composition, and the width it is read at is theirs to set.
	//
	// Zero, the default, gives the blocks the full width.
	Measure unit.Dp
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
	// a strip of empty page over every half-cut line the reader scrolls past,
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
	// implements [WidgetImageProvider] can serve vector images as
	// [layout.Widget]s.
	Images ImageProvider
	// OnTaskClick is called when a GFM task checkbox is activated by pointer
	// click or by Space/Enter while focused. The argument is the *[ListItem]
	// [Parse] produced — the same pointer, so the caller can find it in the
	// tree. The gtx is the layout.Context active on the frame the activation
	// is processed, so a caller may add ops to gtx.Ops inside the callback.
	//
	// Nil, the default, leaves every checkbox display-only: no pointer ops,
	// no visual change.
	OnTaskClick func(gtx layout.Context, item *ListItem)
}

// FromTokens derives the default document style from the platform's colour
// set and a typography.
//
// standsOn is the opaque fill the document is read on — the caller's surface.
// Unstated (a zero alpha), it is the platform's text background, which is what
// a document lies on. Every coverage-carrying name below is flattened onto that
// fill in encoded sRGB, the space the platform composites an alpha name in, so
// what this constructor hands the rasterizer is opaque.
//
// The mapping, name by name:
//
//   - the prose paragraph.FromTokens on the BodyLarge role, so body text is
//     Text and a link is Link with the pointing hand;
//   - code, block and chip alike, on AlternatingContentBackground — the
//     platform's second content fill, a small step off the page, darker in
//     light and lighter in dark — inside a Separator hairline, set in Text;
//   - the quote bar and an open task's box TertiaryLabel, quoted prose
//     SecondaryLabel;
//   - a thematic break Separator, a table's grid lines Grid, its header band
//     the same step a fence takes;
//   - a completed task's box ControlAccent under
//     AlternateSelectedControlText, which is what the platform fills a set
//     checkbox with and what it draws the mark on it in;
//   - the find marks and the arrival flash the platform's find highlight, at
//     the two coverages [matchCoverage] and [currentMatchCoverage] name.
//
// Highlight and Images stay nil — both are opt-in. Pass
// tokens.DefaultTypography for the default look.
//
// Mono and CodeSize come from typo's own Code role — the sixteenth style,
// which sits outside the type grid.
//
// The heading sizes come from tokens.DocumentHeadingScale and not from the
// Headline and Title roles, which size the one big line at the top of a
// screen: against a 16 dp body they run 32 down to 14, which sets a document's
// title a quarter again taller than a typeset reading surface sets one —
// enough to wrap a title that should fit a line — while crowding levels three
// and four onto nearly the same size and then dropping a third of the scale
// between levels four and five. The document scale is stepped off the body
// role instead, evenly, so six levels are six levels.
//
// The block gap, the heading spaces and the announcing seam are set from the
// reading rhythm rather than from the smallest stop that separates two controls:
// prose read at length wants the openness a typeset page has, which is a good
// deal more air between blocks than a form wants between its rows. See
// blockRhythm, [headingSpacing] and listSeam for the proportions and where they
// came from.
//
// Of each role only Size lands in the Style: headings and paragraphs carry
// their typeface, weight and slant per span (paragraph.SpanStyle), so those
// parts of a role reach the shaper through the document's spans rather than
// through this constructor. Mono is the one typeface a Style names outright,
// because code spans are built from it.
func FromTokens(p tokens.PlatformColors, typo tokens.Typography, standsOn color.NRGBA) Style {
	var sizes [6]unit.Sp
	var boxes [6]unit.Sp
	for i, style := range typo.DocumentHeadings {
		sizes[i] = unit.Sp(style.Size)
		boxes[i] = unit.Sp(style.LineHeight)
	}
	gap := blockRhythm - lineLeading
	above, below := headingSpacing(gap, sizes)
	surface := standsOnOr(standsOn, p.TextBackground)
	fence := codeFill(p, surface)
	return Style{
		ContentSurface:        surface,
		Text:                  paragraph.FromTokens(p, typo.BodyLarge, surface),
		HeadingSizes:          sizes,
		HeadingLineHeights:    boxes,
		HeadingSpaceAbove:     above,
		HeadingSpaceBelow:     below,
		Mono:                  font.Typeface(typo.Code.Typeface),
		CodeSize:              unit.Sp(typo.Code.Size),
		CodeColor:             p.Text, // code is text, in a monospace face
		CodeBackground:        fence,
		CodeBorder:            codeRim(p, fence),
		CodeChip:              fence,             // one code surface, not two
		CodeChipBorder:        codeRim(p, fence), // one code edge, not two
		CodeScrollbar:         scrollbar.FromTokens(p, fence),
		QuoteBar:              themecolor.Flatten(p.TertiaryLabel, surface),  // see quoteBar
		QuoteColor:            themecolor.Flatten(p.SecondaryLabel, surface), // quoted prose reads as an aside
		RuleColor:             themecolor.Flatten(p.Separator, surface),
		TableBorder:           p.Grid,
		TableHeaderBackground: fence,                                        // the one step a document takes off its page
		CheckboxBorder:        themecolor.Flatten(p.TertiaryLabel, surface), // see checkboxBorder
		CheckboxFill:          p.ControlAccent,
		CheckmarkColor:        p.AlternateSelectedControlText,
		MatchFill:             matchFill(p, surface, matchCoverage),
		ArrivalFill:           matchFill(p, surface, currentMatchCoverage),
		CurrentMatchFill:      matchFill(p, surface, currentMatchCoverage),
		BlockGap:              gap,
		ListSpaceAbove:        listSeam(gap),
		Indent:                unit.Dp(tokens.Spacing.S6),
	}
}

// standsOnOr answers what the document is read on: the fill the caller stated,
// or plane when it stated none. A surface is opaque, so a zero alpha — the
// zero value of the parameter — is no answer rather than a transparent one.
func standsOnOr(stated, plane color.NRGBA) color.NRGBA {
	if stated.A == 0 {
		return plane
	}
	return stated
}

// The coverages the platform's find highlight is laid on at: one match among
// many, and the mark the reader is standing on. The two are one measured
// colour at two strengths rather than two colours, so a reader tells the
// current match from the rest without learning a second mark.
//
// Mail paints every match in one fill and marks no current one — all three
// runs in mail-find-light.png and mail-find-dark.png carry the identical
// pixel — so the platform measures the mark and not the pair, and the second
// strength is this library's. It is placed this way round rather than the
// other because the coverage is the only currency there is: the measured
// value IS the fill at full coverage, so the only mark that can differ from
// it is a weaker one, and the strong end belongs to the mark the reader is on
// so that mark is the platform's own pixel with the platform's own text
// pairing on it. Laying the fill's step off the page on twice instead was
// measured and rejected: it puts the dark scheme's current match on #bebe7c,
// where the text the highlight is required to leave alone falls from Lc 81 to
// Lc 41.
const (
	matchCoverage        = 0x80
	currentMatchCoverage = 0xff
)

// matchFill is the platform's find highlight over the surface the marked
// content stands on, at coverage. The text the fill covers keeps its colour,
// which is why the coverage is not tuned against a text floor: what the
// composite costs the marked words is measured rather than corrected for.
func matchFill(p tokens.PlatformColors, surface color.NRGBA, coverage uint8) color.NRGBA {
	fill := p.FindHighlight
	fill.A = coverage
	return themecolor.Flatten(fill, surface)
}

// codeFill is the surface quoted code sits on, block and chip alike: the
// platform's alternating content fill over the page the document is read on.
//
// It is the one step the platform gives content off its own plane — #f4f5f5
// on white, white at a twentieth on the dark page — which is a fence darker
// than a light page and lighter than a dark one, and gentle either way. No
// stored capture shows a code block in a platform application, so the fence's
// plane is this name rather than a measured code fill; the grouped box is not
// it, being measured over the chrome plane rather than over the content.
func codeFill(p tokens.PlatformColors, surface color.NRGBA) color.NRGBA {
	return themecolor.Flatten(p.AlternatingContentBackground, surface)
}

// codeRim is the hairline drawn around a code surface: the platform's
// separator over the fill it edges.
//
// The fence's own fill is a small step off the page, so the line is the whole
// of what says where a block of code begins and ends rather than a decoration
// on an already-visible block. A seam over whatever is beneath it is what the
// platform draws in that position, and the fill it lands on is the fence's
// because that is what the line is inset into.
func codeRim(p tokens.PlatformColors, fence color.NRGBA) color.NRGBA {
	return themecolor.Flatten(p.Separator, fence)
}

// quoteBar is the bar that leads a blockquote, and checkboxBorder the outline
// of an open task's box: both are TertiaryLabel over the page.
//
// Neither is a seam. A seam parts two regions and the platform draws it at a
// tenth of a coverage — #e6e6e6 on a white page — which is right for a
// hairline nobody is meant to look at and wrong for the whole of what says
// these lines are quoted, or that there is a task here and it is open. The
// platform's weakest label strength is the subordinate mark it does have: a
// third of a coverage, #bdbdbd on the light page and #5b5b5b on the dark one,
// plainly present and plainly not prose.
//
// No stored capture in the organization's macOS reference shows a blockquote
// or a task list in a platform application — the Notes and TextEdit windows
// carry running prose — so this is the Language's answer rather than a
// measured one.
//
// It is deliberately not the field hairline the checkbox in components/input
// draws its box with: that value was measured around a text field on a sheet,
// where the field's interior is the sheet's own fill, and on a white page it
// is #f3f3f3 — a box a reader cannot find.
// The reading rhythm is measured in what the reader sees — the blank run
// between one block's glyphs and the next's — while a [Style] is written in
// authored space. The difference is the leading the shaped lines carry
// between their glyphs and the edges of their line boxes, and these constants are
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
// glyphs'. A 16 dp body line sets 16 px inside a 24 px box, and the 8 px left
// over is what a pair of ordinary blocks shows the reader on top of the space
// between them.
const (
	// lineLeading is what a pair of ordinary blocks contributes.
	lineLeading = unit.Dp(8)
	// headingLeadingAbove is the same for the transition into a heading: a
	// body line's leading below its glyphs, then a heading line's above its own.
	headingLeadingAbove = unit.Dp(6)
	// headingLeadingBelow is the transition out of one, which is the other two
	// halves — a heading's leading below its glyphs and a body line's above its
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
// own 37 px sits. The swing is small because a line occupies its role's box
// rather than its glyphs': the box edges are fixed, so only where the glyphs sit
// inside them varies with the words.
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
// glyphs and then a heading line's above its own; below one it is the heading's
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
		// with the level. Scaling the whole of it would run the scale from
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
// falls back to the ordinary block gap.
func (s Style) listSpace() unit.Dp {
	if s.ListSpaceAbove > 0 {
		return s.ListSpaceAbove
	}
	return s.BlockGap
}

// heading returns the paragraph style for a heading of the given
// level: the level's type-scale size and line box with body colours. A level
// with no line box of its own falls back to the body's, which is the smallest
// box any of them asks for and so cannot squeeze a heading's own metrics.
func (s Style) heading(level int) paragraph.Style {
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

// codeSpans maps a code block's content onto paragraph spans: highlighted
// runs when the style's Highlighter recognises the language, one plain run
// otherwise. Newlines opening a highlighted run (chroma's whitespace tokens
// lead with them) are moved to the previous run's tail: paragraph treats a
// trailing newline as a clean line end, while a leading one would skew the
// line's metrics.
func (s Style) codeSpans(cb *CodeBlock) []paragraph.SpanStyle {
	if s.Highlight != nil {
		if hl := s.Highlight(cb.Language, cb.Code); len(hl) > 0 {
			out := make([]paragraph.SpanStyle, 0, len(hl))
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
				rs := paragraph.SpanStyle{Content: content, Color: h.Color, Typeface: s.Mono}
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
	return []paragraph.SpanStyle{{Content: cb.Code, Typeface: s.Mono}}
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
// leaves the span at the line's own size.
func (s Style) codeSize(size unit.Sp) unit.Sp {
	if s.CodeSize == 0 || s.Text.Size == 0 {
		return size
	}
	return size * s.CodeSize / s.Text.Size
}

// spanStyles maps model spans onto paragraph spans against the style's
// typefaces. defWeight is the run weight for spans without their own bold
// flag (font.Bold for headings), and size is the size the line is set at,
// which inline code is sized against.
func (s Style) spanStyles(spans []Span, defWeight font.Weight, size unit.Sp) []paragraph.SpanStyle {
	out := make([]paragraph.SpanStyle, 0, len(spans))
	for _, sp := range spans {
		rs := paragraph.SpanStyle{
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
			rs.Chip = paragraph.Chip{
				Color:   s.CodeChip,
				Border:  s.CodeChipBorder,
				Padding: codeChipPad,
				Radius:  codeChipRadius,
			}
		}
		out = append(out, rs)
	}
	return out
}
