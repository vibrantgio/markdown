package markdown_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// TestAMarkInALaterListItemStandsWhereTheItemDoes: every layout path that
// offsets what it drew moves the marks recorded inside it by the same
// amount; a list item below the first is offset by the items above it, and
// its mark must stand below theirs, never at the first item's height.
func TestAMarkInALaterListItemStandsWhereTheItemDoes(t *testing.T) {
	src := "- first needle\n- second needle\n- third needle\n"
	d := markdown.NewDocument(markdown.Parse([]byte(src)))
	shaper := defaultShaper(t)
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	d.Find("needle", 0)
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300))}
	d.Layout(gtx, shaper, style)
	ms := d.Matches()
	if len(ms) != 3 {
		t.Fatalf("found %d matches, want 3", len(ms))
	}
	// Each item stands below the one before it, so each mark does too; a
	// mark reported at its item's own origin would stand on the first's.
	for i := 1; i < 3; i++ {
		if ms[i].Rect.Min.Y <= ms[i-1].Rect.Min.Y {
			t.Errorf("item %d's mark stands at y %d, not below item %d's at y %d", i+1, ms[i].Rect.Min.Y, i, ms[i-1].Rect.Min.Y)
		}
	}
}
