// bases.go — which names a style can be derived from, including the ones a
// person adds themselves.
//
// Chroma ships its styles embedded, and a style is also a small XML document
// that a person can write or download and drop in a folder. Both are offered
// here under one vocabulary — a base is a name — so a chooser lists them
// together and a name that was kept resolves the same way whichever kind it
// turned out to be.
//
// Loaded styles are held HERE and not put into chroma's registry. The registry
// is a curated set that this package promises not to touch (see adapt.go), and
// a file appearing in it would make the promise conditional on what happens to
// be in somebody's folder. The lookups below read this map first and the
// registry after, so a loaded style is reachable everywhere a name is, and
// nothing outside this package can tell that the two came from different
// places.

package highlight

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

// styleExt is the extension a style file is recognised by. Chroma's own
// converter writes this, and the folder is a place a person drops things —
// scanning everything in it and complaining about the ones that are not
// styles would make a README beside them an error.
const styleExt = ".xml"

// Skipped is a file the folder held that did not become a base, and why. It
// is returned rather than logged because the answer belongs on screen where
// the styles were expected: a file silently ignored is indistinguishable from
// one that was never noticed.
type Skipped struct {
	// File is the file's own name, without the folder it sat in — what a
	// person will look for when they go and fix it.
	File string
	// Reason says what went wrong, in one sentence and without a stack.
	Reason string
}

// String is the sentence to show: the file and what was wrong with it.
func (s Skipped) String() string { return s.File + ": " + s.Reason }

// loaded holds the styles read from a folder, keyed by lower-cased name the
// way chroma keys its own registry. The mutex is here because the folder is
// read once at startup while nothing else is running and then read from every
// derivation afterwards, and "once at startup" is a convention rather than a
// guarantee.
var (
	loadedMu sync.RWMutex
	loaded   = map[string]*chroma.Style{}
)

// LoadDir reads every .xml file in dir as a style and makes it choosable by
// name, beside the embedded ones. It returns the names it loaded and the
// files it did not, each with its reason.
//
// Nothing here is fatal. A folder that is not there is not an error — it is a
// person who has not added any styles — and comes back empty and unskipped. A
// file that will not parse, or one naming a style the embedded set already
// has, is skipped and named: one bad file must not cost the rest of the
// folder, and a style silently shadowing a curated one would be worse than
// not loading it.
//
// Loading again reloads: a name loaded by an earlier call is replaced by a
// later one, so re-reading the folder after somebody edits it does what it
// looks like. Two files in one folder claiming the same name are a collision
// and the second is skipped, because there is no ordering between them a
// person would predict.
func LoadDir(dir string) (names []string, skipped []Skipped) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, []Skipped{{File: filepath.Base(dir), Reason: err.Error()}}
	}
	seen := map[string]string{} // lower-cased style name -> the file that claimed it
	found := map[string]*chroma.Style{}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), styleExt) {
			continue
		}
		style, err := readStyle(filepath.Join(dir, e.Name()))
		if err != nil {
			skipped = append(skipped, Skipped{File: e.Name(), Reason: err.Error()})
			continue
		}
		key := strings.ToLower(style.Name)
		if _, embedded := styles.Registry[key]; embedded {
			skipped = append(skipped, Skipped{File: e.Name(),
				Reason: fmt.Sprintf("%q is the name of a style that ships embedded", style.Name)})
			continue
		}
		if first, dup := seen[key]; dup {
			skipped = append(skipped, Skipped{File: e.Name(),
				Reason: fmt.Sprintf("%q is already the name of the style in %s", style.Name, first)})
			continue
		}
		seen[key], found[key] = e.Name(), style
		names = append(names, style.Name)
	}
	loadedMu.Lock()
	for k, v := range found {
		loaded[k] = v
	}
	loadedMu.Unlock()
	slices.Sort(names)
	return names, skipped
}

// readStyle parses one file as a chroma style. The file is closed before the
// caller sees either result.
func readStyle(path string) (*chroma.Style, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	style, err := chroma.NewXMLStyle(f)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(style.Name) == "" {
		return nil, fmt.Errorf("the style carries no name")
	}
	return style, nil
}

// Bases returns every name a base can be chosen by — the embedded styles and
// everything [LoadDir] has loaded — sorted, so a chooser built from it stands
// still between runs.
func Bases() []string {
	out := styles.Names()
	loadedMu.RLock()
	for _, s := range loaded {
		out = append(out, s.Name)
	}
	loadedMu.RUnlock()
	slices.Sort(out)
	return out
}

// Known reports whether name resolves to a base. It is what a caller holding
// a name from somewhere else — a settings file written by an older build, a
// folder that has since lost a file — asks before deriving from it.
func Known(name string) bool {
	_, ok := lookup(name)
	return ok
}

