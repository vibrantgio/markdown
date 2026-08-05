# markdown

Document-grade markdown rendering for [Gio](https://gioui.org), part of
[Vibrant Gio](https://github.com/vibrantgio) — a design system for native
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
alongside [cadence](https://github.com/vibrantgio/cadence). It imports `list`
and `richtext` from [prism](https://github.com/vibrantgio/prism), `tokens` from
[spectrum](https://github.com/vibrantgio/spectrum), and
[svg](https://github.com/vibrantgio/svg) in the `svgimage` subpackage only. It
imports neither mvu, pulse nor cadence, and nothing in the design
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

## The monospace font comes from the theme

`FromTokens` resolves `Style.Mono` and `Style.CodeSize` from the theme's Code
role (`spectrum/tokens.DefaultTypography.Code`): `"Roboto Mono"`, which the
default collection (`tokens.DefaultTypography.Faces`) carries in all four
weight/style combinations code shapes in — regular, bold, italic, and bold
italic. Out of the box, code blocks and inline code render monospaced.

One caveat survives for applications building their own shaper: Gio matches
typeface names literally, with no CSS generic-family fallback and no error or
warning on a miss — a collection without a `Roboto Mono` face silently shapes
code in the proportional body face instead. Include
`vibrantgio/font/robotomono`'s faces in the collection, or point `Style.Mono`
at a monospace family the collection does hold:

```go
// The theme typography's Roboto faces lead (the shaper's default), followed
// by the Roboto Mono faces markdown code spans resolve against.
faces := append(slices.Clone(roboto.FontFaces()), robotomono.FontFaces()...)
shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces))
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

- **Code sizing does not follow a scaled `TypeScale`.** `TypeScale` has no
  code stop — the Code role lives on `tokens.Typography`, outside the MD3
  grid — so `FromTokens`, whose signature is frozen on `TypeScale`, reads
  `CodeSize` from the default Code role. A caller passing a scaled
  `TypeScale` scales headings and body but not code; set `Style.CodeSize`
  from your own Code role after `FromTokens`.
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
