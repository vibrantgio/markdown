package obsidian

import (
	"strings"
	"testing"

	"github.com/vibrantgio/markdown"
)

// TestProbeNote drives a realistic note through the whole recognition
// pipeline — SplitFrontMatter, then markdown.Parse, then WikiSpans and
// BlockAnchors — and checks the result is clean prose with live wiki URLs,
// readable frontmatter and no visible id tails.
func TestProbeNote(t *testing.T) {
	src := []byte(`---
title: Probe Note
tags:
  - probe
  - dialect
status: draft
---
# Probe Note

An opening paragraph citing [[Other Note|a friendly alias]] and a deep
[[Folder/Deep#Sec]] target, plus an embed ![[diagram.png]].

A fact worth citing lives here. ^fact-1

- a list item pointing at [[Third]] ^item-1
- a plain item

` + "```" + `
[[not-a-link]] inside a fence ^not-an-anchor
` + "```" + `

Closing prose with a block ref link [[Other Note#^fact-9]].
`)

	fm, body := SplitFrontMatter(src)
	if !fm.Present {
		t.Fatalf("frontmatter not recognised")
	}
	if len(fm.Fields) != 3 {
		t.Fatalf("Fields = %#v, want title, tags, status", fm.Fields)
	}
	if fm.Fields[0].Value != "Probe Note" || len(fm.Fields[1].Items) != 2 {
		t.Errorf("frontmatter pairs misread: %#v", fm.Fields)
	}

	blocks := markdown.Parse(body)
	blocks = WikiSpans(blocks)
	blocks, anchors := BlockAnchors(blocks)

	// The frontmatter never leaks into the rendered blocks.
	if _, isRule := blocks[0].(*markdown.Rule); isRule {
		t.Fatalf("frontmatter delimiter leaked through as a Rule")
	}
	h, isHeading := blocks[0].(*markdown.Heading)
	if !isHeading || h.Spans[0].Text != "Probe Note" {
		t.Fatalf("first block = %#v, want the H1", blocks[0])
	}

	// Live wiki URLs, embeds included, and no bracket syntax on display.
	var urls []string
	var visible strings.Builder
	var walk func([]markdown.Block)
	walk = func(bs []markdown.Block) {
		for _, b := range bs {
			switch b := b.(type) {
			case *markdown.Heading:
				collect(b.Spans, &urls, &visible)
			case *markdown.Paragraph:
				collect(b.Spans, &urls, &visible)
			case *markdown.List:
				for _, it := range b.Items {
					walk(it.Blocks)
				}
			case *markdown.Blockquote:
				walk(b.Blocks)
			}
		}
	}
	walk(blocks)

	for _, want := range []string{
		"wiki:Other Note|a friendly alias",
		"wiki:Folder/Deep#Sec",
		"wikiembed:diagram.png",
		"wiki:Third",
		"wiki:Other Note#^fact-9",
	} {
		if !contains(urls, want) {
			t.Errorf("missing URL %q in %v", want, urls)
		}
	}
	text := visible.String()
	if strings.Contains(text, "[[") || strings.Contains(text, "]]") {
		t.Errorf("bracket syntax visible in prose: %q", text)
	}
	if !strings.Contains(text, "a friendly alias") {
		t.Errorf("alias not displayed: %q", text)
	}
	if strings.Contains(text, "^fact-1") || strings.Contains(text, "^item-1") {
		t.Errorf("id tails visible in prose: %q", text)
	}

	// The anchors index into the returned blocks, ready for NewDocumentAt.
	for _, id := range []string{"fact-1", "item-1"} {
		at, exists := anchors[id]
		if !exists {
			t.Errorf("anchor %q missing: %v", id, anchors)
			continue
		}
		if at < 0 || at >= len(blocks) {
			t.Errorf("anchor %q index %d out of range", id, at)
		}
	}
	if _, exists := anchors["not-an-anchor"]; exists {
		t.Errorf("fenced id tail became an anchor")
	}

	// The fence survives untouched.
	var fence *markdown.CodeBlock
	for _, b := range blocks {
		if cb, isCode := b.(*markdown.CodeBlock); isCode {
			fence = cb
		}
	}
	if fence == nil || fence.Code != "[[not-a-link]] inside a fence ^not-an-anchor" {
		t.Errorf("code block altered: %#v", fence)
	}
}

func collect(spans []markdown.Span, urls *[]string, visible *strings.Builder) {
	for _, s := range spans {
		if s.URL != "" {
			*urls = append(*urls, s.URL)
		}
		visible.WriteString(s.Text)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
