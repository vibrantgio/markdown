package obsidian

import (
	"reflect"
	"testing"

	"github.com/vibrantgio/markdown"
)

// para wraps spans in a single-paragraph block slice.
func para(spans ...markdown.Span) []markdown.Block {
	return []markdown.Block{&markdown.Paragraph{Spans: spans}}
}

// firstSpans digs the spans back out of the first paragraph.
func firstSpans(t *testing.T, blocks []markdown.Block) []markdown.Span {
	t.Helper()
	p, isPara := blocks[0].(*markdown.Paragraph)
	if !isPara {
		t.Fatalf("first block is %T, want *markdown.Paragraph", blocks[0])
	}
	return p.Spans
}

func TestWikiSpans(t *testing.T) {
	tests := []struct {
		name string
		in   []markdown.Span
		want []markdown.Span
	}{
		{
			name: "plain link",
			in:   []markdown.Span{{Text: "see [[Other Note]] for more"}},
			want: []markdown.Span{
				{Text: "see "},
				{Text: "Other Note", URL: "wiki:Other Note"},
				{Text: " for more"},
			},
		},
		{
			name: "alias displays alias, url keeps raw body",
			in:   []markdown.Span{{Text: "[[Other Note|the alias]]"}},
			want: []markdown.Span{
				{Text: "the alias", URL: "wiki:Other Note|the alias"},
			},
		},
		{
			name: "heading path",
			in:   []markdown.Span{{Text: "[[Folder/Deep#Sec#Sub]]"}},
			want: []markdown.Span{
				{Text: "Folder/Deep#Sec#Sub", URL: "wiki:Folder/Deep#Sec#Sub"},
			},
		},
		{
			name: "same-file heading",
			in:   []markdown.Span{{Text: "[[#Heading]]"}},
			want: []markdown.Span{
				{Text: "#Heading", URL: "wiki:#Heading"},
			},
		},
		{
			name: "block ref",
			in:   []markdown.Span{{Text: "[[Note#^ab-12]]"}},
			want: []markdown.Span{
				{Text: "Note#^ab-12", URL: "wiki:Note#^ab-12"},
			},
		},
		{
			name: "embed gets the wikiembed scheme",
			in:   []markdown.Span{{Text: "before ![[img.png]] after"}},
			want: []markdown.Span{
				{Text: "before "},
				{Text: "img.png", URL: "wikiembed:img.png"},
				{Text: " after"},
			},
		},
		{
			name: "adjacent links stay separate",
			in:   []markdown.Span{{Text: "[[One]][[Two]]"}},
			want: []markdown.Span{
				{Text: "One", URL: "wiki:One"},
				{Text: "Two", URL: "wiki:Two"},
			},
		},
		{
			name: "code span skipped",
			in:   []markdown.Span{{Text: "[[not-a-link]]", Code: true}},
			want: []markdown.Span{{Text: "[[not-a-link]]", Code: true}},
		},
		{
			name: "existing url skipped",
			in:   []markdown.Span{{Text: "[[kept]]", URL: "https://example.org"}},
			want: []markdown.Span{{Text: "[[kept]]", URL: "https://example.org"}},
		},
		{
			name: "styling inherited by the link span",
			in:   []markdown.Span{{Text: "read [[Note]]!", Bold: true}},
			want: []markdown.Span{
				{Text: "read ", Bold: true},
				{Text: "Note", URL: "wiki:Note", Bold: true},
				{Text: "!", Bold: true},
			},
		},
		{
			name: "empty target is not a link",
			in:   []markdown.Span{{Text: "empty [[]] and [[ |x]] stay"}},
			want: []markdown.Span{{Text: "empty [[]] and [[ |x]] stay"}},
		},
		{
			name: "empty alias falls back to target",
			in:   []markdown.Span{{Text: "[[Note|]]"}},
			want: []markdown.Span{{Text: "Note", URL: "wiki:Note|"}},
		},
		{
			name: "no wikilink passes through untouched",
			in:   []markdown.Span{{Text: "just [brackets] and text"}},
			want: []markdown.Span{{Text: "just [brackets] and text"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstSpans(t, WikiSpans(para(tt.in...)))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("spans = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestWikiSpansStylingBoundary pins the documented limitation: a wikilink
// whose body crosses a styling boundary arrives as multiple spans and is
// therefore not recognised.
func TestWikiSpansStylingBoundary(t *testing.T) {
	blocks := markdown.Parse([]byte("[[a *b*]]\n"))
	got := firstSpans(t, WikiSpans(blocks))
	for _, s := range got {
		if s.URL != "" {
			t.Fatalf("styling-boundary wikilink was recognised as a link: %#v", got)
		}
	}
}

// TestWikiSpansEscapedBrackets pins the other documented limitation. The
// spans this function reads are finished text: the parser has already turned
// "\[\[" into "[[", so escaped brackets no longer hold a wikilink off — the
// information that they were escaped is gone by the time the spans arrive.
// An escape inside a body, on the other hand, lands on the right target.
func TestWikiSpansEscapedBrackets(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []markdown.Span
	}{{
		name: "escaped brackets no longer hold the link off",
		src:  `\[\[Note\]\]`,
		want: []markdown.Span{{Text: "Note", URL: "wiki:Note"}},
	}, {
		name: "an escape inside the body reaches the unescaped target",
		src:  `[[q5\_0]]`,
		want: []markdown.Span{{Text: "q5_0", URL: "wiki:q5_0"}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstSpans(t, WikiSpans(markdown.Parse([]byte(tt.src))))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("spans = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestWikiSpansContainers checks the walk reaches headings, list items,
// blockquotes and table cells, and leaves code blocks alone.
func TestWikiSpansContainers(t *testing.T) {
	src := "# See [[Home]]\n\n" +
		"- item [[One]]\n" +
		"  - nested [[Two]]\n\n" +
		"> quoted [[Three]]\n\n" +
		"| h [[Four]] |\n| --- |\n| c [[Five]] |\n\n" +
		"```\n[[fenced]]\n```\n"
	blocks := WikiSpans(markdown.Parse([]byte(src)))

	var urls []string
	var walkSpans func(spans []markdown.Span)
	walkSpans = func(spans []markdown.Span) {
		for _, s := range spans {
			if s.URL != "" {
				urls = append(urls, s.URL)
			}
		}
	}
	var walk func(bs []markdown.Block)
	walk = func(bs []markdown.Block) {
		for _, b := range bs {
			switch b := b.(type) {
			case *markdown.Heading:
				walkSpans(b.Spans)
			case *markdown.Paragraph:
				walkSpans(b.Spans)
			case *markdown.List:
				for _, it := range b.Items {
					walk(it.Blocks)
				}
			case *markdown.Blockquote:
				walk(b.Blocks)
			case *markdown.Table:
				for _, c := range b.Header {
					walkSpans(c.Spans)
				}
				for _, row := range b.Rows {
					for _, c := range row {
						walkSpans(c.Spans)
					}
				}
			case *markdown.CodeBlock:
				if b.Code != "[[fenced]]" {
					t.Errorf("code block content changed: %q", b.Code)
				}
			}
		}
	}
	walk(blocks)

	want := []string{"wiki:Home", "wiki:One", "wiki:Two", "wiki:Three", "wiki:Four", "wiki:Five"}
	if !reflect.DeepEqual(urls, want) {
		t.Errorf("urls = %v, want %v", urls, want)
	}
}
