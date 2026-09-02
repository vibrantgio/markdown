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
// token-themed default with [FromTokens], then set Text.OnLinkClick and,
// for a reader that writes GFM task markers, OnTaskClick.
//
// # Paper
//
// The surface a Style describes is paper: the quiet ground running text is
// read on, distinct from chrome — the rails, bars, cards and controls that
// answer to the theme directly. Paper answers to the theme through roles of
// its own, and the four that make it paper are:
//
//   - [Style.Paper], the ground the document is read on;
//   - [Style.Text]'s colours, the prose inks — the body, its links, its focus
//     ring;
//   - [Style.HeadingSizes] with [Style.HeadingLineHeights], the heading
//     ladder a document is broken up by;
//   - [Style.CodeChip], the fill under a word of code quoted into a sentence.
//
// The rest of the fields dress the blocks standing on that paper — a fence, a
// quote, a rule, a table, a task box.
//
// The invariant: every colour this package draws comes from a field of this
// struct and from nowhere else. The layout code reads the theme for spacing
// and for radii and for no colour at all, so a document looks like what its
// Style says and nothing reaches around it. Deriving a paper role from a theme
// token is fine; drawing a document with a token instead of with a role is
// not, and nothing here does.
type Style struct {
	// Paper is the ground the document is read on: what lies behind the
	// prose, under every block, out to the edges of whatever holds it.
	//
	// Nothing in this package paints it. A document is laid out into a space
	// somebody else owns, and that owner fills the ground — so this is a
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
	// elevation ladder's raised storey — a fence is a raised chip, lighter
	// than the page it lies on, in a light scheme and a dark one alike: a
	// near-black in a dark scheme and a near-white in a light one.
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
	// It is for the ground that does not, which is every ground a fence
	// takes. The ladder climbs toward the light in both
	// schemes and a light scheme has 3.1 L* of room above its paper to climb
	// into, so a raised fence there is a whisper — 1.02:1 off the page — and
	// laid on it unbounded the block stops being a block and the code reads
	// as a paragraph in a monospace face. A syntax palette fitted to paper
	// puts its own near-white in the same position. The line is what says
	// where the fence is when its fill no longer does, and [FromTokens]
	// derives it against whatever fill it is edging (see codeRim).
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
	// and deliberately: a page of prose spotted with somebody else's grounds
	// would be a page arguing with itself, so a chip stays on the quiet fill
	// and in the body's own ink while the block down the page shows the
	// palette whole.
	CodeChip color.NRGBA
	// CodeChipBorder strokes a hairline just inside the chip's rounded edge,
	// as [Style.CodeBorder] does for the fence. A zero alpha draws none.
	//
	// The chip and the fence are one construct at two sizes, so they take one
	// fill, and in a light scheme that fill is a whisper above the paper —
	// which a fence survives, having a rim and a radius and a screenful of
	// area to be recognised by, and a word of code does not. A tint is not
	// available as an answer: a hue in this system belongs to a role, and code
	// is not a role — a chip tinted Primary would hand the brand colour to the
	// one span quoted for not carrying it, and a chip tinted from a status role
	// would say something went wrong. So the chip takes the fence's answer at
	// the chip's own size: the same fill and the same derived rim.
	//
	// It is separate from CodeBorder for the same reason CodeChip is separate
	// from CodeBackground: a fence dressed in a syntax palette takes that
	// palette's ground and the edge that ground calls for, while the chip
	// stays on the theme's quiet fill and keeps the theme's own rim.
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
	// painted inside it, so the stroke lies straight on [Style.Paper] and is
	// the whole of what says there is a task here and it is open — a graphic
	// carrying meaning without being text, owing its page WCAG 1.4.11's 3:1.
	//
	// CheckboxFill is the same box in the other state and is a separate
	// field because the two are drawn on opposite grounds: this one on the
	// page, that one over it. One colour can only serve both while it happens
	// to read on both, which is a property of the brand a Style was derived
	// from and not of this package. See [FromTokens].
	CheckboxBorder color.NRGBA
	// CheckboxFill fills the box of a checked task item, wall to wall, with
	// [Style.CheckmarkColor]'s tick drawn over it. It is a filled mark and
	// not an ink on the page: what it owes contrast to is the tick it
	// carries, not the paper it covers, so it is entitled to be the brand's
	// own colour at the brand's own depth.
	CheckboxFill color.NRGBA
	// CheckmarkColor draws the check mark inside a checked checkbox. Its
	// ground is CheckboxFill rather than [Style.Paper] — the fill covers the
	// box before the tick goes on — so it is a colour chosen against that
	// fill, and a Style that moves the fill has to move this with it.
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

// FromTokens derives the default document style from colour tokens and a
// typography: the paper is the theme's own background, headings take the six
// stops of the typography's document heading scale, body text follows
// richtext.FromTokens on the BodyLarge role, code sits on the elevation
// ladder's raised storey (see codeFill) with the ink codeInk derives, inline
// code on the same fill while keeping the body's own ink so a quoted word
// reads as the sentence's, the quote bar is Primary with Neutral 700 text,
// rules and table grid lines are separators and use Divider, and the table
// header row sits on the Neutral 300 tinted fill. Highlight and Images stay
// nil — both are opt-in. Pass tokens.DefaultTypography for the default look.
//
// The ground is the theme's background because that is where a document lies,
// and a holder that mounts one somewhere else says so by setting Paper
// afterwards: this constructor answers for the theme and not for the
// composition.
//
// The code surface is one step off the page and not three. A fence covers a
// good deal of the column, and area amplifies a fill: the tinted-fill step
// that reads as a tint behind a table's header row reads, spread under a
// screenful of code, as a slab of grey with the page showing white around it —
// worst in a light scheme, where it also leaves the code's own ink barely over
// its floor. The measured reference is gentler still, a code surface 3.4 L*
// off its page against the 4.9 and 5.0 this step gives in the light and dark
// schemes, and it puts one surface under a fence and an inline chip alike, so
// they are one here.
//
// Mono and CodeSize come from typo's own Code role — the sixteenth style,
// which sits outside the MD3 grid.
//
// The heading sizes come from tokens.DocumentHeadingScale and not from the
// Headline and Title roles, which size the one big line at the top of a
// screen: against a 16 dp body they run 32 down to 14, which inks a document's
// title a quarter again taller than a typeset reading surface inks one —
// enough to wrap a title that should fit a line — while crowding levels three
// and four onto nearly the same size and then dropping a third of the ladder
// between levels four and five. The document scale is stepped off the body
// role instead, evenly, so six levels are six levels.
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
		CodeColor:             codeInk(c),  // see codeInk
		CodeBackground:        codeFill(c), // the raised storey, see codeFill
		CodeBorder:            codeRim(c),  // the edge that says where it is
		CodeChip:              codeFill(c), // one code surface, not two
		CodeChipBorder:        codeRim(c),  // one code edge, not two
		CodeScrollbar:         codeScrollbar(c),
		QuoteBar:              quoteBar(c),               // see quoteBar
		QuoteColor:            c.Ramps.Neutral.Step(700), // low-contrast text
		RuleColor:             c.Divider,
		TableBorder:           c.Divider,
		TableHeaderBackground: c.Ramps.Neutral.Step(300), // tinted fill
		CheckboxBorder:        checkboxBorder(c),         // an ink on the page
		CheckboxFill:          checkboxFill(c),           // a fill keeps its brand
		CheckmarkColor:        checkmarkInk(c),           // measured on that fill
		BlockGap:              gap,
		ListSpaceAbove:        listSeam(gap),
		Indent:                unit.Dp(tokens.Spacing.S6),
	}
}

