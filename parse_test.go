package markdown_test

import (
	"bytes"
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

func cell(spans ...markdown.Span) *markdown.TableCell {
	return &markdown.TableCell{Spans: spans}
}

func task(checked bool, marker int, blocks ...markdown.Block) *markdown.ListItem {
	return &markdown.ListItem{Task: true, Checked: checked, MarkerOffset: marker, Blocks: blocks}
}

// TestParseCorpus asserts the corpus document's full block tree: every
// supported construct — heading levels 1–6, styled paragraph runs (bold, italic, bold
// italic, inline code, link, autolink, GFM strikethrough, soft and hard
// breaks), nested unordered and ordered lists, an ordered list with an
// explicit start, GFM task items, nested blockquotes, fenced code with
// language and tab expansion, indented code, and a thematic break.
func TestParseCorpus(t *testing.T) {
	src := corpus(t)
	got := markdown.Parse(src)

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
			task(true, bytes.Index(src, []byte("[x] shipped task")), para(txt("shipped task"))),
			task(false, bytes.Index(src, []byte("[ ] open task")), para(txt("open task"))),
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

// TestParseTable asserts a GFM table maps onto the Table block: delimiter-row
// alignments, styled header and body cells, and a short row padded with an
// empty cell to the column count.
func TestParseTable(t *testing.T) {
	src := "| Package | Role | Stars |\n" +
		"|:--------|:----:|------:|\n" +
		"| `prism` | primitives | 1200 |\n" +
		"| **markdown** | documents |\n"
	got := markdown.Parse([]byte(src))

	want := []markdown.Block{&markdown.Table{
		Alignments: []markdown.Alignment{
			markdown.AlignLeft, markdown.AlignCenter, markdown.AlignRight,
		},
		Header: []*markdown.TableCell{
			cell(txt("Package")), cell(txt("Role")), cell(txt("Stars")),
		},
		Rows: [][]*markdown.TableCell{
			{cell(code("prism")), cell(txt("primitives")), cell(txt("1200"))},
			{cell(bold("markdown")), cell(txt("documents")), cell()},
		},
	}}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse:\n got  %#v\n want %#v", got, want)
	}
}

// TestParseImage asserts a paragraph whose sole content is an image becomes
// an Image block, while an image mixed into text falls back to its alt-text
// runs inside the paragraph.
func TestParseImage(t *testing.T) {
	src := "![prism logo](https://vibrantgio.dev/prism.png)\n\n" +
		"An inline ![tiny icon](icon.png) stays text.\n"
	got := markdown.Parse([]byte(src))

	want := []markdown.Block{
		&markdown.Image{URL: "https://vibrantgio.dev/prism.png", Alt: "prism logo"},
		para(txt("An inline tiny icon stays text.")),
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse:\n got  %#v\n want %#v", got, want)
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

// taskOffsetSrc mixes the cases a marker offset has to record: nested and ordered
// items, [x] and [X], and a fence plus a code span that look like checkboxes
// but are not task items.
const taskOffsetSrc = "" +
	"- [x] shipped\n" +
	"- [ ] open\n" +
	"- [X] caps\n" +
	"1. [x] numbered\n" +
	"- [ ] parent\n" +
	"  - [x] child\n" +
	"```\n" +
	"- [x] fenced\n" +
	"```\n" +
	"- `[x]` codespan\n" +
	"- [ ] mix `[x]` still\n"

func markerAt(src []byte, snippet string) int {
	i := bytes.Index(src, []byte(snippet))
	if i < 0 {
		panic("snippet not in source: " + snippet)
	}
	return i
}

func collectTasks(blocks []markdown.Block) []*markdown.ListItem {
	var out []*markdown.ListItem
	var walk func([]markdown.Block)
	walk = func(bs []markdown.Block) {
		for _, b := range bs {
			switch b := b.(type) {
			case *markdown.List:
				for _, it := range b.Items {
					if it.Task {
						out = append(out, it)
					}
					walk(it.Blocks)
				}
			case *markdown.Blockquote:
				walk(b.Blocks)
			}
		}
	}
	walk(blocks)
	return out
}

// TestParseTaskMarkerOffset records the byte offset of each GFM task
// marker's opening '[' — nested, ordered, [x] and [X] — and confirms a
// checkbox inside a fence or a code span is not a task item.
func TestParseTaskMarkerOffset(t *testing.T) {
	src := []byte(taskOffsetSrc)
	got := markdown.Parse(src)

	want := []struct {
		snippet string
		checked bool
		text    string
	}{
		{"[x] shipped", true, "shipped"},
		{"[ ] open", false, "open"},
		{"[X] caps", true, "caps"},
		{"[x] numbered", true, "numbered"},
		{"[ ] parent", false, "parent"},
		{"[x] child", true, "child"},
		{"[ ] mix `[x]` still", false, "mix "},
	}

	tasks := collectTasks(got)
	if len(tasks) != len(want) {
		t.Fatalf("got %d task items, want %d", len(tasks), len(want))
	}
	for i, w := range want {
		it := tasks[i]
		off := markerAt(src, w.snippet)
		if it.MarkerOffset != off {
			t.Errorf("task %d (%q) MarkerOffset = %d, want %d", i, w.snippet, it.MarkerOffset, off)
		}
		if it.Checked != w.checked {
			t.Errorf("task %d (%q) Checked = %v, want %v", i, w.snippet, it.Checked, w.checked)
		}
		if it.MarkerOffset < 0 || it.MarkerOffset >= len(src) || src[it.MarkerOffset] != '[' {
			t.Errorf("task %d (%q) MarkerOffset %d is not '[' in the source", i, w.snippet, it.MarkerOffset)
		}
		if p, ok := it.Blocks[0].(*markdown.Paragraph); !ok || len(p.Spans) == 0 || p.Spans[0].Text != w.text {
			t.Errorf("task %d (%q) first text = %#v, want %q", i, w.snippet, it.Blocks, w.text)
		}
	}

	// A fence holding "- [x] fenced" is a code block, not a task list.
	var sawFence bool
	for _, b := range got {
		cb, ok := b.(*markdown.CodeBlock)
		if !ok {
			continue
		}
		sawFence = true
		if cb.Code != "- [x] fenced" {
			t.Errorf("fenced code = %q, want %q", cb.Code, "- [x] fenced")
		}
	}
	if !sawFence {
		t.Error("expected a fenced code block containing a lookalike checkbox")
	}

	// The codespan item is a list item whose first run is `[x]`, not a task.
	var sawCodeSpan bool
	for _, b := range got {
		l, ok := b.(*markdown.List)
		if !ok {
			continue
		}
		for _, it := range l.Items {
			if it.Task {
				continue
			}
			p, ok := it.Blocks[0].(*markdown.Paragraph)
			if !ok || len(p.Spans) == 0 || !p.Spans[0].Code || p.Spans[0].Text != "[x]" {
				continue
			}
			sawCodeSpan = true
			if it.MarkerOffset != 0 {
				t.Errorf("codespan item MarkerOffset = %d, want 0", it.MarkerOffset)
			}
		}
	}
	if !sawCodeSpan {
		t.Error("expected a list item whose first span is a `[x]` code span")
	}
}
