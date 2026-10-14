package editor

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// input takes the editor's input as it comes; it reports whether it took
// the event.
func (e *Editor) input(ev ui.InputEvent) bool {
	if e.opts.ReadOnly && (ev.Kind == ui.InputText || ev.Kind == ui.InputCompose) {
		return true
	}
	switch ev.Kind {
	case ui.InputText:
		s := strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f {
				return -1 // Enter, Tab and Backspace come as keys
			}
			return r
		}, ev.Text)
		e.preedit = ""
		if s != "" {
			e.Insert(s)
		}
		return true
	case ui.InputCompose:
		if ev.Text != "" && e.caret != e.anchor {
			a, b := e.Selection()
			e.replaceRange(a, b, "", false)
		}
		e.preedit, e.preeditCaret = singleLine(ev.Text), ev.Caret
		if ev.Text != "" {
			// Composing hides what is offered; the empty composition an
			// input method sends once it committed the text keeps the
			// names that text offers.
			e.ghost, e.compOpen = "", false
		}
		e.reveal = true
		return true
	case ui.InputCommand:
		return e.command(ev.Text)
	case ui.InputKeyDown:
		return e.key(ev.Mods, ev.Key)
	case ui.InputPointerDown:
		return e.pointerDown(ev)
	case ui.InputPointerMove:
		if e.dragging && ev.Button == 0 {
			e.caret = e.hit(ev.X, ev.Y)
			e.reveal = true
			return true
		}
		return false
	case ui.InputPointerUp:
		was := e.dragging
		e.dragging = false
		return was
	case ui.InputScroll:
		if ev.Mods&^ui.Shift == ui.Cmd && e.opts.Zoom != nil {
			e.zoomBy(ev.DY, ev.Precise)
			return true
		}
		e.scrollBy(ev.DY)
		return true
	}
	return false
}

func (e *Editor) command(name string) bool {
	if e.opts.ReadOnly && name != "copy" && name != "selectAll" {
		return false
	}
	switch name {
	case "copy":
		e.Copy()
	case "cut":
		e.Cut()
	case "paste":
		e.Paste()
	case "selectAll":
		e.SelectAll()
	case "undo":
		e.Undo()
	case "redo":
		e.Redo()
	case "delete":
		e.DeleteForward()
	default:
		return false
	}
	return true
}

// key handles a key pressed.
func (e *Editor) key(mods ui.Modifiers, k ui.Key) bool {
	if e.preedit != "" {
		return false // the input method has the keys while it composes
	}
	if e.opts.OwnMenu != nil && (k == ui.KeyContextMenu && mods == 0 || k == ui.KeyF10 && mods == ui.Shift) {
		e.menuKey = true
		return true
	}
	shift := mods&ui.Shift != 0
	base := mods &^ ui.Shift
	if e.opts.ReadOnly {
		if base == ui.Cmd {
			if k != ui.KeyA && k != ui.KeyC && k != ui.KeyHome && k != ui.KeyEnd && k != ui.KeyLeft && k != ui.KeyRight {
				return false
			}
		} else {
			switch k {
			case ui.KeyLeft, ui.KeyRight, ui.KeyUp, ui.KeyDown, ui.KeyHome, ui.KeyEnd, ui.KeyPageUp, ui.KeyPageDown:
			default:
				return false
			}
		}
	}
	if e.compOpen && mods == 0 {
		switch k {
		case ui.KeyUp:
			e.compSel = (e.compSel + len(e.comp) - 1) % len(e.comp)
			return true
		case ui.KeyDown:
			e.compSel = (e.compSel + 1) % len(e.comp)
			return true
		case ui.KeyEnter:
			return e.AcceptCompletion()
		case ui.KeyEscape:
			e.compOpen = false
			return true
		}
	}
	if base == ui.Cmd {
		switch k {
		case ui.KeyZ:
			if shift {
				e.Redo()
			} else {
				e.Undo()
			}
			return true
		case ui.KeyY:
			e.Redo()
			return true
		case ui.KeyJ:
			e.RequestPrediction()
			return e.opts.Predictor != nil && !e.opts.NoAI
		case ui.KeyA:
			e.SelectAll()
			return true
		case ui.KeyC:
			e.Copy()
			return true
		case ui.KeyX:
			e.Cut()
			return true
		case ui.KeyV:
			e.Paste()
			return true
		case ui.KeyHome:
			e.MoveTo(0, shift)
			return true
		case ui.KeyEnd:
			e.MoveTo(e.Doc.Len(), shift)
			return true
		case ui.KeyLeft:
			e.MoveTo(e.wordLeft(), shift)
			return true
		case ui.KeyRight:
			e.MoveTo(e.wordRight(), shift)
			return true
		case ui.KeyBackspace:
			e.deleteWord(false)
			return true
		case ui.KeyDelete:
			e.deleteWord(true)
			return true
		}
		return false
	}
	if base != 0 {
		return false
	}
	switch k {
	case ui.KeyTab:
		if shift {
			return false
		}
		if !e.AcceptGhost() && !e.opts.Prose {
			e.Insert("\u3000\u3000") // a paragraph's indent
		}
	case ui.KeyEscape:
		if e.Ghost() == "" && !e.predicting {
			return false
		}
		e.DismissGhost()
	case ui.KeyEnter:
		a, b := e.Selection()
		e.replaceRange(a, b, "\n", false)
		e.compOpen = false
	case ui.KeyBackspace:
		e.Backspace()
	case ui.KeyDelete:
		e.DeleteForward()
	case ui.KeyLeft, ui.KeyRight:
		a, b := e.Selection()
		switch {
		case a != b && !shift && k == ui.KeyLeft:
			e.MoveTo(a, false)
		case a != b && !shift:
			e.MoveTo(b, false)
		case k == ui.KeyLeft:
			e.MoveTo(e.caret-1, shift)
		default:
			e.MoveTo(e.caret+1, shift)
		}
	case ui.KeyUp, ui.KeyDown:
		d := 1
		if k == ui.KeyUp {
			d = -1
		}
		e.moveLines(d, shift)
	case ui.KeyPageUp, ui.KeyPageDown:
		n := max(int(e.height/max(e.lay.lineH, 1))-1, 1)
		if k == ui.KeyPageUp {
			n = -n
		}
		e.scrollBy(float32(n) * e.lay.lineH)
		e.moveLines(n, shift)
	case ui.KeyHome, ui.KeyEnd:
		e.stable()
		l := &e.lay
		if len(l.lines) == 0 {
			return true
		}
		ln := &l.lines[l.lineOf(e.caretDisp())]
		i := ln.start
		if k == ui.KeyEnd {
			i = ln.end
			if !ln.last && i > ln.start {
				i-- // stay on this line rather than start the next
			}
		}
		e.MoveTo(l.toDoc(i), shift)
	default:
		return false
	}
	return true
}

