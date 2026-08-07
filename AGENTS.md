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
`prism/richtext` and `prism/tokens`, and the support libraries svg and
`svg/driver/gio` in the `svgimage` subpackage only; it does not import mvu,
spectrum, pulse or cadence at all. Nothing in the design system imports
markdown — the workbench applications `mindchat` and `sitedocs` are its
consumers.

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

**Golden images.** Tests in three packages compare rendered output against
PNGs committed under `testdata/golden/`. They render through
`github.com/vibrantgio/prism/golden`, which declares `-golden.update` and is
shared with pulse, cadence and the workbench apps. F5.5 deleted
`markdown/internal/golden`, this repo's copy of it — one of twenty-nine in
the org — so do not reintroduce a local harness or a second
`-golden.update`: two registrations of one flag name in a test binary panic
at init.

When a change legitimately moves pixels, regenerate them within the same
change, look at what came out, and say so in the commit. From the repository
root:

    go test . ./highlight ./svgimage -golden.update

Both halves of that line matter. `go test` cannot tell that an unfamiliar
flag is boolean, so a flag placed before the packages swallows them: `go
test -golden.update ./...` tests whatever package the repository root
holds, not `./...`. And `./...` cannot stand in for the list, though this
file told you it could until F5.5 — `internal/lint` stores no goldens, never
links the harness, and a test binary rejects a flag it never declared, so
`go test ./... -golden.update` has always failed there.

**A green CI run does not say these images matched.** The harness answers a
failed `headless.NewWindow` with `t.Skipf`, and a skipped test passes, so
the pixels and the build status are independent facts. Until F5.4 nothing
could tell them apart: the workflow ran plain `go test`, which never prints
a skip, and downloading a run log needs admin rights on the repository. The
test step is verbose now, and the step after it publishes the verdict as a
workflow annotation — and annotations, unlike logs, are public: the
`check-runs` endpoint for the commit returns them with no token at all.
Read it before treating green as a golden-image gate, and expect the answer
to be that they skipped. The runner installs the GL *development* headers,
where gio's own Linux CI also installs the drivers — `libegl-mesa0`,
`libglx-mesa0`, `libgl1-mesa-dri`, `mesa-libgallium`, `libgbm1`, `libegl1`,
`mesa-vulkan-drivers` — without which there is no EGL display to initialise
and no Vulkan ICD for gio's fallback context to find.

**A golden test pins its faces; application code does not.** Every golden and
pixel test here builds its shaper with
`tokens.DefaultTypography.DeterministicShaper()` — the default typography's
faces and nothing else, system fonts off, so the stored PNGs are the same on
every machine. Applications call `Shaper()` instead, which falls back to the
platform's own fonts so that text outside Roboto and Roboto Mono still
resolves. The two are not interchangeable: a golden written against
`Shaper()` passes on the machine that wrote it and fails on one with a
different font set, which is the failure the split constructor exists to make
impossible. When a test genuinely needs a glyph the default faces lack, widen
the collection rather than reach for the system —
`tokens.DefaultTypography.WithFaces(notosansmono.FontFace()).DeterministicShaper()`
— and assert the shaper resolved the rune rather than storing it as pixels.
