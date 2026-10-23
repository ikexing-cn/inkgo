package desk

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/editor"
	"writer/workspace"
)

var markKinds = map[editor.MarkKind]string{
	editor.MarkForeshadow: workspace.MarkForeshadow,
	editor.MarkAI:         workspace.MarkAI,
	editor.MarkPayoff:     workspace.MarkPayoff,
}

func storedMarks(ms []editor.Mark) []workspace.ChapterMark {
	var out []workspace.ChapterMark
	for _, m := range ms {
		out = append(out, workspace.ChapterMark{Kind: markKinds[m.Kind], Start: m.Start, End: m.End, Note: m.Note, Orphaned: m.Orphaned, Quote: m.Quote,
			ID: m.Key, Of: m.Ref, At: m.At, Dropped: m.Dropped})
	}
	return out
}

func editorMarks(ms []workspace.ChapterMark) []editor.Mark {
	var out []editor.Mark
	for _, m := range ms {
		for k, name := range markKinds {
			if name == m.Kind {
				out = append(out, editor.Mark{Kind: k, Start: m.Start, End: m.End, Note: m.Note, Orphaned: m.Orphaned, Quote: m.Quote,
					Key: m.ID, Ref: m.Of, At: m.At, Dropped: m.Dropped})
			}
		}
	}
	return out
}

// firstChapter is the first document in the tree, "" when there is none.
func (a *App) firstChapter() string {
	first := ""
	a.book.Tree().Walk(func(n *workspace.Node, _ int) {
		if first == "" && !n.Folder {
			first = n.ID
		}
	})
	return first
}

// enterWriting reads the tree as the page shows, and opens the node last
// open, or the first document.
func (a *App) enterWriting() {
	w := &a.write
	if w.folded == nil {
		w.folded = map[string]bool{}
	}
	if err := a.loadTree(); err != nil {
		a.warn(err)
		return
	}
	a.warn(a.book.ReloadCounts())
	w.checked = time.Time{}
	a.keepTabs()
	if _, ok := a.book.Tree().Node(w.node); ok {
		if w.ed == nil && w.renaming == "" {
			a.openNode(w.node)
		}
		return
	}
	if w.blank && w.node == "" {
		return // every tab was closed: none opens by itself
	}
	w.node = a.firstChapter()
	if w.node != "" {
		a.openNode(w.node)
	}
}

// chapterEditor returns an editor of the text and marks of the chapter
// shown, continuing it with the writing model.
func (a *App) chapterEditor(text string, marks []workspace.ChapterMark) *editor.Editor {
	doc := editor.NewDoc(text)
	doc.LoadMarks(editorMarks(marks))
	return editor.New(doc, editor.Options{Prose: true, Placeholder: "在此开始写作…", Font: ui.Font{Size: float32(a.write.textSize())}, LineHeight: lhPaper,
		Predictor: pen{writer: &a.write.pen, chapter: a.write.node}, PredictDelay: saveAfter, PredictOnDemand: !a.write.autoPredict, NoAI: !a.aiOn(),
		GhostNote: func() string {
			if pw := a.write.pen.Load(); pw != nil {
				return (*pw).Name()
			}
			return ""
		},
		Zoom:    a.sizeText,
		OwnMenu: func(x, y float32) { a.menu = menuState{owner: "正文", x: x, y: y} },
	})
}

// textMenu is the menu of the chapter's text: what the clipboard does, what
// the AI, while it is on, and the setting do with the selection, and its
// foreshadowings.
func (a *App) textMenu(m *menu) {
	ed := a.write.ed
	s, e := ed.Selection()
	busy := ed.ReadOnly()
	m.Command(a, cmdCut).Act(a)
	m.Command(a, cmdCopy).Act(a)
	m.Command(a, cmdPaste).Act(a)
	m.Command(a, cmdSelectAll).Act(a)
	m.Separator()
	if a.aiOn() {
		// The row's own state is the editor's: it does not wait on the
		// chapter's saves as the title bar's does.
		m.Command(a, cmdContinue).Act(a).Disabled(busy)
		m.Command(a, cmdRewrite).Act(a)
		m.Command(a, cmdQuote).Act(a)
	}
	m.Command(a, cmdTake).Act(a)
	a.nameMenu(m, ed.Doc.Slice(s, e))
	m.Separator()
	a.foreMenu(m)
	if ed.Marked() {
		m.Command(a, cmdUnmark).Act(a)
	}
}

