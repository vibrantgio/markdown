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

- headings on the `tokens` document heading scale — a ladder stepped off the
  body role for reading surfaces rather than the display roles that size a
  screen's own headline — each carrying its own vertical space, wider above
  than below, so a heading parts from the section it closes and binds to the
  one it opens
- paragraphs as `components/richtext` span flows (bold, italic, inline code,
  links, GFM strikethrough)
- ordered/unordered lists with real nesting and indentation, including GFM
  task-list checkboxes
- blockquotes as inset columns with a leading token-coloured bar
- thematic breaks as rules
- fenced and indented code blocks as monospace on a surface background, with
  tab expansion, optional syntax highlighting through the `Highlighter` hook
  on `Style`, and horizontal scrolling for lines too wide for the column —
  the fence is a `components/scrollarea`, so an over-wide line is scrolled to
  rather than wrapped or cut, the cut edge dissolves into the fence while
  there is more past it, a slim bar rides the fence's bottom padding while it
  moves, and the fence claims the horizontal axis only so a wheel over it
  still scrolls the document
- GFM tables as grids with an emphasised header row, token borders, and
  per-column alignment
- images through a caller-supplied `ImageProvider` (the library performs no
  I/O), rendered with `widget.Image` and falling back to alt text

The document widget lays top-level blocks through `components/list`, so long
documents stay O(visible).

Notes written in the Obsidian dialect — YAML-style frontmatter, `[[wikilink]]`
and `![[embed]]` syntax, trailing `^block-id` anchors — are recognised by the
`markdown/obsidian` subpackage, which works on the source and on the public
block model rather than inside the parser. It recognises; it resolves nothing:
a wikilink becomes an ordinary link span carrying the raw link body, so the
application decides what the target means.

## Where it sits

