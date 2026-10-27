package markdown_test

import (
	"image"
	"image/color"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// TestDocumentTakesTheKeyboard gives the keyboard to a document laid out
// with Layout and reads that the document has it. The document is the first
// Tab stop: the first forward move in a fresh window puts the keyboard on it.
// One forward move from it puts the keyboard on the first task box, and Enter
// there activates that task. One backward move gives the keyboard back to the
// document.
func TestDocumentTakesTheKeyboard(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte("- [ ] open task\n- [x] done task\n"))
	open := blocks[0].(*markdown.List).Items[0]

	var clicks []*markdown.ListItem
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	style.OnTaskClick = func(_ layout.Context, item *markdown.ListItem) {
		clicks = append(clicks, item)
	}

	d := markdown.NewDocument(blocks)
	size := image.Pt(400, 80)
	var give, focused bool
	w := func(gtx layout.Context) layout.Dimensions {
		if give {
			gtx.Execute(key.FocusCmd{Tag: d.Focus()})
			give = false
		}
		paint.FillShape(gtx.Ops, tokens.PlatformLight.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
		dims := d.Layout(gtx, shaper, style)
		focused = d.Focused(gtx)
		return dims
	}

	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveTaskFrame(w, ops, r, size)
	if focused {
		t.Fatal("the document has the keyboard before anything gave it the keyboard")
	}

	give = true
	driveTaskFrame(w, ops, r, size)
	driveTaskFrame(w, ops, r, size)
	if !focused {
		t.Fatal("a FocusCmd to the document's tag did not give the document the keyboard")
	}

	r.MoveFocus(key.FocusForward)
	driveTaskFrame(w, ops, r, size)
	if focused {
		t.Fatal("one forward move from the document left the keyboard on the document")
	}
	r.Queue(
		key.Event{Name: key.NameReturn, State: key.Press},
		key.Event{Name: key.NameReturn, State: key.Release},
	)
	driveTaskFrame(w, ops, r, size)
	if len(clicks) != 1 || clicks[0] != open {
		t.Fatalf("Enter after one forward move from the document: clicks = %v, want [%p open]", clicks, open)
	}

	r.MoveFocus(key.FocusBackward)
	driveTaskFrame(w, ops, r, size)
	if !focused {
		t.Error("one backward move from the first task box did not give the keyboard back to the document")
	}

	r = new(gioinput.Router)
	driveTaskFrame(w, ops, r, size)
	r.MoveFocus(key.FocusForward)
	driveTaskFrame(w, ops, r, size)
	if !focused {
		t.Error("the first forward move in a fresh window did not put the keyboard on the document; the document is not the first Tab stop")
	}
}

// TestFocusedDocumentPixelsUnchanged holds that the keyboard on the document
// draws nothing: the focused document and the unfocused one are the same
// pixels.
func TestFocusedDocumentPixelsUnchanged(t *testing.T) {
	shaper := defaultShaper(t)
	blocks := markdown.Parse([]byte("# Heading\n\nA paragraph with a [link](https://example.com).\n\n- [ ] open\n- [x] done\n"))
	style := markdown.FromTokens(tokens.PlatformLight, tokens.DefaultTypography, color.NRGBA{})
	size := image.Pt(400, 160)

	frame := func(give bool) *image.RGBA {
		d := markdown.NewDocument(blocks)
		w := func(gtx layout.Context) layout.Dimensions {
			if give {
				gtx.Execute(key.FocusCmd{Tag: d.Focus()})
			}
			paint.FillShape(gtx.Ops, tokens.PlatformLight.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return d.Layout(gtx, shaper, style)
		}
		r := new(gioinput.Router)
		ops := new(op.Ops)
		driveTaskFrame(w, ops, r, size)
		driveTaskFrame(w, ops, r, size)
		var held bool
		img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
			gtx.Source = r.Source()
			dims := w(gtx)
			held = d.Focused(gtx)
			return dims
		})
		if held != give {
			t.Fatalf("the captured document has the keyboard = %v, want %v", held, give)
		}
		return img
	}
	idle := frame(false)
	focused := frame(true)
	if n := golden.PixelDiff(idle, focused); n != 0 {
		t.Errorf("the keyboard on the document moved %d pixels; a focused document draws nothing", n)
	}
}