// openNode shows node id in its tab, a new one beside the tab shown when
// it has none, saving what was shown first. The tab left keeps its editor,
// and the one come back to has its own again: what can be undone, the
// caret, how far it is scrolled.
func (a *App) openNode(id string) {
	a.flush()
	a.leaveCheck()
	a.scanMarks()
	w := &a.write
	w.closing = nil // the question about saving was about the node shown
	from := w.node
	if id != from {
		a.park()
	}
	if !w.stepping && from != "" && from != id {
		w.back = append(w.back, from)
		if len(w.back) > 50 {
			w.back = w.back[1:]
		}
		w.forward = nil
	}
	w.node, w.ed, w.external, w.lost = id, nil, nil, false
	w.outline, w.outlineSaved, w.outlinePrev = "", "", ""
	w.changed = false
	w.current = 0
	kept := w.parked[id]
	delete(w.parked, id)
	if kept != nil && kept.lost {
		// Gone from the disk, its words are only here.
		a.unpark(kept)
		a.savePrefs()
		return
	}
	n, ok := a.book.Tree().Node(id)
	if !ok {
		w.node = ""
		return
	}
	ch, err := a.Store.ReadChapter(id)
	if err != nil {
		a.warn(err)
		w.node = ""
		return
	}
	w.outline, w.outlineSaved, w.outlinePrev = ch.Outline, ch.Outline, ch.Outline
	switch {
	case n.Folder:
		w.gists = a.gistsIn(n)
	case kept != nil && (kept.unsettled() || kept.same(ch)):
		a.unpark(kept)
		if !kept.unsettled() {
			w.stamp = ch.Stamp
		}
	default:
		// Not open before, or changed on the disk while its tab was not
		// shown: read anew.
		w.ed = a.chapterEditor(ch.Text, ch.Marks)
		if at, ok := w.carets[id]; ok {
			w.ed.MoveTo(at, false)
		}
		w.saved, w.savedVer, w.stamp = ch.Text, w.ed.Doc.Version(), ch.Stamp
		if strings.TrimSpace(ch.Text) != "" {
			_, err = a.Store.TakeSnapshot(id, workspace.SnapOpen, ch.Text, ch.Marks)
			a.warn(err)
		}
		w.lastSnap = time.Now()
	}
	if !slices.Contains(w.tabs, id) {
		w.tabs = slices.Insert(w.tabs, slices.Index(w.tabs, from)+1, id)
	}
	w.blank = false
	a.savePrefs()
}

// stepHistory opens the chapter opened before the one shown, or after it
// when going forward, as a browser goes back and forth between pages. A
// chapter since deleted is passed over.
func (a *App) stepHistory(forward bool) {
	w := &a.write
	from, to := &w.back, &w.forward
	if forward {
		from, to = to, from
	}
	for len(*from) > 0 {
		id := (*from)[len(*from)-1]
		*from = (*from)[:len(*from)-1]
		if _, ok := a.book.Tree().Node(id); !ok || id == w.node {
			continue
		}
		if w.node != "" {
			*to = append(*to, w.node)
		}
		w.stepping = true
		a.openNode(id)
		w.stepping = false
		return
	}
}

// gistsIn reads a line on each chapter in a folder: the start of its
// outline, else of its prose, so the folder reads as a plan.
func (a *App) gistsIn(n *workspace.Node) map[string]string {
	const gistLen = 200
	gists := map[string]string{}
	for _, child := range n.Children {
		if child.Folder {
			continue
		}
		ch, err := a.Store.ReadChapter(child.ID)
		if err != nil {
			continue
		}
		gist := ch.Outline
		if strings.TrimSpace(gist) == "" {
			gist = ch.Text
		}
		// Two lines show of it at most.
		if r := []rune(gist); len(r) > gistLen {
			gist = string(r[:gistLen])
		}
		if gist = strings.Join(strings.Fields(gist), " "); gist != "" {
			gists[child.ID] = gist
		}
	}
	return gists
}

// Sizes of the text: the default, and the least and most it goes to.
const (
	textSizeDefault = 17
	textSizeMin     = 13
	textSizeMax     = 28
)

// textSize is the size the chapter's text is set in.
func (w *writeView) textSize() int {
	if w.size == 0 {
		return textSizeDefault
	}
	return w.size
}

// sizeText makes the text a size larger (d > 0) or smaller, or back to
// the default (d == 0), and keeps the size with the work.
func (a *App) sizeText(d int) {
	w := &a.write
	size := textSizeDefault
	if d != 0 {
		size = min(max(w.textSize()+d, textSizeMin), textSizeMax)
	}
	if size == w.textSize() {
		return
	}
	w.size = size
	if size == textSizeDefault {
		w.size = 0
	}
	if w.ed != nil {
		w.ed.SetFontSize(float32(size))
	}
	a.savePrefs()
}

// keepCaret notes where the caret is in the chapter shown, to put it back
// there when the chapter opens again, in this run or the next.
func (a *App) keepCaret() {
	w := &a.write
	if w.ed == nil || w.node == "" {
		return
	}
	if at, ok := w.carets[w.node]; ok && at == w.ed.Caret() {
		return
	}
	if w.carets == nil {
		w.carets = map[string]int{}
	}
	w.carets[w.node] = w.ed.Caret()
	for id := range w.carets {
		if _, ok := a.book.Tree().Node(id); !ok && a.book.Tree().Root != nil {
			delete(w.carets, id) // gone from the tree
		}
	}
	a.savePrefs()
}

// stepChapter opens the chapter before (d < 0) or after the node open, in
// the tree's order, passing over folders; at either end it stays.
func (a *App) stepChapter(d int) {
	w := &a.write
	var order []string
	at, folder := -1, false
	a.book.Tree().Walk(func(n *workspace.Node, _ int) {
		if n.ID == w.node {
			at, folder = len(order), n.Folder
		}
		if !n.Folder {
			order = append(order, n.ID)
		}
	})
	next := at + d
	switch {
	case at < 0: // nothing open: from the start
		next = 0
	case folder && d > 0:
		next = at // a folder sits just before its first chapter
	}
	if next < 0 || next >= len(order) {
		return
	}
	a.openNode(order[next])
}
