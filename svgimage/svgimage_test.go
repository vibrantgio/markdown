package svgimage_test

import (
	"image"
	"testing"
	"testing/fstest"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/svgimage"
	"github.com/vibrantgio/prism/golden"
	"github.com/vibrantgio/spectrum/tokens"
)

// iconSVG is a viewBox-sized disc with an inset square: enough geometry to
// prove paths parse, fill, and scale.
const iconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` +
	`<circle cx="12" cy="12" r="11" fill="#10a37f"/>` +
	`<rect x="8" y="8" width="8" height="8" fill="#ffffff"/>` +
	`</svg>`

func testFS() fstest.MapFS {
	return fstest.MapFS{"icon.svg": &fstest.MapFile{Data: []byte(iconSVG)}}
}

// TestDocumentRendersSVGGolden records or diffs a document whose sole image
// is served by the provider as vector geometry.
func TestDocumentRendersSVGGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	style := markdown.FromTokens(tokens.DefaultLight, tokens.DefaultTypography)
	style.Images = svgimage.New(testFS())
	blocks := markdown.Parse([]byte("before\n\n![vector icon](icon.svg)\n\nafter\n"))
	d := markdown.NewDocument(blocks)

	golden.Render(t, "svg-icon-light", image.Pt(200, 120), func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, tokens.DefaultLight.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return d.Layout(gtx, shaper, style)
		})
	})
}

// TestImageWidgetFallthroughs verifies the provider's contract edges: only
// .svg destinations are served, missing files error, and non-SVG
// destinations delegate to the raster provider when present.
func TestImageWidgetFallthroughs(t *testing.T) {
	p := svgimage.New(testFS())

	if w, err := p.ImageWidget("icon.svg"); err != nil || w == nil {
		t.Errorf("ImageWidget(icon.svg) = (%v, %v); want a widget", w, err)
	}
	if _, err := p.ImageWidget("photo.png"); err == nil {
		t.Error("ImageWidget(photo.png) succeeded; want a not-svg error")
	}
	if _, err := p.ImageWidget("missing.svg"); err == nil {
		t.Error("ImageWidget(missing.svg) succeeded; want an open error")
	}
	if _, err := p.Image("photo.png"); err == nil {
		t.Error("Image without a raster provider succeeded; want an error")
	}

	rp := rasterProvider{img: image.NewNRGBA(image.Rect(0, 0, 4, 4))}
	pr := svgimage.NewWithRaster(testFS(), rp)
	if img, err := pr.Image("photo.png"); err != nil || img == nil {
		t.Errorf("Image with raster provider = (%v, %v); want the delegate's image", img, err)
	}
}

type rasterProvider struct{ img image.Image }

func (r rasterProvider) Image(string) (image.Image, error) { return r.img, nil }
