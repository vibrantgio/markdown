package markdown_test

import (
	"image"
	"reflect"
	"testing"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// paragraphSpans parses a source expected to hold exactly one paragraph and
// returns its spans.
func paragraphSpans(t *testing.T, src string) []markdown.Span {
	t.Helper()
	blocks := markdown.Parse([]byte(src))
	if len(blocks) != 1 {
		t.Fatalf("Parse(%q) returned %d blocks, want 1", src, len(blocks))
	}
	p, ok := blocks[0].(*markdown.Paragraph)
	if !ok {
		t.Fatalf("Parse(%q) returned %T, want *Paragraph", src, blocks[0])
	}
	return p.Spans
}

// TestEscapesInProse pins CommonMark's backslash rule on paragraph text: a
// backslash before ASCII punctuation yields the bare character, a backslash
// before anything else stays, and a doubled backslash yields one.
func TestEscapesInProse(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []markdown.Span
	}{{
		name: "escaped underscore",
		src:  `q5\_0 plain`,
		want: []markdown.Span{txt("q5_0 plain")},
	}, {
		name: "assorted punctuation",
		src:  `\!bang \# hash \~tilde \| pipe`,
		want: []markdown.Span{txt("!bang # hash ~tilde | pipe")},
	}, {
		name: "doubled backslash yields one",
		src:  `a \\ b`,
		want: []markdown.Span{txt(`a \ b`)},
	}, {
		name: "doubled backslash escapes nothing after it",
		src:  `a \\*still emphasis*`,
		want: []markdown.Span{txt(`a \`), ital("still emphasis")},
	}, {
		name: "backslash before a letter stays literal",
		src:  `\a letter and \9 a digit`,
		want: []markdown.Span{txt(`\a letter and \9 a digit`)},
	}, {
		name: "backslash before a space stays literal",
		src:  `two\ words`,
		want: []markdown.Span{txt(`two\ words`)},
	}, {
		name: "trailing backslash stays literal",
		src:  `trailing backslash\`,
		want: []markdown.Span{txt(`trailing backslash\`)},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := paragraphSpans(t, tc.src)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q) spans:\n got  %#v\n want %#v", tc.src, got, tc.want)
			}
		})
	}
}

// TestEscapesSuppressDelimiters pins the other half of the rule: an escaped
// delimiter carries no delimiter role, so it opens no emphasis, no
// strikethrough and no link — and an unescaped delimiter still does, with the
// escaped one riding along inside it as ordinary text.
func TestEscapesSuppressDelimiters(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []markdown.Span
	}{{
		name: "escaped asterisks open no emphasis",
		src:  `a \*not emphasis\* b`,
		want: []markdown.Span{txt("a *not emphasis* b")},
	}, {
		name: "escaped underscores open no emphasis",
		src:  `a \_not emphasis\_ b`,
		want: []markdown.Span{txt("a _not emphasis_ b")},
	}, {
		name: "escaped tildes open no strikethrough",
		src:  `a \~\~not struck\~\~ b`,
		want: []markdown.Span{txt("a ~~not struck~~ b")},
	}, {
		name: "escaped brackets open no link",
		src:  `\[not a link\](notes.md)`,
		want: []markdown.Span{txt("[not a link](notes.md)")},
	}, {
		name: "escaped bracket inside a real link stays text",
		src:  `[a \[bracket\] here](notes.md)`,
		want: []markdown.Span{{
			Text: "a [bracket] here",
			URL:  "notes.md",
		}},
	}, {
		name: "escaped asterisk inside real emphasis stays text",
		src:  `*em with \* inside*`,
		want: []markdown.Span{ital("em with * inside")},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := paragraphSpans(t, tc.src)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q) spans:\n got  %#v\n want %#v", tc.src, got, tc.want)
			}
		})
	}
}

// TestEscapesInTableCells carries the rule into table cells: escaped
// underscores lose their backslashes, and an escaped pipe stays inside its
// cell instead of splitting it.
func TestEscapesInTableCells(t *testing.T) {
	src := "| Quant | Note |\n" +
		"|-------|------|\n" +
		`| q5\_0 | fits \| both |` + "\n" +
		`| q8\_0 | larger |` + "\n"

	want := []markdown.Block{&markdown.Table{
		Alignments: []markdown.Alignment{markdown.AlignLeft, markdown.AlignLeft},
		Header:     []*markdown.TableCell{cell(txt("Quant")), cell(txt("Note"))},
		Rows: [][]*markdown.TableCell{
			{cell(txt("q5_0")), cell(txt("fits | both"))},
			{cell(txt("q8_0")), cell(txt("larger"))},
		},
	}}

	if got := markdown.Parse([]byte(src)); !reflect.DeepEqual(got, want) {
		t.Errorf("Parse:\n got  %#v\n want %#v", got, want)
	}
}

// TestEscapedTableCellsRenderAsLiterals is the pixel half of the same claim:
// the escaped source and the source someone would write to mean the same
// thing shape into the same image. A backslash left on screen would be extra
// ink, and the widths it shifts would move every glyph after it.
func TestEscapedTableCellsRenderAsLiterals(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	size := image.Pt(400, 160)

	shot := func(row string) *image.RGBA {
		src := "| Quant | Note |\n|---|---|\n" + row
		d := markdown.NewDocument(markdown.Parse([]byte(src)))
		return golden.Capture(t, size, themed(d, shaper, style, tokens.DefaultLight))
	}

	escaped := shot(`| q5\_0 | q8\_0 |` + "\n")
	literal := shot(`| q5_0 | q8_0 |` + "\n")

	if n := golden.PixelDiff(escaped, literal); n != 0 {
		t.Errorf(`cells written "q5\_0" and "q5_0" differ by %d pixels, want an identical rendering`, n)
	}
	// The probe only means something if it can see a cell's text change at
	// all: one different character has to move pixels.
	if n := golden.PixelDiff(literal, shot(`| q5_1 | q8_0 |`+"\n")); n == 0 {
		t.Error("the pixel probe saw no difference between two different cell texts")
	}
}

// TestCodeKeepsBackslashes pins the exemption: backslashes inside a code span
// or a code block are literal by definition, escape nothing, and survive
// verbatim — including the escaped pipe a table cell needs to carry a code
// span through the row splitter.
func TestCodeKeepsBackslashes(t *testing.T) {
	t.Run("code span", func(t *testing.T) {
		got := paragraphSpans(t, "a `q5\\_0` b")
		want := []markdown.Span{txt("a "), code(`q5\_0`), txt(" b")}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("spans:\n got  %#v\n want %#v", got, want)
		}
	})

	t.Run("code block", func(t *testing.T) {
		blocks := markdown.Parse([]byte("```\nprintf(\"a\\_b\\n\");\n```\n"))
		want := []markdown.Block{&markdown.CodeBlock{Code: `printf("a\_b\n");`}}
		if !reflect.DeepEqual(blocks, want) {
			t.Errorf("Parse:\n got  %#v\n want %#v", blocks, want)
		}
	})

	t.Run("escaped pipe inside a code span in a cell", func(t *testing.T) {
		src := "| a |\n|---|\n" + "| `x\\|y` |\n"
		want := []markdown.Block{&markdown.Table{
			Alignments: []markdown.Alignment{markdown.AlignLeft},
			Header:     []*markdown.TableCell{cell(txt("a"))},
			Rows:       [][]*markdown.TableCell{{cell(code("x|y"))}},
		}}
		if got := markdown.Parse([]byte(src)); !reflect.DeepEqual(got, want) {
			t.Errorf("Parse:\n got  %#v\n want %#v", got, want)
		}
	})
}

// TestEscapeAtLineEnd separates the two things a backslash can mean at the
// end of a line: alone it is the hard-break marker the block model already
// carries as a trailing newline, and doubled it is one literal backslash
// followed by an ordinary soft break.
func TestEscapeAtLineEnd(t *testing.T) {
	t.Run("hard break consumes the backslash", func(t *testing.T) {
		got := paragraphSpans(t, "a hard\\\nbreak\n")
		want := []markdown.Span{txt("a hard\nbreak")}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("spans:\n got  %#v\n want %#v", got, want)
		}
	})

	t.Run("doubled backslash is literal before a soft break", func(t *testing.T) {
		got := paragraphSpans(t, "a\\\\\nbreak\n")
		want := []markdown.Span{txt(`a\ break`)}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("spans:\n got  %#v\n want %#v", got, want)
		}
	})
}

// TestEscapesInDestinationsAndAltText carries the rule into the two inline
// strings that are not styled runs: a link or image destination, and the alt
// text an image falls back to.
func TestEscapesInDestinationsAndAltText(t *testing.T) {
	t.Run("link destination", func(t *testing.T) {
		got := paragraphSpans(t, `[label](notes/q5\_0.md)`)
		want := []markdown.Span{{Text: "label", URL: "notes/q5_0.md"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("spans:\n got  %#v\n want %#v", got, want)
		}
	})

	t.Run("image destination and alt text", func(t *testing.T) {
		blocks := markdown.Parse([]byte(`![a \_quiet\_ chart](charts/q5\_0.png)`))
		want := []markdown.Block{&markdown.Image{
			URL: "charts/q5_0.png",
			Alt: "a _quiet_ chart",
		}}
		if !reflect.DeepEqual(blocks, want) {
			t.Errorf("Parse:\n got  %#v\n want %#v", blocks, want)
		}
	})
}