Tier 4 of the stack — `mvu → theme → components → effects → patterns → markdown` —
alongside [patterns](https://github.com/vibrantgio/patterns). It imports `list`
and `richtext` from [components](https://github.com/vibrantgio/components), `tokens` from
[theme](https://github.com/vibrantgio/theme), and
[svg](https://github.com/vibrantgio/svg) in the `svgimage` subpackage only. It
imports neither mvu, effects nor patterns, and nothing in the design
system imports it: it is consumed by applications, at the top of the stack,
and `AGENTS.md` names them from the measured graph rather than from memory.
The [organization page](https://github.com/vibrantgio) has the full stack.

## Packages

| package | what it does |
| --- | --- |
| `markdown` | `Parse` a source into a block model, `Document` to lay it out, `Style` to theme it. Carries goldmark, and nothing heavier. |
| `markdown/highlight` | A chroma-backed `Highlighter` for fenced code — `New` to wear a stock style, `Adapt` to derive one fitted to your tokens. Importing this package is what pulls chroma into a build; no chroma type reaches its exported API. |
| `markdown/svgimage` | An image provider serving `.svg` destinations as vector widgets through `svg/driver/gio`. Importing this package is what pulls svg in. |
| `markdown/obsidian` | Recognition of the Obsidian dialect around `Parse`: `SplitFrontMatter` before it, `WikiSpans` and `BlockAnchors` after it. Pure Go, no dependency beyond the parent package. |

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

`New` wears a stock style exactly as its author wrote it, which means its
inks were fitted to that author's background rather than to the fill your
theme puts under a fence. `Adapt` derives a style instead: it holds each
entry's hue and chroma and re-fits the lightness against your own code
surface until every ink clears the WCAG AA contrast ratio, settles one bold
and italic policy across the light and dark members of the pair, and keeps
the plain-foreground fallback. One base name covers both appearances — which
member is derived from follows the tokens — so a single line replaces the
pair of highlighters above, and it re-derives with the theme rather than
staying where it was built:

```go
st := markdown.FromTokens(c, typo)
st.Highlight = highlight.Adapt(highlight.DefaultBase, c)
```

`DefaultBase` is `catppuccin-latte`, whose registered counterpart
`catppuccin-mocha` is the dark member that same name reaches. It is a
default, not a policy: pass any name chroma's registry holds. It is the
default because its accents already sit in one perceptual-lightness band
with the hues carrying the semantics, which is exactly the shape a
lightness re-fit leaves intact — a palette that told a keyword from a
string by how dark it was would come out of the same fit with the two
harder to tell apart.

The choice is not limited to what ships embedded. A chroma style is a small
XML document, and `highlight.LoadDir` reads a folder of them and makes each
one choosable by its own name:

```go
loaded, skipped := highlight.LoadDir(dir) // names, and files with reasons
names := highlight.Bases()                // embedded and loaded, sorted
base := highlight.BaseOrDefault(chosen)   // a name that no longer resolves
```

A folder that is not there loads nothing and reports nothing; a file that
will not parse, or one claiming the name of an embedded style, is skipped
and named with its reason rather than thrown, so one bad file never costs
the rest of the folder. Loaded styles are held beside chroma's registry and
never inside it.

Derive once per theme, not once per frame: the walk over a base's entry
table is cheap but not free. Stock styles are untouched by any of this —
adaptation builds a new style beside the registry and never mutates it, so
`New("github")` still yields exactly what chroma ships.

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

## Inline code is sized for the line it sits in

A code span takes `Style.CodeSize` too — the same size a fence is set at, held
as a proportion of the line so a span inside a heading takes the heading's own
size less that proportion. It is not a stylistic preference: at the body's own
size the monospace face asks for more ascent than the body face does, and
since every segment on a line shares one baseline, the taller ascent pushes
the whole line's baseline down and out from under everything hung beside it —
a list's markers first of all, which is how the defect showed itself.

The span then sits on `Style.CodeChip`: a rounded fill one ramp step off the
page, padded horizontally, taking the code's own shaped height so it can
never stretch the line it is quoted into. `FromTokens` sets it; a `Style`
built by hand leaves it zero and inline code sits on the page, as it did
before.

## The Obsidian dialect

`markdown/obsidian` adds recognition of the three things Obsidian-flavoured
notes carry that GitHub-flavoured markdown does not. The parser is untouched:
one pass runs before it and two run over the block model it returns.

```go
fm, body := obsidian.SplitFrontMatter(src)          // properties off the top
blocks, anchors := obsidian.BlockAnchors(           // "^id" tails → indices
    obsidian.WikiSpans(markdown.Parse(body)))       // [[link]] → link spans
doc := markdown.NewDocumentAt(blocks, anchors["intro"])
```

- **Frontmatter** is split off as data before parsing, because a leading
  `---` block otherwise renders as a rule and a setext heading. The fields are
  read by a trivial line split — scalars and `- item` lists — with the raw
  text kept, so a document needing full YAML can hand `FrontMatter.Raw` to a
  parser of its choosing. No YAML dependency arrives here.
- **Wikilinks** — `[[target]]`, `[[target|alias]]` and the `![[target]]`
  embed form — become ordinary link spans whose URL is the raw link body
  under a `wiki:` scheme (`wikiembed:` for embeds). Nothing is resolved: what
  a target names is the application's question, and answering it needs the
  folder the notes live in, which this library never reads. Spans already
  marked code or already carrying a URL are left alone.
- **Block ids** — a trailing ` ^id` on a paragraph or list item, or an `^id`
  written on its own line under a table or fence — are stripped from what is
  displayed and returned as a map from id to top-level block index, which is
  what `NewDocumentAt` takes.

One limitation is by construction and pinned by a test: a span is a styling
run, so a wikilink whose body crosses a styling boundary (`[[a *b*]]`) is not
recognised. Obsidian link targets do not carry markdown styling, so this
excludes approximately nothing.

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

Current tag `v0.2.0` — a pre-release number, like every tag in the
organization. What renders, renders well; these are the honest gaps.

- **v0.2.0 is additive.** It adds the `markdown/obsidian` subpackage and
  changes nothing that existed before it: the parser, the block model, the
  document widget and `Style` are untouched, and the stored golden images
  are byte-identical across the release. A consumer that does not import the
  new subpackage sees no difference.
- **v0.1.0 was a breaking release.** `FromTokens` takes a
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
  remain the chroma style's own, and a style whose author drew them on a
  near-white page may leave them short of AA on a tinted code fill —
  `highlight.Adapt` is the constructor that re-fits them, and it takes one
  base name for both appearances where `New` needs one style per appearance.
  An unrecognised chroma style name panics in either — chroma's silent
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
