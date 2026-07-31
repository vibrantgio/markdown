// Package svgimage serves a document's ![](...) images from SVG sources,
// rendered as crisp vector geometry through vibrantgio/svg's Gio driver.
// It implements the core package's [markdown.WidgetImageProvider] hook and
// carries the svg dependency the same way markdown/highlight carries
// chroma: consumers that want SVG images import this package, everyone
// else stays free of it.
//
// A destination is recognised by its literal .svg extension — the match is
// case-insensitive, but anything trailing defeats it: icon.svg is served while
// icon.svg?v=2 and icon.svg#frag are not SVG destinations at all. Everything
// unrecognised goes to the raster provider given to [NewWithRaster], and a
// [New] provider has none, so a document mixing PNGs with SVGs needs the
// two-argument constructor.
//
// Every failure — wrong extension, missing file, unparseable SVG — is returned
// as an error and rendered by the document as the image's alt text. Note that
// the document caches that outcome per block, failures included, so a file
// that only appears after the first frame keeps rendering alt text until a new
// markdown.Document is built.
package svgimage

import (
	"fmt"
	"image"
	"io/fs"
	"path"
	"strings"

	"gioui.org/layout"

	giodriver "github.com/vibrantgio/svg/driver/gio"
	"github.com/vibrantgio/svg/parser"

	"github.com/vibrantgio/markdown"
)

// Provider serves destinations ending in .svg from a file system —
// typically a go:embed asset tree, so the document performs no network
// I/O — as vector widgets sized by the SVG's own viewBox and scaled down
// to the constraint. Other destinations delegate to the optional raster
// provider, and every failure falls through to the document's alt-text
// rendering.
type Provider struct {
	fsys   fs.FS
	raster markdown.ImageProvider
}

// New returns a Provider serving .svg destinations from fsys.
func New(fsys fs.FS) *Provider {
	return &Provider{fsys: fsys}
}

// NewWithRaster returns a Provider serving .svg destinations from fsys and
// delegating every other destination to raster.
func NewWithRaster(fsys fs.FS, raster markdown.ImageProvider) *Provider {
	return &Provider{fsys: fsys, raster: raster}
}

// ImageWidget parses the SVG at the destination path in the provider's
// file system and returns a widget rendering it at its viewBox size,
// constrained down with its aspect preserved.
func (p *Provider) ImageWidget(url string) (layout.Widget, error) {
	if !strings.EqualFold(path.Ext(url), ".svg") {
		return nil, fmt.Errorf("svgimage: %q is not an .svg destination", url)
	}
	f, err := p.fsys.Open(url)
	if err != nil {
		return nil, fmt.Errorf("svgimage: %w", err)
	}
	defer f.Close()
	icon, err := parser.NewParser(parser.IgnoreErrorMode).ParseStream(f)
	if err != nil {
		return nil, fmt.Errorf("svgimage: parse %q: %w", url, err)
	}
	return giodriver.IconWidget(icon, 0, 0, 1), nil
}

// Image delegates non-SVG destinations to the raster provider, when set.
func (p *Provider) Image(url string) (image.Image, error) {
	if p.raster == nil {
		return nil, fmt.Errorf("svgimage: no raster provider for %q", url)
	}
	return p.raster.Image(url)
}