// stable drops what the layout shows at the caret but the caret moving
// would take away, so that the layout's indexes are the document's.
func (e *Editor) stable() {
	e.ghost, e.preedit = "", ""
	e.ensureLayout()
}

// moveLines moves the caret n lines down (up when negative), keeping its x.
func (e *Editor) moveLines(n int, extend bool) {
	e.stable()
	l := &e.lay
	if len(l.lines) == 0 {
		return
	}
	i := l.lineOf(e.caretDisp())
	if !e.hasGoal {
		e.goalX = l.x(&l.lines[i], e.caretDisp())
	}
	j := i + n
	var to int
	switch {
	case j < 0:
		to = 0
	case j >= len(l.lines):
		to = e.Doc.Len()
	default:
		to = l.toDoc(l.hitLine(j, e.goalX))
	}
	goal := e.goalX
	e.MoveTo(to, extend)
	e.goalX, e.hasGoal = goal, true
}

func (e *Editor) wordLeft() int {
	i := e.caret
	if i == 0 {
		return 0
	}
	a, _ := e.wordAt(i - 1)
	return a
}

func (e *Editor) wordRight() int {
	i := e.caret
	if i >= e.Doc.Len() {
		return i
	}
	_, b := e.wordAt(i)
	return b
}

// hit returns the document index at x, y of the element's box.
func (e *Editor) hit(x, y float32) int {
	e.ensureLayout()
	l := &e.lay
	if len(l.lines) == 0 {
		return 0
	}
	i := l.lineAt(y + e.scrollY)
	return l.toDoc(l.hitLine(i, x-l.left))
}

func (e *Editor) pointerDown(ev ui.InputEvent) bool {
	if e.compOpen && e.popRect.Contains(ev.X, ev.Y) && ev.Button == 0 {
		row := int((ev.Y - e.popRect.Y - popPad) / popRowH)
		if row >= 0 && row < len(e.comp) {
			e.compSel = row
			e.AcceptCompletion()
		}
		return true
	}
	if ev.Button == 1 {
		// The context menu acts on the selection, or where it was opened.
		if a, b := e.Selection(); a == b {
			e.MoveTo(e.hit(ev.X, ev.Y), false)
		}
		return false
	}
	if ev.Button != 0 {
		return false
	}
	e.stable()
	i := e.hit(ev.X, ev.Y)
	switch {
	case ev.Clicks == 2:
		a, b := e.wordAt(i)
		e.anchor, e.caret = a, b
	case ev.Clicks >= 3:
		a, b := e.paragraphAt(i)
		e.anchor, e.caret = a, b
	default:
		e.MoveTo(i, ev.Mods&ui.Shift != 0)
		e.dragging = true
	}
	e.compOpen = false
	e.wantFocus = true
	return true
}

func (e *Editor) scrollBy(dy float32) {
	e.scrollY = clampf(e.scrollY+dy, 0, e.maxScroll())
}

// zoomBy takes a Ctrl+wheel scroll of dy: a wheel's notch is one step,
// a touchpad's steps every zoomStep DIPs. Scrolling up makes the text
// larger.
func (e *Editor) zoomBy(dy float32, precise bool) {
	if !precise {
		e.zoomed = 0
		if dy != 0 {
			e.opts.Zoom(-sign(dy))
		}
		return
	}
	e.zoomed += dy
	for e.zoomed <= -zoomStep || e.zoomed >= zoomStep {
		d := sign(e.zoomed)
		e.zoomed -= float32(d) * zoomStep
		e.opts.Zoom(-d)
	}
}

const zoomStep = 40

func sign(v float32) int {
	if v < 0 {
		return -1
	}
	return 1
}

// maxScroll is how far the text scrolls: to its end, or in typewriter
// mode until its last line is mid-view.
func (e *Editor) maxScroll() float32 {
	if e.typewriter {
		return max(e.lay.height-e.height/2, 0)
	}
	return max(e.lay.height-e.height, 0)
}

func clampf(v, lo, hi float32) float32 { return max(lo, min(v, hi)) }
