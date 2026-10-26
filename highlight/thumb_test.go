package highlight

import (
	"image"
	stdcolor "image/color"
	"testing"
	"time"

	"gioui.org/layout"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// wideFence is a fence whose one line runs past the column, so the block
// scrolls and draws its bar.
const wideFence = "```go\n" +
	"func main() { println(\"a line far too long for the column it is set in, and then some more of it\") }\n" +
	"```\n"

// TestTheThumbOverAStyledBlockIsAStepFromItsBackground draws a wide fence in
// the default highlighter style with the standing bar. The thumb is the
// platform's knob flattened onto the background the block is drawn in, so
// it is one step from that background. The thumb the theme's own code fill
// gives is a neutral grey, and the drawn block holds none of it. A bar set
// before the style is applied and a bar set after it are the same bar.
func TestTheThumbOverAStyledBlockIsAStepFromItsBackground(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	size := image.Pt(420, 120)
	for _, sc := range schemes() {
		t.Run(sc.name, func(t *testing.T) {
			plain := markdown.FromTokens(sc.tok, tokens.DefaultTypography, stdcolor.NRGBA{})
			st := plain
			st.CodeScrollbar = st.CodeScrollbar.Always()
			Apply(&st, DefaultStyle, sc.tok)

			after := plain
			Apply(&after, DefaultStyle, sc.tok)
			after.CodeScrollbar = after.CodeScrollbar.Always()
			if st.CodeScrollbar.ThumbColor != after.CodeScrollbar.ThumbColor || st.CodeScrollbar.FadeDelay != 0 {
				t.Errorf("the bar set before the style is %+v; set after it, %+v", st.CodeScrollbar, after.CodeScrollbar)
			}

			want := color.Flatten(sc.tok.ScrollbarThumb, st.CodeBackground)
			if st.CodeScrollbar.ThumbColor != want {
				t.Fatalf("the thumb is %v; want the knob flattened onto the block's background %v, %v", st.CodeScrollbar.ThumbColor, st.CodeBackground, want)
			}
			if want == plain.CodeScrollbar.ThumbColor {
				t.Fatal("the styled block's thumb equals the theme's, so this proves nothing")
			}

			doc := markdown.NewDocument(markdown.Parse([]byte(wideFence)))
			img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
				gtx.Now = time.Unix(1700000000, 0)
				return doc.Layout(gtx, shaper, st)
			})
			thumb := stdcolor.RGBAModel.Convert(want).(stdcolor.RGBA)
			grey := stdcolor.RGBAModel.Convert(plain.CodeScrollbar.ThumbColor).(stdcolor.RGBA)
			var drawn, neutral int
			for y := range size.Y {
				for x := range size.X {
					switch img.RGBAAt(x, y) {
					case thumb:
						drawn++
					case grey:
						neutral++
					}
				}
			}
			if drawn == 0 {
				t.Error("the styled block drew no thumb in the knob flattened onto its background")
			}
			if neutral != 0 {
				t.Errorf("the styled block drew %d pixels in the thumb the theme's code fill gives", neutral)
			}
		})
	}
}

// TestAZeroBarStaysZero: a Style that turned the code block's bar off keeps
// it off when a highlighter style is applied.
func TestAZeroBarStaysZero(t *testing.T) {
	st := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, stdcolor.NRGBA{})
	st.CodeScrollbar = scrollbar.Style{}
	Apply(&st, DefaultStyle, tokens.PlatformLight)
	if st.CodeScrollbar.Width() != 0 || st.CodeScrollbar.ThumbColor != (stdcolor.NRGBA{}) {
		t.Errorf("an applied style gave the turned-off bar %+v", st.CodeScrollbar)
	}
}