// codeFill is the surface quoted code sits on, block and chip alike: the
// elevation ladder's raised storey, asked of the palette.
//
// A fence is a raised chip — lighter than the page it lies on, in both
// schemes — which is what the measured reference shows and what walking the
// neutral ramp one rung off the pin cannot give, that step darkening in a
// light scheme and lightening in a dark one.
//
// On the default palettes the dark fence lands on #222222 over #181818, and
// the light one on #F8F8F8 over #F6F6F6 — a whisper, 0.7 L*, the light scheme
// having spent nearly all of its tonal axis on the paper already. That is the
// ladder's trade: whisper steps toward white, with a derived hairline carrying
// the visible edge. codeRim is that hairline.
func codeFill(c tokens.ColorTokens) color.NRGBA {
	return c.SurfaceAt(tokens.Level1)
}

// codeRim is the hairline drawn around a code surface: the neutral rung
// nearest the ramp's mid-value step that reaches [codeFloor] against the fill
// it edges.
//
// It is the same derivation every other surface's edge in this design system
// takes against its own fill. The fill carries 1.02:1 against a light paper,
// so the line is the whole of what says a block of code is a block rather than
// a paragraph in a monospace face. A graphic that carries meaning without
// being text owes WCAG 1.4.11's 3:1, so the line takes it.
//
// Both schemes take the line. The dark fence's fill measures 1.12:1 off its
// page — no more a floor than the light fence's 1.02:1, only more of a hint —
// so edging one and not the other would be a per-scheme rule. On the default
// palettes the line lands on #797979 in the light scheme, 4.10:1 on the fence
// and 4.03:1 on the paper, and on #9E9E9E in the dark one, 5.94:1 and 6.63:1.
func codeRim(c tokens.ColorTokens) color.NRGBA {
	return c.MarkOn(tokens.RoleNeutral, codeFill(c), codeFloor)
}

// codeFloor is WCAG 1.4.11's contrast floor for a graphic that carries
// meaning without being text — 3:1. A code surface's rim is exactly such a
// graphic once its fill has stopped separating: it is the whole of what says
// where the code begins and ends. It is the palette's own graphic floor
// under a local name, not a second number.
const codeFloor = tokens.GraphicFloor

