package markdown_test

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// wideTable is a table whose cells are far too wide for the width it is laid
// out at: fifteen body rows of two-thousand-character prose, which is the
// shape a document's milestone table has and the shape whose shaping cost
// three passes a frame before the cells were recorded.
func wideTable(rows, chars int) []byte {
	words := []string{
		"geometry", "constraint", "extrusion", "revolve", "fillet", "chamfer",
		"sketch", "solver", "assembly", "component", "tolerance", "surface",
		"boolean", "timeline", "parameter", "reference", "projection",
	}
	var cell strings.Builder
	for i := 0; cell.Len() < chars; i++ {
		if i > 0 {
			cell.WriteByte(' ')
		}
		cell.WriteString(words[i%len(words)])
	}
	text := cell.String()

	var b strings.Builder
	b.WriteString("| # | Milestone | Status | Notes |\n|---|---|---|---|\n")
	for r := range rows {
		b.WriteString("| M")
		b.WriteString(string(rune('0' + r%10)))
		b.WriteString(" | ")
		b.WriteString(text)
		b.WriteString(" | in progress | ")
		b.WriteString(text)
		b.WriteString(" |\n")
	}
	return []byte(b.String())
}

// frames lays d out n times at size and returns how long each frame took.
func frames(d *markdown.Document, shaper *text.Shaper, style markdown.Style, size image.Point, n int) []time.Duration {
	out := make([]time.Duration, 0, n)
	for range n {
		var ops op.Ops
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Ops:         &ops,
		}
		start := time.Now()
		d.Layout(gtx, shaper, style)
		out = append(out, time.Since(start))
	}
	return out
}

// TestWideTableSteadyFrameCost asserts the table block measures its columns
// and shapes its cells once rather than on every frame: the first frame over
// a fifteen-row table of two-thousand-character cells may measure, and every
// frame after it costs the recorded cells' replay alone.
//
// The budget asserted is twenty milliseconds, four times the five a steady
// frame is meant to stay inside, because a test machine under load is not a
// frame budget; the measured number is logged so a regression that is real
// but inside the margin is still visible. Before the records existed the
// frames this test lays out cost upwards of two hundred milliseconds each,
// which no margin covers.
func TestWideTableSteadyFrameCost(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	blocks := markdown.Parse(wideTable(15, 2000))
	d := markdown.NewDocument(blocks)

	took := frames(d, shaper, style, image.Pt(700, 900), 6)
	t.Logf("frames: %v", took)
	const budget = 20 * time.Millisecond
	for i, f := range took[1:] {
		if f > budget {
			t.Errorf("frame %d took %v, over the %v budget; the table is being re-measured or re-shaped every frame", i+1, f, budget)
		}
	}
}

// TestTableReplayDrawsTheSamePixels verifies that replaying a recorded cell
// paints what laying it out painted: three frames of the same document are
// pixel-identical, and so is a document laid out for the first time.
func TestTableReplayDrawsTheSamePixels(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	blocks := markdown.Parse(wideTable(4, 300))
	size := image.Pt(360, 520)

	d := markdown.NewDocument(blocks)
	w := themed(d, shaper, style, tokens.PlatformLight)
	first := golden.Capture(t, size, w)
	second := golden.Capture(t, size, w)
	third := golden.Capture(t, size, w)
	if n := golden.PixelDiff(first, second); n != 0 {
		t.Errorf("%d pixels differ between the recording frame and the first replay", n)
	}
	if n := golden.PixelDiff(second, third); n != 0 {
		t.Errorf("%d pixels differ between two replayed frames", n)
	}
	fresh := golden.Capture(t, size, themed(markdown.NewDocument(markdown.Parse(wideTable(4, 300))), shaper, style, tokens.PlatformLight))
	if n := golden.PixelDiff(first, fresh); n != 0 {
		t.Errorf("%d pixels differ between a reused document and a fresh one", n)
	}
}

// TestTableRemeasuresOnWidthAndStyle verifies the records are keyed on what
// they were drawn for: a document laid out at another width, or in another
// theme, and then brought back draws exactly what a document seeing that
// width or that theme for the first time draws.
func TestTableRemeasuresOnWidthAndStyle(t *testing.T) {
	shaper := defaultShaper(t)
	light := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	dark := markdown.FromTokens(tokens.PlatformDark, tokens.DefaultTypography, color.NRGBA{})
	src := wideTable(4, 300)
	narrow, wide := image.Pt(300, 520), image.Pt(560, 520)

	d := markdown.NewDocument(markdown.Parse(src))
	golden.Capture(t, narrow, themed(d, shaper, light, tokens.PlatformLight))
	widened := golden.Capture(t, wide, themed(d, shaper, light, tokens.PlatformLight))
	freshWide := golden.Capture(t, wide, themed(markdown.NewDocument(markdown.Parse(src)), shaper, light, tokens.PlatformLight))
	if n := golden.PixelDiff(widened, freshWide); n != 0 {
		t.Errorf("%d pixels differ after a width change; the columns were not re-measured", n)
	}

	d = markdown.NewDocument(markdown.Parse(src))
	golden.Capture(t, narrow, themed(d, shaper, light, tokens.PlatformLight))
	switched := golden.Capture(t, narrow, themed(d, shaper, dark, tokens.PlatformDark))
	freshDark := golden.Capture(t, narrow, themed(markdown.NewDocument(markdown.Parse(src)), shaper, dark, tokens.PlatformDark))
	if n := golden.PixelDiff(switched, freshDark); n != 0 {
		t.Errorf("%d pixels differ after a theme change; the cells were not re-drawn", n)
	}
}

// TestTableMarksSurviveTheRecord verifies a find's marks on a table cell are
// still drawn and still reported on a frame the cell is replayed from its
// record, and that stepping the current match re-draws the cell it moved to.
func TestTableMarksSurviveTheRecord(t *testing.T) {
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	src := []byte("| Package | Role |\n|---|---|\n| prism | primitives |\n| cadence | patterns |\n")
	blocks := markdown.Parse(src)
	size := image.Pt(360, 200)

	d := markdown.NewDocument(blocks)
	d.Find("p", 0)
	w := themed(d, shaper, style, tokens.PlatformLight)
	marked := golden.Capture(t, size, w)
	replayed := golden.Capture(t, size, w)
	if n := golden.PixelDiff(marked, replayed); n != 0 {
		t.Errorf("%d pixels differ between a marked frame and its replay", n)
	}

	first := d.Matches()
	_ = golden.Capture(t, size, w)
	again := d.Matches()
	if len(first) == 0 {
		t.Fatalf("no matches found for %q", "p")
	}
	for i := range first {
		if first[i] != again[i] {
			t.Errorf("match %d reported %v then %v; a replayed cell must report its marks where it painted them", i, first[i], again[i])
		}
	}

	d.Find("p", 1)
	stepped := golden.Capture(t, size, w)
	fresh := markdown.NewDocument(markdown.Parse(src))
	fresh.Find("p", 1)
	freshImg := golden.Capture(t, size, themed(fresh, shaper, style, tokens.PlatformLight))
	if n := golden.PixelDiff(stepped, freshImg); n != 0 {
		t.Errorf("%d pixels differ after stepping the current match; the record was not keyed on it", n)
	}
}
