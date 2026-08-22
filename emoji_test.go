package markdown_test

import (
	"image"
	"strings"
	"testing"
	"unicode"

	"gioui.org/font"
	"gioui.org/text"
	"golang.org/x/image/math/fixed"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

const grin = '😀'

// emojiShaper is the pinned collection plus Noto Color Emoji — the
// shaper a document that actually draws emoji uses. Default-path
// goldens stay on defaultShaper so they do not parse the 9.9 MB face.
func emojiShaper(t *testing.T) *text.Shaper {
	t.Helper()
	typ := tokens.DefaultTypography.WithEmoji()
	return typ.DeterministicShaper()
}

// resolvedGrin shapes 😀 with f and reports the font's own glyph ID
// and the face it came from. Glyph ID 0 is .notdef; the appended face
// is Noto Color Emoji.
func resolvedGrin(t *testing.T, shaper *text.Shaper, f font.Font) (gid uint32, faceIdx int) {
	t.Helper()
	shaper.LayoutString(text.Parameters{
		Font:     f,
		PxPerEm:  fixed.I(16),
		MaxWidth: 1000,
	}, string(grin))
	g, ok := shaper.NextGlyph()
	if !ok {
		t.Fatalf("font %+v: shaper produced no glyph for 😀", f)
	}
	return uint32(g.ID), int(uint64(g.ID) >> 48)
}

// TestEmojiResolvesInEachConstruct: a paragraph, a heading, a list
// item and a table cell each resolve 😀 to a real glyph on the
// appended face. A fenced comment still does — fallback from
// "Roboto Mono" onto that face, not tofu on the mono face.
func TestEmojiResolvesInEachConstruct(t *testing.T) {
	typ := tokens.DefaultTypography.WithEmoji()
	shaper := typ.DeterministicShaper()
	style := markdown.FromTokens(tokens.DefaultLight, typ)
	appended := len(typ.Faces) - 1

	cases := []struct {
		name string
		src  string
		font font.Font
		peek func(t *testing.T, blocks []markdown.Block)
	}{
		{
			name: "paragraph",
			src:  "Hello 😀\n",
			font: font.Font{},
			peek: func(t *testing.T, blocks []markdown.Block) {
				p, ok := blocks[0].(*markdown.Paragraph)
				if !ok {
					t.Fatalf("got %T, want *Paragraph", blocks[0])
				}
				if !strings.ContainsRune(spanText(p.Spans), grin) {
					t.Fatalf("paragraph %q has no 😀", spanText(p.Spans))
				}
			},
		},
		{
			name: "heading",
			src:  "# Hello 😀\n",
			font: font.Font{Weight: font.Bold},
			peek: func(t *testing.T, blocks []markdown.Block) {
				h, ok := blocks[0].(*markdown.Heading)
				if !ok {
					t.Fatalf("got %T, want *Heading", blocks[0])
				}
				if !strings.ContainsRune(spanText(h.Spans), grin) {
					t.Fatalf("heading %q has no 😀", spanText(h.Spans))
				}
			},
		},
		{
			name: "list item",
			src:  "- Hello 😀\n",
			font: font.Font{},
			peek: func(t *testing.T, blocks []markdown.Block) {
				l, ok := blocks[0].(*markdown.List)
				if !ok || len(l.Items) == 0 {
					t.Fatalf("got %T, want a *List with an item", blocks[0])
				}
				p, ok := l.Items[0].Blocks[0].(*markdown.Paragraph)
				if !ok {
					t.Fatalf("item block is %T, want *Paragraph", l.Items[0].Blocks[0])
				}
				if !strings.ContainsRune(spanText(p.Spans), grin) {
					t.Fatalf("list item %q has no 😀", spanText(p.Spans))
				}
			},
		},
		{
			name: "table cell",
			src:  "| col |\n| --- |\n| Hello 😀 |\n",
			font: font.Font{},
			peek: func(t *testing.T, blocks []markdown.Block) {
				tb, ok := blocks[0].(*markdown.Table)
				if !ok || len(tb.Rows) == 0 || len(tb.Rows[0]) == 0 {
					t.Fatalf("got %T, want a *Table with a body cell", blocks[0])
				}
				if !strings.ContainsRune(spanText(tb.Rows[0][0].Spans), grin) {
					t.Fatalf("table cell %q has no 😀", spanText(tb.Rows[0][0].Spans))
				}
			},
		},
		{
			name: "fenced comment",
			src:  "```\n// Hello 😀\n```\n",
			font: font.Font{Typeface: style.Mono},
			peek: func(t *testing.T, blocks []markdown.Block) {
				cb, ok := blocks[0].(*markdown.CodeBlock)
				if !ok {
					t.Fatalf("got %T, want *CodeBlock", blocks[0])
				}
				if !strings.ContainsRune(cb.Code, grin) {
					t.Fatalf("fence %q has no 😀", cb.Code)
				}
				if style.Mono != "Roboto Mono" {
					t.Fatalf("Mono = %q, want Roboto Mono; the fallback under test is from that face", style.Mono)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := markdown.Parse([]byte(tc.src))
			if len(blocks) == 0 {
				t.Fatal("Parse returned no blocks")
			}
			tc.peek(t, blocks)
			gid, faceIdx := resolvedGrin(t, shaper, tc.font)
			if gid == 0 {
				t.Fatalf("😀 resolved to glyph ID 0 (.notdef) under %+v", tc.font)
			}
			if faceIdx != appended {
				t.Errorf("😀 resolved on face %d, want the appended emoji face %d", faceIdx, appended)
			}
		})
	}

	if tofu, _ := resolvedGrin(t, defaultShaper(t), font.Font{}); tofu != 0 {
		t.Errorf("😀 on DefaultTypography.DeterministicShaper resolved to glyph %d, want 0 (tofu control)", tofu)
	}
	if tofu, _ := resolvedGrin(t, defaultShaper(t), font.Font{Typeface: "Roboto Mono"}); tofu != 0 {
		t.Errorf("😀 on Roboto Mono without the face resolved to glyph %d, want 0 (tofu control)", tofu)
	}
}

func spanText(spans []markdown.Span) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

// emojiHelloSize is the viewport the Hello 😀 goldens render in: one
// body line plus the themed inset, wide enough for the grin.
var emojiHelloSize = image.Pt(240, 64)

// TestEmojiHelloGolden records a short document "Hello 😀" in both
// schemes on the emoji shaper. The same document without the face is
// the tofu control and must not match.
func TestEmojiHelloGolden(t *testing.T) {
	with := emojiShaper(t)
	without := defaultShaper(t)
	blocks := markdown.Parse([]byte("Hello 😀\n"))
	size := emojiHelloSize
	cases := []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"emoji-hello-light", tokens.DefaultLight},
		{"emoji-hello-dark", tokens.DefaultDark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := markdown.FromTokens(tc.colors, tokens.DefaultTypography)
			painted := golden.Capture(t, size, themed(markdown.NewDocument(blocks), with, style, tc.colors))
			golden.Compare(t, tc.name, painted)

			tofu := golden.Capture(t, size, themed(markdown.NewDocument(blocks), without, style, tc.colors))
			if golden.PixelDiff(painted, tofu) == 0 {
				t.Fatal("Hello 😀 with the emoji face matches the tofu control; Bitmaps did not paint")
			}
		})
	}
}

// TestCorpusContainsNoEmoji pins the other half of the paint contract:
// default-path corpus goldens stay emoji-free, so they do not need the
// color-emoji face and must not move when it is appended elsewhere.
func TestCorpusContainsNoEmoji(t *testing.T) {
	for i, r := range string(corpus(t)) {
		if isEmojiRune(r) {
			t.Fatalf("testdata/corpus.md contains U+%04X at byte %d; default-path goldens must stay emoji-free", r, i)
		}
	}
}

func isEmojiRune(r rune) bool {
	switch {
	case r == 0xFE0F, r == 0x200D:
		return true
	case r >= 0x1F300 && r <= 0x1FAFF:
		return true
	case r >= 0x2600 && r <= 0x27BF:
		return unicode.Is(unicode.So, r)
	default:
		return false
	}
}
