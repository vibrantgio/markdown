# AGENTS.md — markdown

Document-grade Markdown rendering for Gio: a goldmark AST with GFM walked
into a block model and laid out with prism primitives — headings on the
token type scale, richtext paragraphs, nested lists with task boxes,
blockquotes, rules, fenced code with tab expansion and optional chroma
highlighting, GFM tables, and images through a caller-supplied provider
that does the I/O. The goldmark dependency stops here and the chroma
dependency stops in `highlight`, so prism never sees either.

**Layer.** Tier 4 of ADR-001's stack, `mvu → spectrum → prism → pulse →
cadence → markdown`, alongside cadence. It imports `prism/list`,
`prism/richtext`, `prism/scrollbar` and `prism/tokens`, plus the support
libraries svg and `svg/driver/gio`; it does not import mvu, spectrum, pulse
or cadence at all. Nothing in the design system imports markdown — the
workbench applications `mindchat` and `sitedocs` are its consumers.

**Read the canonical guide before you write code against this module.** It is
the organization's one agent guide — the module inventory with current tags,
the application skeleton, the MVU loop and rx semantics, typography, and the
pitfalls that are not guessable. It lives exactly once, in `vibrantgio/.github`,
and this file links it rather than copying it:

    https://raw.githubusercontent.com/vibrantgio/.github/master/llms.txt

**Module.** `github.com/vibrantgio/markdown`, one module at the repository
root.

**Build and test.** From the repository root:

    go build ./... && go test ./...

**Golden images.** Tests in four packages compare rendered output against
PNGs committed under `testdata/golden/`. When a change legitimately moves
pixels, regenerate them within the same change, look at what came out, and
say so in the commit. From the repository root:

    go test ./... -golden.update

The flag comes last on purpose: `go test` cannot tell that an unfamiliar
flag is boolean, so anything after it stops being a package argument. `go
test -golden.update ./...` tests whatever package the repository root
holds, not `./...`.
