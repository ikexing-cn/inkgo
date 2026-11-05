package editor

import (
	"strings"
	"unicode"
	"writer/locales"

	"github.com/egoist/mygo/ui"
)

// View builds the editor's element; size it as any element (Fill, Grow).
func (e *Editor) View(c *ui.Context) ui.Element {
	e.sv = c.Services()
	e.predict.poll(e)
	el := ui.Box(c).Focusable().FocusRing(false).Cursor(ui.CursorText).Label("正文")
	// While a column folds the scroll bar may stand outside the box; the
	// text clips itself in paint.
	if e.barOffset == 0 {
		el.Clip()
	}
	barHot := false
	if e.maxScroll() > 0 {
		el.Children(func() {
			bar := ui.Box(c.Key("scrollbar")).Absolute().Right(-e.barOffset).Top(0).Bottom(0).Width(barW).Cursor(ui.CursorDefault).Label("正文滚动条")
			_, _, barHot = bar.PointerPosition()
			bar.HandleInput(func(ev ui.InputEvent) bool {
				ev.X += e.width + e.barOffset - barW
				return e.input(ev)
			})
		})
	}
	x, y, over := el.PointerPosition()
	e.barHot = barHot || over && e.onBar(x)
	e.nameHover = nameRange{}
	e.lineHover = [2]int{}
	if over && !e.barHot && !e.barDrag {
		e.nameHover = e.nameRangeAt(x, y)
		e.lineHover = e.markedRangeAt(x, y)
	}
	switch {
	case e.barHot || e.barDrag:
		el.Cursor(ui.CursorDefault)
	case e.proseMarkAt(x, y) != nil && e.proseMarkAt(x, y).Open != nil:
		el.Cursor(ui.CursorPointer)
	case e.nameHover.entry != "":
		el.Tooltip(locales.EditorSeeEntry()).Cursor(ui.CursorPointer)
	}
	if e.wantFocus {
		el.Focus()
		e.wantFocus = false
	}
	e.focused = el.Focused()
	if !e.focused {
		e.preedit = "" // input methods drop their composition with the focus
	}
	e.predict.ask(e, c)
	el.HandleInput(e.input)
	el.TextCaret(e.caretRect())
	el.Draw(e.paint)
	el.DrawOver(e.paintPopup)
	e.contextMenu(el)
	return el
}

// Focus gives the editor the keyboard focus in the next frame.
func (e *Editor) Focus() { e.wantFocus = true }

// Focused reports whether the editor had the keyboard focus as it was last
// built.
func (e *Editor) Focused() bool { return e.focused }

// Unfocus takes back a Focus not yet acted on. A new editor asks for the
// focus; one opened beside a field that is to be typed in first leaves it.
func (e *Editor) Unfocus() { e.wantFocus = false }

// SetEntities sets the names that complete as they are typed.
func (e *Editor) SetEntities(ents []Entity) {
	e.opts.Entities = ents
	if e.compOpen {
		e.updateCompletion()
	}
}

// SetTypewriter turns typewriter scrolling on or off: while on, typing
// keeps the line written on in the middle of the view.
func (e *Editor) SetTypewriter(on bool) {
	if on != e.typewriter {
		e.typewriter, e.reveal = on, true
		e.centered = -1
	}
}

func (e *Editor) FontSize() float32 { return e.opts.Font.Size }

// SetTextColumn changes the text's line width and margins without moving
// the viewport or its scroll bar.
func (e *Editor) SetTextColumn(column TextColumn) {
	if column != e.opts.Column {
		e.opts.Column, e.reveal = column, true
	}
}

// SetScrollBarOffset places the scroll bar offset DIPs from the viewport's
// right edge, including its pointer target. Positive is to the right.
func (e *Editor) SetScrollBarOffset(offset float32) { e.barOffset = offset }

// SetFontSize sets the text in px, keeping the caret in view.
func (e *Editor) SetFontSize(px float32) {
	if px > 0 && px != e.opts.Font.Size {
		e.opts.Font.Size, e.reveal = px, true
		e.shapes = nil // shaped at the old size
	}
}

type layoutKey struct {
	ver, caret, props int
	preedit, ghost    string
	width, size       float32
	column            TextColumn
}

