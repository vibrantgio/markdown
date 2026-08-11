# markdown

Document-grade markdown rendering for [Gio](https://gioui.org), part of
[Vibrant Gio](https://github.com/vibrantgio) — a design system for native
desktop applications on macOS, Windows and Linux, written in pure Go. Gio gives
you a text shaper and a paint API and has no notion of a document. This module
walks a [goldmark](https://github.com/yuin/goldmark) AST into a block model and
lays that model out with [components](https://github.com/vibrantgio/components)
primitives, so a chat message, a README or a documentation page renders as a
real document — headings on the type scale, bordered tables, code on a surface
— rather than as one long paragraph.

The package walks a goldmark AST (with `extension.GFM`) into a block model and
renders it with components primitives:

- headings on the `tokens` typography scale
- paragraphs as `components/richtext` span flows (bold, italic, inline code,
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

The document widget lays top-level blocks through `components/list`, so long
documents stay O(visible).

## Where it sits

Tier 4 of the stack — `mvu → theme → components → effects → cadence → markdown` —
alongside [cadence](https://github.com/vibrantgio/cadence). It imports `list`
and `richtext` from [components](https://github.com/vibrantgio/components), `tokens` from
[theme](https://github.com/vibrantgio/theme), and
[svg](https://github.com/vibrantgio/svg) in the `svgimage` subpackage only. It
imports neither mvu, effects nor cadence, and nothing in the design
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
// and there are two, because the keyword and string colours are chroma's
// own, not the theme's; only the plain runs follow Style.CodeColor.
var (
    highlightLight = highlight.New("github")
    highlightDark  = highlight.New("github-dark")
)

func docsStyle(c tokens.ColorTokens, typo tokens.Typography) markdown.Style {
    st := markdown.FromTokens(c, typo)
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
doc.Layout(gtx, shaper, docsStyle(colors, typography))
```

## The monospace font comes from the theme

`FromTokens` resolves `Style.Mono` and `Style.CodeSize` from the Code role of
the `tokens.Typography` it is handed — `"Roboto Mono"` for the default, which
the default collection (`tokens.DefaultTypography.Faces`) carries in all four
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
lives on in `components/richtext`, and this module owns the block layer that
`x/markdown` lacks.

## For coding assistants

`AGENTS.md` in this repository points at the organization's canonical guide,
which is the one place the module inventory, the application skeleton, the MVU
loop and rx semantics, typography and the non-guessable pitfalls are written
down. Read it before writing code against this module:

<https://raw.githubusercontent.com/vibrantgio/.github/master/llms.txt>

## Status

Current tag `v0.1.0` — a pre-release number, like every tag in the
organization. What renders, renders well; these are the honest gaps.

- **v0.1.0 is a breaking release.** `FromTokens` takes a
  `tokens.Typography` where it took a `tokens.TypeScale`:
  `markdown.FromTokens(c, tokens.DefaultTypography)`. `TypeScale` is gone
  from spectrum as of v0.3.0, and it never had a code stop — the Code role
  lives on `Typography`, outside the MD3 grid — so the old constructor had
  to read `Mono` and `CodeSize` off `tokens.DefaultTypography` no matter
  what typography the theme carried, and a caller passing a scaled
  `TypeScale` scaled headings and body but not code. Code now follows the
  typography you hand it, and the "set `Style.CodeSize` afterwards"
  workaround is retired.
- **`doc.Layout`'s positional-shaper signature is unchanged.** The other
  half of the surface v0.1.0 was expected to re-cut stayed as it is; it
  costs nothing today and no consumer has asked.
- **Highlight colours are chroma's, except the plain runs.** Runs a chroma
  style would render in its plain-text foreground — whitespace, punctuation,
  plain identifiers — are emitted colourless and take `Style.CodeColor`, so
  plain code follows the token theme; keyword, string, and comment colours
  remain the chroma style's own. The application must still build one
  highlighter per appearance and swap them, as the example above does. An
  unrecognised chroma style name panics in `highlight.New` — chroma's silent
  fallback is a dark-background style that fails visibly on only one of the
  two themes, so a typo fails at construction instead.
- **Text is not selectable or copyable.** Neither this module nor
  `components/richtext` implements selection — the same gap the comparison above
  notes in `x/markdown`. Links are clickable and focusable; that is all.
- **Image results are cached per document, failures included.** A destination
  that fails to load keeps rendering its alt text for the life of that
  `Document`, even if the file later appears. Build a new `Document` to retry.

## License

MIT — see [LICENSE](./LICENSE).