// quoteBar is the bar that leads a blockquote: the brand's own colour where
// that colour reads on the page, and the rung of the brand's ramp that does
// where it does not.
//
// The bar is a graphic carrying meaning without being text — it is the whole
// of what says these lines are quoted, the quoted prose itself being set in a
// neutral — so it owes the page WCAG 1.4.11's 3:1, the same floor the code
// rim takes.
//
// The Primary pin will not serve: a pin is the brand colour at the brand's own
// depth, chosen so that text laid on TOP of it reads, and it is not measured
// against the page. On the canonical seed it measures 5.94:1 against the light
// paper, but on an accent stated at a dark scheme's tone — the shape a palette
// published for dark mode hands out — it measures 1.95:1, a bar nobody can
// see. Asking the palette for an ink measures it instead, and the canonical
// seed's bar is unchanged.
func quoteBar(c tokens.ColorTokens) color.NRGBA {
	return c.InkOn(tokens.RolePrimary, c.SurfaceAt(tokens.Level0), tokens.GraphicFloor)
}

// checkboxBorder is the outline of an open task's box: the brand's own colour
// where that colour reads on the page, and the rung of the brand's ramp that
// does where it does not — the quote bar's derivation, on the same ground and
// at the same floor, because it is the same kind of thing. An empty box is
// nothing but its outline, so the outline carries the whole of "there is a
// task here" without being text: WCAG 1.4.11's 3:1 against the page, which is
// [Style.Paper], which is the theme's own ground.
//
// The derivation matters where the brand does NOT read on the paper: an accent
// stated at a dark scheme's tone derives a light palette whose primary pin sits
// a whisper off its own page, and an open task box drawn in it is a box nobody
// can find. Over the seed sweep 208 of 414 light schemes put that pin under
// this floor.
func checkboxBorder(c tokens.ColorTokens) color.NRGBA {
	return c.InkOn(tokens.RolePrimary, c.SurfaceAt(tokens.Level0), tokens.GraphicFloor)
}

// checkboxFill is the body of a completed task's box, and it is the pin,
// deliberately.
//
// It is not gated against the page, because that would be measuring the wrong
// pair. A fill is not an ink: it covers its ground rather
// than sitting on it, and a solid mark that reads as brand-coloured is what a
// finished task is meant to look like. What it owes contrast to is the tick
// laid ON it, and the pin's whole guarantee — the one the derivation solves
// for every seed — is precisely that something reads on top of it. Walking
// this to suit the page would move a filled box out from under its own tick
// to fix a comparison nothing makes. It is the same claim the palette's own
// on-colour derivation states about text over a base, and the same one every
// other solid brand body in this design system is drawn under.
func checkboxFill(c tokens.ColorTokens) color.NRGBA {
	return c.Primary
}

// checkmarkInk is the tick drawn on [checkboxFill].
//
// It is a mark and not text — a stroked glyph-shaped path carrying "done"
// with no words in it — so the floor it owes its ground is WCAG 1.4.11's 3:1,
// the graphic floor, not the 4.5:1 a run of words would owe. What it actually
// gets is more than that, and by construction rather than by luck: while the
// fill is the Primary pin, the colour derived to read over that pin is
// OnPrimary, which the derivation holds to the 4.5:1 text floor for every
// seed. Naming the fill's own on-colour is therefore both the right answer
// and a comfortable one.
//
// The pairing is asserted per seed rather than assumed: a hand that moves
// checkboxFill and leaves this alone orphans the tick on a ground it was never
// measured against.
func checkmarkInk(c tokens.ColorTokens) color.NRGBA {
	return c.OnPrimary
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
// Only the two colours change, and what they answer is a question the shared
// bar does not have. scrollbar.FromTokens derives a translucent thumb: the
// most transparent one that still clears its contrast floor over the two
// surfaces an overlay bar rides, the window's page and the chrome level its
// panes are filled at. A fence's fill is neither of those — it is a raised
// level, which is lighter than both in either scheme — so the shared
// bar clears its floor here by more than it was asked to, and this override
// is not buying legibility.
//
// What it buys is the thing translucency was protecting, spent where there
// is nothing to protect. Coverage is how much of what lies under the bar
// stops showing through, and a column's bar lies over the column's own text,
// where showing through is the whole point; a fence's lies over the fence's
// bottom padding, where nothing shows through it either way. So the fence
// spends the coverage it cannot use and takes an opaque thumb, at the ramp's
// low-contrast text step — as present against the fence's fill as text on it
// would be, 6.30:1 in the light appearance and 9.91:1 in the dark, against
// the 3:1 the shared bar stops at — darkening to the ramp's far end while
// hovered or dragged. A pairing already that far past the floor is not one a
// derivation aimed at the floor should be allowed to walk back.
//
// That is a weight against a ground and not a match to the code's own ink,
// which is why it stays on this step in both appearances while the light
// appearance's code sits one step past it (see codeInk) — the bar lies on the
// code surface, it is not a run of code, and a fence's one draggable
// affordance does not get heavier because the reading got heavier.
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
// between them.
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
// own 37 px sits. The swing is small because a line occupies its role's box
// rather than its glyphs': the box edges are fixed, so only where the ink sits
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
// falls back to the ordinary block gap.
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
// leaves the span at the line's own size.
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
				Border:  s.CodeChipBorder,
				Padding: codeChipPad,
				Radius:  codeChipRadius,
			}
		}
		out = append(out, rs)
	}
	return out
}