// Loaded reports whether name came from a file rather than from the embedded
// set. A chooser can use it to say where a style came from; nothing else
// about the name behaves differently.
func Loaded(name string) bool {
	loadedMu.RLock()
	defer loadedMu.RUnlock()
	_, ok := loaded[strings.ToLower(name)]
	return ok
}

// BaseSuits reports whether the base named is one to offer for a dark
// appearance, or for a light one — the question a chooser that shows one half
// of the list at a time has to ask of every name in it.
//
// The answer is measured, off the style's own Background entry: the ground its
// author fitted its inks against, read on the same perceptual lightness axis
// the derivation uses to decide which way an ink has to move. It is measured
// and not read off the name because a name is not evidence: most of the
// embedded set says nothing about its appearance at all, and a name that does
// say something is under no obligation to be true — a styles folder is a place
// people put files they wrote themselves. A chooser that guessed would put a
// style under the wrong half with nothing to appeal to.
//
// A style that names no ground at all suits both. It was fitted to nothing, so
// whatever it is drawn on is the theme's own surface and there is no appearance
// it is the wrong choice for; putting it under one half would take it out of
// reach under the other for no measured reason. Four of the embedded styles are
// like this.
//
// A name that resolves to nothing suits neither: there is no style to measure,
// and none to offer.
func BaseSuits(name string, dark bool) bool {
	s, ok := lookup(name)
	if !ok {
		return false
	}
	bg := s.Get(chroma.Background).Background
	if !bg.IsSet() {
		return true
	}
	return isDarkSurface(fromChroma(bg)) == dark
}

// BaseOrDefault returns name when it resolves and [DefaultBase] when it does
// not, which is the whole of what a kept preference needs: a name nobody
// chose, or one whose file is no longer in the folder, leaves the code
// coloured the way it is coloured for somebody who never chose at all.
func BaseOrDefault(name string) string {
	if Known(name) {
		return name
	}
	return DefaultBase
}

// BasePair is a base per appearance: the palette code is coloured from under a
// light one, and the palette it is coloured from under a dark one. It is what
// a person has chosen when they have chosen twice, and what [AdaptPair]
// derives through.
type BasePair struct {
	Light string
	Dark  string
}

// DefaultBases is the pair to derive through when nothing was chosen:
// [DefaultBase] under a light appearance and [DefaultDarkBase] under a dark
// one.
func DefaultBases() BasePair { return BasePair{Light: DefaultBase, Dark: DefaultDarkBase} }

// Base returns the member for one appearance.
func (p BasePair) Base(dark bool) string {
	if dark {
		return p.Dark
	}
	return p.Light
}

// BasesOrDefault resolves a pair that was kept into a pair that can be drawn:
// each member stands when it names a style this build has AND that style was
// fitted to the appearance it is being kept for, and falls back to the default
// for that appearance otherwise.
//
// Both halves of that are the same rule — a member has to be usable where it
// is going. A name nobody chose, or one whose file has left the folder, is the
// ordinary case and falls back the way [BaseOrDefault] does. A name fitted to
// the other ground is the odder one: it can only arrive from a file naming one
// base with no appearance attached, or from a file somebody edited by hand,
// and it is measured off the style's own background by [BaseSuits] rather than
// guessed from the name. Sending it through anyway would put a palette
// balanced for paper on a near-black slab, and would leave a chooser marking a
// row that its own list does not hold.
//
// So one kept name resolves by measurement: passed as both members, it keeps
// the appearance it was fitted to and the other takes the default. A style
// fitted to no ground at all suits both and keeps both.
func BasesOrDefault(light, dark string) BasePair {
	return BasePair{Light: baseFor(light, false), Dark: baseFor(dark, true)}
}

// baseFor is one member of [BasesOrDefault].
func baseFor(name string, dark bool) string {
	if Known(name) && BaseSuits(name, dark) {
		return name
	}
	if dark {
		return DefaultDarkBase
	}
	return DefaultBase
}

// lookup resolves a name to the style it names, loaded styles first and
// chroma's registry after. Unknown names return false rather than chroma's
// fallback style: this package's callers panic on a name that is not there,
// and silently colouring with a style nobody asked for is what that is for.
func lookup(name string) (*chroma.Style, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	loadedMu.RLock()
	s, ok := loaded[key]
	loadedMu.RUnlock()
	if ok {
		return s, true
	}
	s, ok = styles.Registry[key]
	return s, ok
}

// forMode resolves the member of name's pair that draws in mode, by the rule
// chroma's own registry uses: the style itself when it already draws that way,
// its counterpart when it names one that does, and the style itself when
// there is no better answer. A style with no counterpart is therefore derived
// from for both appearances, which is what an unpaired base means.
func forMode(name string, mode chroma.Mode) (*chroma.Style, bool) {
	s, ok := lookup(name)
	if !ok {
		return nil, false
	}
	if s.Mode() == mode || s.Counterpart == "" {
		return s, true
	}
	if cp, ok := lookup(s.Counterpart); ok && cp.Mode() == mode {
		return cp, true
	}
	return s, true
}
