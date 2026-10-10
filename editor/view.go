package editor

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
)

// View builds the editor's element; size it as any element (Fill, Grow).
func (e *Editor) View(c *ui.Context) *ui.Element {
	e.c = c
	e.pollPrediction()
	el := ui.Box(c).Focusable().FocusRing(false).Cursor(ui.CursorText).Clip().Label("正文")
	if e.wantFocus {
		el.Focus()
		e.wantFocus = false
	}
	e.focused = el.Focused()
	if !e.focused {
		e.preedit = "" // input methods drop their composition with the focus
	}
	e.maybePredict(c)
	el.HandleInput(e.input)
	el.TextCaret(e.caretRect())
	el.Draw(e.paint)
	el.DrawOver(e.paintPopup)
	el.ContextMenu(e.menu)
	return el
}

// Focus gives the editor the keyboard focus in the next frame.
func (e *Editor) Focus() { e.wantFocus = true }

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

// FontSize is the size the text is set in.
func (e *Editor) FontSize() float32 { return e.opts.Font.Size }

// SetFontSize sets the text in px, keeping the caret in view.
func (e *Editor) SetFontSize(px float32) {
	if px > 0 && px != e.opts.Font.Size {
		e.opts.Font.Size, e.reveal = px, true
		e.shapes = map[string]shaped{} // shaped at the old size
	}
}

type layoutKey struct {
	ver, caret     int
	preedit, ghost string
	width, size    float32
}