func (e *Editor) ensureLayout() {
	extra, _ := e.extra()
	caret := 0
	if e.preedit != "" || extra != "" {
		caret = e.caret
	}
	k := layoutKey{e.Doc.textVer, caret, e.propVer, e.preedit, extra, e.width, e.opts.Font.Size, e.opts.Column}
	if k != e.layKey || e.lay.lines == nil {
		e.buildLayout(e.width)
		e.layKey = k
	}
}

// caretDisp is the display index the caret shows at: in the composition
// where the input method puts it, else before the prediction.
func (e *Editor) caretDisp() int {
	if e.preedit != "" {
		return e.lay.extraAt + clamp(e.preeditCaret, 0, len([]rune(e.preedit)))
	}
	return e.lay.toDisp(e.caret, false)
}

// extra is what shows at the caret without being in the text, besides
// the composition: the prediction; and where.
func (e *Editor) extra() (string, int) {
	return e.Ghost(), e.caret
}

// caretPos returns the caret's x and its line's top, in content
// coordinates.
func (e *Editor) caretPos() (float32, float32) {
	e.ensureLayout()
	l := &e.lay
	if len(l.lines) == 0 {
		return l.left, padTop
	}
	d := e.caretDisp()
	ln := &l.lines[l.lineOf(d)]
	return l.left + l.x(ln, d), ln.y
}

// caretRect is where input methods show their candidates, relative to the
// element.
func (e *Editor) caretRect() ui.Rect {
	if _, ok := e.geometry(); !ok {
		return ui.Rect{}
	}
	x, y := e.caretPos()
	return ui.Rect{X: x, Y: y - e.scrollY, W: 2, H: e.lay.lineH}
}

func (e *Editor) geometry() (*layout, bool) {
	if e.width == 0 {
		return nil, false
	}
	e.ensureLayout()
	return &e.lay, len(e.lay.lines) != 0
}

// SelectionTop returns where the selection starts, relative to the
// element: X is where, Y and H the top and height of its line. ok is false
// without a selection, while the mouse still drags it, and while its start
// is out of view.
func (e *Editor) SelectionTop() (r ui.Rect, ok bool) {
	a, b := e.Selection()
	if a == b || e.dragging {
		return ui.Rect{}, false
	}
	l, ready := e.geometry()
	if !ready {
		return ui.Rect{}, false
	}
	d := l.toDisp(a, true)
	ln := &l.lines[l.lineOf(d)]
	y := ln.y - e.scrollY
	if y < 0 || y+l.lineH > e.height {
		return ui.Rect{}, false
	}
	return ui.Rect{X: l.left + l.x(ln, d), Y: y, H: l.lineH}, true
}

// RangeBox returns where the range [a, b) lies, relative to the element:
// X is where it starts, or where the lines start when it runs over several,
// Y the top of its first line and H the height down to the bottom of its
// last. ok is false before the editor was painted and
// while none of the range is in view.
func (e *Editor) RangeBox(a, b int) (r ui.Rect, ok bool) {
	r, ok = e.RangeBoxUnclipped(a, b)
	if ok && (r.Y+r.H < 0 || r.Y > e.height) {
		return ui.Rect{}, false
	}
	return r, ok
}

// RangeBoxUnclipped is RangeBox including ranges above or below the viewport.
func (e *Editor) RangeBoxUnclipped(a, b int) (r ui.Rect, ok bool) {
	l, ready := e.geometry()
	if !ready {
		return ui.Rect{}, false
	}
	a, b = e.Doc.span(a, b)
	d := l.toDisp(a, true)
	first, last := &l.lines[l.lineOf(d)], &l.lines[l.lineOf(l.toDisp(b, false))]
	top, bottom := first.y-e.scrollY, last.y+l.lineH-e.scrollY
	x := l.left + l.x(first, d)
	if first != last {
		x = l.left // over several lines it starts where the lines do
	}
	return ui.Rect{X: x, Y: top, H: bottom - top}, true
}

func (e *Editor) Height() float32 { return e.height }

func (e *Editor) Width() float32 { return e.width }

// contextMenu shows the editor's own menu, or tells the host that draws
// one where it was asked for.
func (e *Editor) contextMenu(el ui.Element) {
	if e.opts.OwnMenu == nil {
		el.ContextMenu(e.menu)
		return
	}
	at := el.Bounds()
	if el.RightClicked() {
		x, y, _ := el.PointerPosition()
		e.opts.OwnMenu(at.X+x, at.Y+y)
	} else if e.menuKey {
		r := e.caretRect()
		e.opts.OwnMenu(at.X+r.X, at.Y+r.Y+r.H)
	}
	e.menuKey = false
}

