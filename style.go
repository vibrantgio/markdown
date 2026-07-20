package markdown

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/unit"

	"github.com/vibrantgio/prism/richtext"
	"github.com/vibrantgio/prism/tokens"
)

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
}

// FromTokens derives the default document style from colour tokens and the
// type scale: headings step down HeadlineLarge..TitleSmall, body text follows
// richtext.FromTokens, code sits on the SurfaceVariant surface, the quote bar
// is Primary with OnSurfaceVariant text, and rules use Outline.
func FromTokens(c tokens.ColorTokens, ts tokens.TypeScale) Style {
	return Style{
		Text: richtext.FromTokens(c, ts),
		HeadingSizes: [6]unit.Sp{
			unit.Sp(ts.HeadlineLarge),
			unit.Sp(ts.HeadlineMedium),
			unit.Sp(ts.HeadlineSmall),
			unit.Sp(ts.TitleLarge),
			unit.Sp(ts.TitleMedium),
			unit.Sp(ts.TitleSmall),
		},
		Mono:           "Go Mono, monospace",
		CodeSize:       unit.Sp(ts.BodyMedium),
		CodeColor:      c.OnSurfaceVariant,
		CodeBackground: c.SurfaceVariant,
		QuoteBar:       c.Primary,
		QuoteColor:     c.OnSurfaceVariant,
		RuleColor:      c.Outline,
		CheckboxColor:  c.Primary,
		CheckmarkColor: c.OnPrimary,
		BlockGap:       unit.Dp(tokens.Spacing.S2),
		Indent:         unit.Dp(tokens.Spacing.S6),
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