// ensureLayout lays the text out again when it, the caret or the width
// changed since the last time.
func (e *Editor) ensureLayout() {
	extra, _ := e.extra()
	k := layoutKey{e.Doc.version, e.caret, e.preedit, extra, e.width, e.opts.Font.Size}
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

// extra is what shows in the text without being in it, besides the
// composition: the rewrite proposed, or else the prediction; and where.
func (e *Editor) extra() (string, int) {
	if e.proposing() {
		return e.prop.text, e.prop.b
	}
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
	if e.width == 0 {
		return ui.Rect{}
	}
	x, y := e.caretPos()
	return ui.Rect{X: x, Y: y - e.scrollY, W: 2, H: e.lay.lineH}
}

// SelectionTop returns where the selection starts, relative to the
// element: X is where, Y and H the top and height of its line. ok is false
// without a selection, while the mouse still drags it, and while its start
// is out of view.
func (e *Editor) SelectionTop() (r ui.Rect, ok bool) {
	a, b := e.Selection()
	if a == b || e.dragging || e.width == 0 {
		return ui.Rect{}, false
	}
	e.ensureLayout()
	l := &e.lay
	if len(l.lines) == 0 {
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
	if e.width == 0 {
		return ui.Rect{}, false
	}
	e.ensureLayout()
	l := &e.lay
	if len(l.lines) == 0 {
		return ui.Rect{}, false
	}
	a, b = clamp(min(a, b), 0, e.Doc.Len()), clamp(max(a, b), 0, e.Doc.Len())
	d := l.toDisp(a, true)
	first, last := &l.lines[l.lineOf(d)], &l.lines[l.lineOf(l.toDisp(b, false))]
	top, bottom := first.y-e.scrollY, last.y+l.lineH-e.scrollY
	if bottom < 0 || top > e.height {
		return ui.Rect{}, false
	}
	x := l.left + l.x(first, d)
	if first != last {
		x = l.left // over several lines it starts where the lines do
	}
	return ui.Rect{X: x, Y: top, H: bottom - top}, true
}

// Height is the element's height when it was last painted.
func (e *Editor) Height() float32 { return e.height }

// Width is the element's width when it was last painted.
func (e *Editor) Width() float32 { return e.width }

// maybePredict asks the predictor once typing paused at the end of a
// paragraph.
func (e *Editor) maybePredict(c *ui.Context) {
	if e.opts.ReadOnly || e.opts.Predictor == nil || !e.focused || e.predicting || e.caret != e.anchor || e.preedit != "" ||
		e.predictVer == e.Doc.version && e.predictAt == e.caret {
		return
	}
	if e.opts.PredictOnDemand && !e.asked || e.compOpen && !e.asked {
		return // while completions are offered, Tab is theirs
	}
	if !e.asked {
		if e.caret < e.Doc.Len() && e.Doc.text[e.caret] != '\n' {
			return // predictions continue a paragraph, not the middle of one
		}
		if wait := e.opts.PredictDelay - time.Since(e.lastEdit); wait > 0 {
			c.After(wait)
			return
		}
	}
	e.asked = false
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	e.cancel, e.predicting = cancel, true
	e.predictVer, e.predictAt = e.Doc.version, e.caret
	ver, at := e.Doc.version, e.caret
	from := max(0, at-2000)
	before := e.Doc.Slice(from, at)
	p, results := e.opts.Predictor, e.results
	go func() {
		s, err := p.Predict(ctx, before)
		results <- prediction{text: singleLine(s), err: err, ver: ver, at: at}
		c.Invalidate()
	}()
}

// pollPrediction takes the predictions that came, keeping one that still
// applies.
func (e *Editor) pollPrediction() {
	for {
		select {
		case r := <-e.results:
			if r.ver != e.predictVer || r.at != e.predictAt {
				continue // a request cancelled since
			}
			e.predicting, e.cancel = false, nil
			if r.err == nil && r.ver == e.Doc.version && r.at == e.caret && r.text != "" {
				e.ghost, e.ghostAt, e.ghostVer = r.text, r.at, r.ver
			}
		default:
			return
		}
	}
}

func (e *Editor) cancelPredict() {
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	e.predicting = false
}

// DismissGhost drops the prediction shown, without asking again here.
func (e *Editor) DismissGhost() {
	e.ghost = ""
	e.cancelPredict()
	e.predictVer, e.predictAt = e.Doc.version, e.caret
}

// menu is the editor's context menu.
func (e *Editor) menu(m *ui.Menu) {
	a, b := e.Selection()
	if e.opts.ReadOnly {
		if m.Item("复制").Shortcut(ui.Cmd, ui.KeyC).Disabled(a == b).Chosen() {
			e.copy()
		}
		if m.Item("全选").Shortcut(ui.Cmd, ui.KeyA).Chosen() {
			e.SelectAll()
		}
		return
	}
	if m.Item("剪切").Shortcut(ui.Cmd, ui.KeyX).Disabled(a == b).Chosen() {
		e.cut()
	}
	if m.Item("复制").Shortcut(ui.Cmd, ui.KeyC).Disabled(a == b).Chosen() {
		e.copy()
	}
	if m.Item("粘贴").Shortcut(ui.Cmd, ui.KeyV).Chosen() {
		e.paste()
	}
	if m.Item("全选").Shortcut(ui.Cmd, ui.KeyA).Chosen() {
		e.SelectAll()
	}
	m.Separator()
	if !e.opts.OwnMarks && m.Item("标记为伏笔").Disabled(a == b).Chosen() {
		e.MarkSelection(MarkForeshadow, "")
	}
	id := e.markAt(e.caret)
	if m.Item("移除此处标记").Disabled(id == 0).Chosen() {
		before := e.Doc.Marks()
		e.Doc.RemoveMark(id)
		e.undo = append(e.undo, undoStep{at: e.caret, caretBefore: e.caret, anchorBefore: e.anchor, caretAfter: e.caret,
			marksBefore: before, marksAfter: e.Doc.Marks(), when: time.Now()})
		e.redo = nil
	}
	if e.opts.Menu != nil {
		m.Separator()
		e.opts.Menu(m)
	}
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
		if m.Start <= i && i <= m.End && m.End > m.Start && (size < 0 || m.End-m.Start < size) {
			id, size = m.ID, m.End-m.Start
		}
	}
	return id
}

func (e *Editor) copy() {
	if s := e.SelectedText(); s != "" && e.c != nil {
		e.c.WriteClipboard(s)
	}
}

func (e *Editor) cut() {
	a, b := e.Selection()
	if a == b {
		return
	}
	e.copy()
	e.replaceRange(a, b, "", false)
}

func (e *Editor) paste() {
	if e.c == nil {
		return
	}
	if s := e.c.ReadClipboard(); s != "" {
		if e.opts.Prose {
			s = unindent(s)
		}
		a, b := e.Selection()
		e.replaceRange(a, b, s, false)
		e.compOpen = false
	}
}
