package obsidian

import (
	"bytes"
	"reflect"
	"testing"
)

func TestSplitFrontMatter(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		present bool
		raw     string
		body    string
	}{
		{
			name:    "plain block",
			src:     "---\ntitle: A\n---\nbody\n",
			present: true,
			raw:     "title: A\n",
			body:    "body\n",
		},
		{
			name:    "dots terminator",
			src:     "---\ntitle: A\n...\nbody\n",
			present: true,
			raw:     "title: A\n",
			body:    "body\n",
		},
		{
			name:    "empty block",
			src:     "---\n---\nbody",
			present: true,
			raw:     "",
			body:    "body",
		},
		{
			name:    "terminator is last line without newline",
			src:     "---\nk: v\n---",
			present: true,
			raw:     "k: v\n",
			body:    "",
		},
		{
			name:    "crlf delimiters",
			src:     "---\r\nk: v\r\n---\r\nbody\r\n",
			present: true,
			raw:     "k: v\r\n",
			body:    "body\r\n",
		},
		{
			name: "unterminated left alone",
			src:  "---\ntitle: A\nno end",
			body: "---\ntitle: A\nno end",
		},
		{
			name: "opening dashes only",
			src:  "---\n",
			body: "---\n",
		},
		{
			name: "mid-document rule left alone",
			src:  "intro\n\n---\n\nafter\n",
			body: "intro\n\n---\n\nafter\n",
		},
		{
			name: "indented dashes are not frontmatter",
			src:  " ---\nk: v\n---\n",
			body: " ---\nk: v\n---\n",
		},
		{
			name: "longer dash run is not a delimiter",
			src:  "----\nk: v\n----\n",
			body: "----\nk: v\n----\n",
		},
		{
			name: "no frontmatter passthrough",
			src:  "# Heading\n\nprose\n",
			body: "# Heading\n\nprose\n",
		},
		{
			name: "empty source",
			src:  "",
			body: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := SplitFrontMatter([]byte(tt.src))
			if fm.Present != tt.present {
				t.Errorf("Present = %v, want %v", fm.Present, tt.present)
			}
			if fm.Raw != tt.raw {
				t.Errorf("Raw = %q, want %q", fm.Raw, tt.raw)
			}
			if string(body) != tt.body {
				t.Errorf("body = %q, want %q", body, tt.body)
			}
			if !tt.present && !bytes.Equal(body, []byte(tt.src)) {
				t.Errorf("absent frontmatter must pass the source through byte-identical")
			}
		})
	}
}

func TestFrontMatterFields(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []Field
	}{
		{
			name: "real properties block",
			src: "---\n" +
				"title: Weekly review\n" +
				"date: 2026-08-16\n" +
				"tags:\n" +
				"  - project\n" +
				"  - design-system\n" +
				"published: true\n" +
				"rating: 4\n" +
				"---\n",
			want: []Field{
				{Key: "title", Value: "Weekly review"},
				{Key: "date", Value: "2026-08-16"},
				{Key: "tags", Items: []string{"project", "design-system"}},
				{Key: "published", Value: "true"},
				{Key: "rating", Value: "4"},
			},
		},
		{
			name: "aliases and cssclasses lists, unindented dashes",
			src: "---\n" +
				"aliases:\n" +
				"- Review\n" +
				"- Weekly\n" +
				"cssclasses:\n" +
				"  - wide\n" +
				"---\n",
			want: []Field{
				{Key: "aliases", Items: []string{"Review", "Weekly"}},
				{Key: "cssclasses", Items: []string{"wide"}},
			},
		},
		{
			name: "empty property and quoted scalar kept literal",
			src: "---\n" +
				"summary:\n" +
				"source: \"https://example.org/a: b\"\n" +
				"flow: [a, b]\n" +
				"---\n",
			want: []Field{
				{Key: "summary"},
				{Key: "source", Value: "\"https://example.org/a: b\""},
				{Key: "flow", Value: "[a, b]"},
			},
		},
		{
			name: "nested map stays raw",
			src: "---\n" +
				"meta:\n" +
				"  author: someone\n" +
				"title: Kept\n" +
				"---\n",
			want: []Field{
				{Key: "title", Value: "Kept"},
			},
		},
		{
			name: "blank lines and lone dash item",
			src: "---\n" +
				"\n" +
				"tags:\n" +
				"  -\n" +
				"  - one\n" +
				"\n" +
				"done: false\n" +
				"---\n",
			want: []Field{
				{Key: "tags", Items: []string{"", "one"}},
				{Key: "done", Value: "false"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, _ := SplitFrontMatter([]byte(tt.src))
			if !fm.Present {
				t.Fatalf("frontmatter not recognised")
			}
			if !reflect.DeepEqual(fm.Fields, tt.want) {
				t.Errorf("Fields = %#v, want %#v", fm.Fields, tt.want)
			}
		})
	}
}
