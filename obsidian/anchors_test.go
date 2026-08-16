package obsidian

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vibrantgio/markdown"
)

// blockText flattens a block's visible text for assertions.
func blockText(b markdown.Block) string {
	var sb strings.Builder
	var spans func([]markdown.Span)
	spans = func(ss []markdown.Span) {
		for _, s := range ss {
			sb.WriteString(s.Text)
		}
	}
	var walk func(markdown.Block)
	walk = func(b markdown.Block) {
		switch b := b.(type) {
		case *markdown.Heading:
			spans(b.Spans)
		case *markdown.Paragraph:
			spans(b.Spans)
		case *markdown.List:
			for _, it := range b.Items {
				for _, c := range it.Blocks {
					walk(c)
				}
			}
		case *markdown.Blockquote:
			for _, c := range b.Blocks {
				walk(c)
			}
		case *markdown.CodeBlock:
			sb.WriteString(b.Code)
		}
	}
	walk(b)
	return sb.String()
}

func TestBlockAnchors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		anchors map[string]int
		text    []string // visible text per surviving top-level block
	}{
		{
			name:    "paragraph tail",
			src:     "First.\n\nAnchored here. ^ref-1\n\nLast.\n",
			anchors: map[string]int{"ref-1": 1},
			text:    []string{"First.", "Anchored here.", "Last."},
		},
		{
			name:    "list item tails anchor the list",
			src:     "intro\n\n- one ^a1\n- two\n- three ^a2\n",
			anchors: map[string]int{"a1": 1, "a2": 1},
			text:    []string{"intro", "onetwothree"},
		},
		{
			name:    "own-line id below a table anchors the table",
			src:     "| h |\n| --- |\n| c |\n\n^tbl\n\nafter\n",
			anchors: map[string]int{"tbl": 0},
			text:    []string{"", "after"},
		},
		{
			name:    "quoted paragraph tail anchors the quote",
			src:     "> quoted line ^q1\n",
			anchors: map[string]int{"q1": 0},
			text:    []string{"quoted line"},
		},
		{
			name:    "no ids no change",
			src:     "plain ^ not-an-id\n\ncaret^inline stays\n",
			anchors: map[string]int{},
			text:    []string{"plain ^ not-an-id", "caret^inline stays"},
		},
		{
			name:    "code is never an anchor",
			src:     "```\ncode ^c1\n```\n\ntext with `code ^c2` inline\n",
			anchors: map[string]int{},
			text:    []string{"code ^c1", "text with code ^c2 inline"},
		},
		{
			name:    "invalid id characters are left alone",
			src:     "not stripped ^has_underscore\n",
			anchors: map[string]int{},
			text:    []string{"not stripped ^has_underscore"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, anchors := BlockAnchors(markdown.Parse([]byte(tt.src)))
			if !reflect.DeepEqual(anchors, tt.anchors) {
				t.Errorf("anchors = %v, want %v", anchors, tt.anchors)
			}
			if len(blocks) != len(tt.text) {
				t.Fatalf("got %d blocks, want %d", len(blocks), len(tt.text))
			}
			for i, want := range tt.text {
				if got := blockText(blocks[i]); got != want {
					t.Errorf("block %d text = %q, want %q", i, got, want)
				}
			}
			for id := range anchors {
				if at := anchors[id]; at < 0 || at >= len(blocks) {
					t.Errorf("anchor %q index %d out of range for NewDocumentAt", id, at)
				}
			}
		})
	}
}
