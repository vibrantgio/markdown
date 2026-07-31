# markdown

Document-grade markdown rendering for [Gio](https://gioui.org), part of
[VibrantGio](https://github.com/vibrantgio) — a design system for native
desktop applications on macOS, Windows and Linux, written in pure Go. Gio gives
you a text shaper and a paint API and has no notion of a document. This module
walks a [goldmark](https://github.com/yuin/goldmark) AST into a block model and
lays that model out with [prism](https://github.com/vibrantgio/prism)
primitives, so a chat message, a README or a documentation page renders as a
real document — headings on the type scale, bordered tables, code on a surface
— rather than as one long paragraph.

The package walks a goldmark AST (with `extension.GFM`) into a block model and
renders it with prism primitives:

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

## Where it sits

Tier 4 of the stack — `mvu → spectrum → prism → pulse → cadence → markdown` —
alongside [cadence](https://github.com/vibrantgio/cadence). It imports `list`,
`richtext` and `tokens` from [prism](https://github.com/vibrantgio/prism), and
[svg](https://github.com/vibrantgio/svg) in the `svgimage` subpackage only. It
imports neither mvu, spectrum, pulse nor cadence, and nothing in the design
system imports it: its consumers are the
[workbench](https://github.com/vibrantgio/workbench) applications `mindchat`
and `sitedocs`. The [organization page](https://github.com/vibrantgio) has the
full stack.

## Packages

| package | what it does |
| --- | --- |
| `markdown` | `Parse` a source into a block model, `Document` to lay it out, `Style` to theme it. Carries goldmark, and nothing heavier. |
| `markdown/highlight` | A chroma-backed `Highlighter` for fenced code. Importing this package is what pulls chroma into a build. |
| `markdown/svgimage` | An image provider serving `.svg` destinations as vector widgets through `svg/driver/gio`. Importing this package is what pulls svg in. |

## Usage

From `sitedocs`, the workbench documentation browser:

```go
// Built once and shared by every page. FromTokens leaves Highlight nil, so
// assigning a highlighter is the application's opt-in to syntax colouring —
// and there are two, because chroma's colours are its own, not the theme's.
var (
    highlightLight = highlight.New("github")
    highlightDark  = highlight.New("github-dark")
)

func docsStyle(c tokens.ColorTokens, ts tokens.TypeScale) markdown.Style {
    st := markdown.FromTokens(c, ts)
    if isDarkColor(c.Background) {
        st.Highlight = highlightDark
    } else {
        st.Highlight = highlightLight
    }
    st.Text.OnLinkClick = func(_ layout.Context, url string) { openURL(url) }
    return st
}
```

Allocate the `Document` once and reuse it on every frame — it holds the scroll
position and the per-block interaction state (link focus and hover, code block
scroll, the resolved image for each block):

```go
doc := markdown.NewDocument(markdown.Parse(source))
// ...then, inside the layer:
doc.Layout(gtx, shaper, docsStyle(colors, typeScale))
```

## The application supplies the monospace font

`FromTokens` sets `Style.Mono` to `"Go Mono, monospace"`, and this module ships
no font at all. If the shaper's collection holds no `Go Mono` typeface, code
renders in the proportional body face with no error and no warning — measured
pixel-identical to leaving `Style.Mono` empty. Gio matches typeface names
literally and has no CSS generic-family fallback, so the `, monospace` half of
that string resolves nothing on its own; only the `Go Mono` token does any
work. `mindchat` handles it like this:

```go
// The Roboto faces lead (the shaper's default), followed by the Go
// collection so markdown code spans resolve their "Go Mono" typeface.
shaper := text.NewShaper(text.WithCollection(append(style.FontFaces(), gofont.Collection()...)))
```

## Why not `gioui.org/x/markdown`?

The existing community renderer was evaluated on 2026-07-20 and rejected as a
dependency: it flattens the whole document into a single richtext flow and
drops blockquotes, thematic rules, images, tables, list nesting, and all of
GFM; headings are distinguished by size only; tabs render as tofu; there is no
text selection. It served as a span-model reference only — the span shape
lives on in `prism/richtext`, and this module owns the block layer that
`x/markdown` lacks.

## For coding assistants

`AGENTS.md` in this repository points at the organization's canonical guide,
which is the one place the module inventory, the application skeleton, the MVU
loop and rx semantics, typography and the non-guessable pitfalls are written
down. Read it before writing code against this module:

<https://raw.githubusercontent.com/vibrantgio/.github/master/llms.txt>

## Status

Current tag `v0.0.6`. What renders, renders well; these are the honest gaps.

- **Typography is hard-coded, not themed.** `Style.Mono` is the string
  `"Go Mono, monospace"` and the type scale reaches this module as sizes only,
  with no seam for a typeface — the section above is the workaround. Phase C
  (task C2.8) migrates the renderer, `highlight` and `svgimage` to theme
  typography and removes the last `gofont` import, tests included.
- **Syntax highlighting does not follow the theme.** A `Highlighter` colours
  every run it emits, so `Style.CodeColor` is never reached and only the code
  block's background stays themed. The application must build one highlighter
  per appearance and swap them, as the example above does; an unrecognised
  chroma style name is not an error but falls back to a dark-background
  default, which on a light theme is near-white text on `SurfaceVariant`.
- **Text is not selectable or copyable.** Neither this module nor
  `prism/richtext` implements selection — the same gap the comparison above
  notes in `x/markdown`. Links are clickable and focusable; that is all.
- **Image results are cached per document, failures included.** A destination
  that fails to load keeps rendering its alt text for the life of that
  `Document`, even if the file later appears. Build a new `Document` to retry.

## License

MIT — see [LICENSE](./LICENSE).