// menu is the editor's context menu.
func (e *Editor) menu(m *ui.Menu) {
	a, b := e.Selection()
	if e.opts.ReadOnly {
		if m.Item(locales.CommonCopy()).Shortcut(ui.Cmd, ui.KeyC).Disabled(a == b).Chosen() {
			e.Copy()
		}
		if m.Item(locales.CommonSelectAll()).Shortcut(ui.Cmd, ui.KeyA).Chosen() {
			e.SelectAll()
		}
		return
	}
	if m.Item(locales.CommonCut()).Shortcut(ui.Cmd, ui.KeyX).Disabled(a == b).Chosen() {
		e.Cut()
	}
	if m.Item(locales.CommonCopy()).Shortcut(ui.Cmd, ui.KeyC).Disabled(a == b).Chosen() {
		e.Copy()
	}
	if m.Item(locales.CommonPaste()).Shortcut(ui.Cmd, ui.KeyV).Chosen() {
		e.Paste()
	}
	if m.Item(locales.CommonSelectAll()).Shortcut(ui.Cmd, ui.KeyA).Chosen() {
		e.SelectAll()
	}
	m.Separator()
	if m.Item(locales.EditorMarkForeshadow()).Disabled(a == b).Chosen() {
		e.MarkSelection(MarkForeshadow, "")
	}
	if m.Item(locales.CmdUnmark()).Disabled(!e.Marked()).Chosen() {
		e.Unmark()
	}
}

// Marked reports whether the caret is in a mark.
func (e *Editor) Marked() bool { return e.markAt(e.caret) != 0 }

// Unmark removes the innermost mark around the caret, as one step to undo.
func (e *Editor) Unmark() {
	id := e.markAt(e.caret)
	if id == 0 {
		return
	}
	before := e.Doc.Marks()
	e.Doc.RemoveMark(id)
	e.pushMarkStep(before)
}

// unindent drops the indent before each line of s.
func unindent(s string) string {
	lines := strings.Split(normalize(s), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimLeftFunc(l, unicode.IsSpace)
	}
	return strings.Join(lines, "\n")
}

// markAt returns the id of the innermost mark around i, or 0.
func (e *Editor) markAt(i int) int {
	id, size := 0, -1
	for _, m := range e.Doc.marks {
		if m.Start <= i && i <= m.End && m.End > m.Start && !e.hidden(m) && (size < 0 || m.End-m.Start < size) {
			id, size = m.ID, m.End-m.Start
		}
	}
	return id
}

func (e *Editor) Copy() {
	if s := e.SelectedText(); s != "" {
		e.sv.WriteClipboard(s)
	}
}

func (e *Editor) Cut() {
	a, b := e.Selection()
	if a == b {
		return
	}
	e.Copy()
	e.replaceRange(a, b, "", false)
}

func (e *Editor) Paste() {
	if s := e.sv.ReadClipboard(); s != "" {
		if e.opts.Prose {
			s = unindent(s)
		}
		a, b := e.Selection()
		e.replaceRange(a, b, s, false)
		e.compOpen = false
	}
}

// ScrollAnchor matches a fraction through the prose viewport to a
// paragraph and a fraction of its height. A second layout can place that
// same point at that same viewport fraction, including both endpoints.
func (e *Editor) ScrollAnchor() (fraction float32, paragraph int, into float32) {
	e.ensureLayout()
	if most := e.maxScroll(); most > 0 {
		fraction = clampf(e.scrollY/most, 0, 1)
	}
	y := e.scrollY + fraction*e.height
	lines := e.lay.lines
	for i := 0; i < len(lines); {
		j := i + 1
		for j < len(lines) && lines[j].pstart == lines[i].pstart {
			j++
		}
		bottom := lines[j-1].y + e.lay.lineH
		if bottom >= y || j == len(lines) {
			// Proposed paragraphs and IME text are display-only. Locate
			// this point in the source document before naming its paragraph.
			paragraph = 0
			for _, r := range e.Doc.text[:clamp(e.lay.toDoc(lines[i].pstart), 0, e.Doc.Len())] {
				if r == '\n' {
					paragraph++
				}
			}
			return fraction, paragraph, clampf((y-lines[i].y)/max(bottom-lines[i].y, 1), 0, 1)
		}
		i = j
	}
	return
}
