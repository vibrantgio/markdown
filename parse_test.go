package markdown_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/vibrantgio/markdown"
)

func corpus(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/corpus.md")
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// span constructors keep the expected tree readable.
func txt(s string) markdown.Span  { return markdown.Span{Text: s} }
func bold(s string) markdown.Span { return markdown.Span{Text: s, Bold: true} }
func ital(s string) markdown.Span { return markdown.Span{Text: s, Italic: true} }
func code(s string) markdown.Span { return markdown.Span{Text: s, Code: true} }

func para(spans ...markdown.Span) *markdown.Paragraph {
	return &markdown.Paragraph{Spans: spans}
}

func item(blocks ...markdown.Block) *markdown.ListItem {
	return &markdown.ListItem{Blocks: blocks}
}

func task(checked bool, blocks ...markdown.Block) *markdown.ListItem {
	return &markdown.ListItem{Task: true, Checked: checked, Blocks: blocks}
}

// TestParseCorpus asserts the corpus document's full block tree: every G6.2
// construct — heading levels 1–6, styled paragraph runs (bold, italic, bold
// italic, inline code, link, autolink, GFM strikethrough, soft and hard
// breaks), nested unordered and ordered lists, an ordered list with an
// explicit start, GFM task items, nested blockquotes, fenced code with
// language and tab expansion, indented code, and a thematic break.
func TestParseCorpus(t *testing.T) {
	got := markdown.Parse(corpus(t))

	want := []markdown.Block{
		&markdown.Heading{Level: 1, Spans: []markdown.Span{txt("Markdown corpus")}},
		para(
			txt("This corpus exercises every construct the G6.2 block renderer supports: it stands in for the 2026-07-20 "),
			code("gioui.org/x/markdown"),
			txt(" evaluation document."),
		),
		para(
			txt("Inline styles: "),
			bold("bold"),
			txt(", "),
			ital("italic"),
			txt(", "),
			markdown.Span{Text: "bold italic", Bold: true, Italic: true},
			txt(", "),
			code("inline code"),
			txt(", a "),
			markdown.Span{Text: "prism link", URL: "https://github.com/vibrantgio/prism"},
			txt(", "),
			markdown.Span{Text: "struck text", Strikethrough: true},
			txt(", and an autolink "),
			markdown.Span{Text: "https://gioui.org", URL: "https://gioui.org"},
			txt(" for good measure."),
		),
		&markdown.Heading{Level: 2, Spans: []markdown.Span{txt("Lists")}},
		&markdown.List{Items: []*markdown.ListItem{
			item(para(txt("first bullet"))),
			item(
				para(txt("second bullet holds "), bold("bold"), txt(" text")),
				&markdown.List{Items: []*markdown.ListItem{
					item(
						para(txt("nested bullet")),
						&markdown.List{Items: []*markdown.ListItem{
							item(para(txt("deeper still"))),
						}},
					),
				}},
			),
			item(para(txt("third bullet"))),
		}},
		&markdown.List{Ordered: true, Start: 1, Items: []*markdown.ListItem{
			item(para(txt("step one"))),
			item(
				para(txt("step two")),
				&markdown.List{Ordered: true, Start: 1, Items: []*markdown.ListItem{
					item(para(txt("sub-step"))),
				}},
			),
		}},
		para(txt("Ordered lists can start anywhere:")),
		&markdown.List{Ordered: true, Start: 7, Items: []*markdown.ListItem{
			item(para(txt("seventh"))),
			item(para(txt("eighth"))),
		}},
		&markdown.Heading{Level: 3, Spans: []markdown.Span{txt("Tasks")}},
		&markdown.List{Items: []*markdown.ListItem{
			task(true, para(txt("shipped task"))),
			task(false, para(txt("open task"))),
		}},
		&markdown.Heading{Level: 4, Spans: []markdown.Span{txt("Quotes")}},
		&markdown.Blockquote{Blocks: []markdown.Block{
			para(txt("Quoted paragraph with "), ital("emphasis"), txt(" inside.")),
			&markdown.Blockquote{Blocks: []markdown.Block{
				para(txt("A nested quote one level deeper.")),
			}},
		}},
		&markdown.Heading{Level: 5, Spans: []markdown.Span{txt("Code")}},
		&markdown.CodeBlock{
			Language: "go",
			Code:     "func main() {\n    if true {\n        fmt.Println(\"tabs expand to spaces\")\n    }\n}",
		},
		&markdown.CodeBlock{Code: "indented code block\nsecond line"},
		&markdown.Heading{Level: 6, Spans: []markdown.Span{txt("Break below")}},
		&markdown.Rule{},
		para(txt("Soft break becomes a space, and a hard\nbreak becomes a new line.")),
	}

	if len(got) != len(want) {
		t.Fatalf("Parse returned %d top-level blocks, want %d", len(got), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("block %d:\n got  %#v\n want %#v", i, got[i], want[i])
		}
	}
}

// TestParseEmpty confirms empty and blank sources yield no blocks.
func TestParseEmpty(t *testing.T) {
	for _, src := range []string{"", "\n\n  \n"} {
		if got := markdown.Parse([]byte(src)); len(got) != 0 {
			t.Errorf("Parse(%q) = %#v, want no blocks", src, got)
		}
	}
}

// TestCodeTabExpansion pins the 4-column tab-stop expansion inside code
// blocks, including tabs that start mid-column.
func TestCodeTabExpansion(t *testing.T) {
	got := markdown.Parse([]byte("```\na\tb\nab\tc\n\td\n```\n"))
	if len(got) != 1 {
		t.Fatalf("Parse returned %d blocks, want 1", len(got))
	}
	cb, ok := got[0].(*markdown.CodeBlock)
	if !ok {
		t.Fatalf("block is %T, want *CodeBlock", got[0])
	}
	want := "a   b\nab  c\n    d"
	if cb.Code != want {
		t.Errorf("Code = %q, want %q", cb.Code, want)
	}
}
