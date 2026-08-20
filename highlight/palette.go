// palette.go — a base read as a list of colours rather than as a style.
//
// A syntax style is a curated palette that somebody spent time on: a couple
// of dozen inks chosen to sit together, already discrete, already deliberate.
// That makes it a better source for a colour than a photograph is, and the
// only thing standing between the two is a type — one side of this module
// speaks chroma and nothing outside this package is allowed to. So the base
// comes out of here as plain colours, and what reads them is free of every
// dependency this file has.

package highlight

import (
	stdcolor "image/color"
	"slices"

	"github.com/alecthomas/chroma/v2"
)

// BasePalette returns the colours the named base draws code with, or nil for a
// name that resolves to no style. It is what a caller hands to a seed
// extractor or paints as swatches, and it names no chroma type, so reading a
// style this way costs nothing but this package.
//
// The list is the base's inks and not its whole entry table. An entry with no
// colour of its own contributes nothing, and so does one resolving to the
// style's plain foreground: that is the colour ordinary code is drawn in —
// the one a highlighter built here deliberately does not emit, so the theme's
// own text colour shows through — and counting it would let the least
// deliberate colour in a style outweigh every chosen one.
//
// Colours repeat, and the repeats are the point. A style that draws eight
// kinds of name in one blue and one number in one orange puts that blue in
// the list eight times, so a reader ranking the list by prominence sees how
// much of the style each colour actually is. Callers wanting the distinct set
// can reduce it themselves.
//
// The order is the token type order, which is stable across runs and across
// machines: a chroma style holds its entries in a map, and this walks a sorted
// copy of the keys rather than the map.
func BasePalette(name string) []stdcolor.NRGBA {
	s, ok := lookup(name)
	if !ok {
		return nil
	}
	return palette(s)
}

// palette is [BasePalette] on a style already resolved.
func palette(s *chroma.Style) []stdcolor.NRGBA {
	plain := plainForeground(s)
	types := s.Types()
	slices.Sort(types)
	out := make([]stdcolor.NRGBA, 0, len(types))
	for _, tt := range types {
		e := s.Get(tt)
		if !e.Colour.IsSet() || e.Colour == plain {
			continue
		}
		out = append(out, fromChroma(e.Colour))
	}
	return out
}
