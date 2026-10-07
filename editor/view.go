package editor

import (
	"context"
	"time"

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

type layoutKey struct {
	ver, caret     int
	preedit, ghost string
	width          float32
}

// ensureLayout lays the text out again when it, the caret or the width
// changed since the last time.
func (e *Editor) ensureLayout() {
	k := layoutKey{e.Doc.version, e.caret, e.preedit, e.Ghost(), e.width}
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
	return e.lay.extraAt
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

// maybePredict asks the predictor once typing paused at the end of a
// paragraph.
func (e *Editor) maybePredict(c *ui.Context) {
	if e.opts.Predictor == nil || !e.focused || e.predicting || e.caret != e.anchor || e.preedit != "" ||
		e.predictVer == e.Doc.version && e.predictAt == e.caret {
		return
	}
	if e.caret < e.Doc.Len() && e.Doc.text[e.caret] != '\n' {
		return // predictions continue a paragraph, not the middle of one
	}
	if wait := e.opts.PredictDelay - time.Since(e.lastEdit); wait > 0 {
		c.After(wait)
		return
	}
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
	if m.Item("标记为伏笔").Disabled(a == b).Chosen() {
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
		a, b := e.Selection()
		e.replaceRange(a, b, s, false)
		e.compOpen = false
	}
}
