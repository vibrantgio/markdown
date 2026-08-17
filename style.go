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
	// BlockGap is the vertical space between sibling blocks.
	BlockGap unit.Dp
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
	// Highlight, when non-nil, syntax-highlights fenced code blocks.
	// markdown/highlight provides a chroma-backed implementation.
	Highlight Highlighter
	// Images, when non-nil, supplies the pixels for [Image] blocks; without
	// it every image falls back to its alt text. A value that also
	// implements [WidgetImageProvider] can serve vector images as widgets.
	Images ImageProvider
}

// FromTokens derives the default document style from colour tokens and a
// typography: headings step down HeadlineLarge..TitleSmall, body text follows
// richtext.FromTokens on the BodyLarge role, code sits on the Neutral 300
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
// Of each role only Size lands in the Style: headings and paragraphs carry
// their typeface, weight and slant per span (richtext.SpanStyle), so those
// parts of a role reach the shaper through the document's spans rather than
// through this constructor. Mono is the one typeface a Style names outright,
// because code spans are built from it.
func FromTokens(c tokens.ColorTokens, typo tokens.Typography) Style {
	return Style{
		Text: richtext.FromTokens(c, typo.BodyLarge),
		HeadingSizes: [6]unit.Sp{
			unit.Sp(typo.HeadlineLarge.Size),
			unit.Sp(typo.HeadlineMedium.Size),
			unit.Sp(typo.HeadlineSmall.Size),
			unit.Sp(typo.TitleLarge.Size),
			unit.Sp(typo.TitleMedium.Size),
			unit.Sp(typo.TitleSmall.Size),
		},
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
		BlockGap:              unit.Dp(tokens.Spacing.S2),
		Indent:                unit.Dp(tokens.Spacing.S6),
	}
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
