# markdown

Document-grade markdown rendering for [Gio](https://gioui.org), built on the
[prism](https://github.com/vibrantgio/prism) component foundation
(DESIGN §Markdown).

The package walks a [goldmark](https://github.com/yuin/goldmark) AST (with
`extension.GFM`) into a block model and renders it with prism primitives:

- headings on the `tokens` typography scale
- paragraphs as `prism/richtext` span flows (bold, italic, inline code,
  links, GFM strikethrough)
- ordered/unordered lists with real nesting and indentation, including GFM
  task-list checkboxes
- blockquotes as inset columns with a leading token-coloured bar
- thematic breaks as rules
- fenced and indented code blocks as monospace on a surface background with
  tab expansion, horizontal overflow scrolling, and optional syntax
  highlighting through the `Highlighter` hook on `Style`
- GFM tables as grids with an emphasised header row, token borders, and
  per-column alignment
- images through a caller-supplied `ImageProvider` (the library performs no
  I/O), rendered with `widget.Image` and falling back to alt text

The document widget lays top-level blocks through `prism/list`, so long
documents stay O(visible).

This module deliberately quarantines the goldmark dependency away from prism;
syntax highlighting (chroma) is likewise quarantined in the `highlight`
subpackage — assign `highlight.New("github")` (or any chroma style name) to
`Style.Highlight` — so the core package never imports chroma.

## Why not `gioui.org/x/markdown`?

The existing community renderer was evaluated on 2026-07-20 and rejected as a
dependency: it flattens the whole document into a single richtext flow and
drops blockquotes, thematic rules, images, tables, list nesting, and all of
GFM; headings are distinguished by size only; tabs render as tofu; there is no
text selection. It served as a span-model reference only — the span shape
lives on in `prism/richtext`, and this module owns the block layer that
`x/markdown` lacks.

## License

MIT — see [LICENSE](./LICENSE).
