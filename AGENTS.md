# AGENTS.md — markdown

Document-grade Markdown rendering for Gio: a goldmark AST with GFM walked
into a block model and laid out with components primitives — headings on
the token type scale, richtext paragraphs, nested lists with task boxes,
blockquotes, rules, fenced code with tab expansion and optional chroma
highlighting, GFM tables, and images through a caller-supplied provider
that does the I/O. The goldmark dependency stops here and the chroma
dependency stops in `highlight`, so components never sees either.

**Layer.** Tier 4 of ADR-001's stack, `mvu → theme → components → pulse →
cadence → markdown`, alongside cadence. Its root module imports
`components`, `svg`, `svg/driver/gio` and `theme`, and reaches `font`
through them. No other repository's root module imports it; outside the
tier table it is imported by the workbench applications `mindchat` and
`sitedocs`. Both directions are measured rather than typed —
`scripts/check-layers.sh --edges` reports the graph and
`scripts/sync-agents.sh` renders these sentences from it — so correcting
them here changes nothing.

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
`github.com/vibrantgio/components/golden`, which declares `-golden.update`
and is shared with `cadence`, `pulse` and `workbench`. Do not inline a copy
of it, and do not declare a second `-golden.update`: two registrations of
one flag name in a single test binary panic in `flag.Bool` at init, before
any test runs. When a change legitimately moves pixels, regenerate them
within the same change, look at what came out, and say so in the commit.
From the repository root:

    go test ./highlight ./svgimage . -golden.update

Both halves of that line matter. `go test` cannot tell that an unfamiliar
flag is boolean, so a flag placed before the packages swallows them: `go
test -golden.update ./...` tests whatever package the repository root
holds, not `./...`. And `./...` cannot stand in for the list — this module
has test packages that store no goldens, and a test binary rejects a flag
it never declared.

**A green CI run does not say these images matched. They are compared only
on a developer's machine, and that is deliberate.** The harness answers a
failed `headless.NewWindow` with `t.Skipf`, a skipped test passes, and the
runner has no GL driver for it to open — so the pixels and the build status
are independent facts. The `build` job's *Were the golden images compared,
or skipped?* step, added by F5.4, publishes which of the two happened as a
workflow annotation, readable without a token at `GET
/repos/vibrantgio/markdown/commits/<sha>/check-runs`; it has answered
SKIPPED on every run. F5.7 then measured the alternative rather than
leaving it as an open question. Adding the drivers gio's own Linux CI
installs — `libegl1`, `libegl-mesa0`, `libglx-mesa0`, `libgl1-mesa-dri`,
`mesa-libgallium`, `libgbm1`, `mesa-vulkan-drivers` — does work: on pulse
the verdict flipped to COMPARED on the next run. Nine of that repository's
twenty-one images then failed, 12782 pixels apart, while the three drawn on
the CPU still matched exactly. Every golden in the organization was
recorded on macOS, so the gate would not be asserting that the images are
right, only that Linux mesa and Metal rasterise identically — which they do
not, and need not. **So CI gates the build and the tests, never the
pixels**, and moving an image is checked where it is regenerated.

**A golden test pins its faces; application code does not.** Every golden
and pixel test here builds its shaper with
`tokens.DefaultTypography.DeterministicShaper()` — the default typography's
faces and nothing else, system fonts off, so the stored PNGs are the same
on every machine. Applications call `Shaper()` instead, which falls back to
the platform's own fonts so that text outside Roboto and Roboto Mono still
resolves. The two are not interchangeable: a golden written against
`Shaper()` passes on the machine that wrote it and fails on one with a
different font set, which is the failure the split constructor exists to
make impossible.

When a test genuinely needs a glyph the default faces lack, widen the
collection rather than reach for the system:

    tokens.DefaultTypography.WithFaces(notosansmono.FontFace()).DeterministicShaper()

Then assert that the shaper resolved the rune, rather than storing the
result as pixels. A stored image proves the glyph came out somewhere; only
the assertion says which face drew it.
