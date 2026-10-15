package desk

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"writer/desk/aiport"
	"writer/editor"
	"writer/workspace"
)

// The writing page saves as the author pauses, looks at the disk for
// changes made outside, and keeps a snapshot every so often while they
// write. Tests shorten these.
var (
	saveAfter  = 1500 * time.Millisecond
	checkEvery = 2 * time.Second
	snapEvery  = 10 * time.Minute
)

// writeView is the state of the writing page.
type writeView struct {
	entered bool // the page read the tree as it showed
	tree    workspace.Tree
	counts  map[string]int
	folded  map[string]bool // folders folded, by ID
	node    string          // the node shown
	// back and forward are the nodes shown before and after it, for the
	// toolbar's arrows; stepping is set while they open one.
	back, forward []string
	stepping      bool
	// tabs are the nodes open, left to right, node the one of them shown;
	// parked is what the others keep while they are not; closing, the tabs
	// whose closing waits for an answer about the words not saved; blank,
	// that the author closed them all; tabSeen, the tab the row last
	// showed as the one open, which it scrolled to.
	tabs    []string
	parked  map[string]*sheet
	closing []string
	blank   bool
	tabSeen string
	ed      *editor.Editor
	// What the disk has of the node shown, as of the last read or write.
	saved    string
	savedVer int
	// counted is the text the open chapter's count was last taken of.
	counted    *editor.Doc
	countedVer int
	stamp      workspace.Stamp
	lastSnap   time.Time
	checked    time.Time
	// The outline, saved as the author pauses like the text.
	outline, outlineSaved, outlinePrev string
	outlineAt                          time.Time
	outlineOpen                        bool
	// The summary of the chapter shown, as stored and as being edited;
	// changed, whether its prose was saved since it opened.
	sum                   workspace.Summary
	summary, summarySaved string
	summaryOpen           bool
	changed               bool
	autoSummary           bool
	day                   string // the local date the count below was taken
	dayStart              int    // the book's count as that day began
	goal                  int    // a day's goal, or 0
	goalText              string // the goal as typed
	goalOpen              bool
	offered               string            // the numbered title a new node's name began with
	days                  map[string]int    // what was written each day before, by local date
	size                  int               // the text's size, or 0 for the default
	gists                 map[string]string // a line on each chapter of the folder open
	carets                map[string]int    // where the caret was in each chapter, as it was left
	unsummarized          []string          // the chapters before without a summary, as a draft opens
	recapped              int               // and how many before have one
	recapStale            bool              // summaries were being written since the two were read
	// The disk's text, changed outside while the node was open; lost, the
	// node gone from the disk.
	external  *workspace.Chapter
	lost      bool
	lostTitle string

	renaming, title string
	titleWas        string // the name kept when the title is left empty
	titleHeld       bool   // the box for the title has had the caret: losing it settles the name
	titleAll        bool   // the name the box opens with is selected, to be typed over
	focus           bool   // the page alone, without the tree and the column
	finding         bool
	query, with     string
	current         int
	wide            bool              // finding in the whole book
	sure            bool              // asking before replacing through the book
	texts           map[string]string // the chapters' text while finding in the book
	hits            []bookHit
	hitsFor         [3]any // the query, chapter and edit the hits were found for

	// named is the editor last given the names, as of namedRevs revisions
	// of the setting.
	named     *editor.Editor
	namedRevs int

	// What is pending: the marks of the chapters as of the disk, the list
	// open, showing the open chapter's alone, the groups listed whole, and
	// the keys of what the author ignored.
	marks    map[string][]workspace.ChapterMark
	pendOpen bool
	pendHere int
	pendMore map[string]bool
	ignored  map[string]bool

	// The foreshadowings: what their column and popovers show, and how
	// many chapters may follow one before it is buried long (0: the
	// default).
	fore      foreState
	lateAfter int

	snapsOpen bool
	snaps     []workspace.Snapshot
	snap      string // the snapshot chosen
	snapText  string
	compare   bool
	snapShown snapView

	trashOpen bool
	trash     []workspace.Trashed
	purge     string // the trashed node whose purge awaits confirmation

	takeOpen               bool
	takeQuote, takeText    string
	takeCategory           string
	exportOpen, exportDone bool
	indent                 bool
	exportFrom             string // the node exported with what is in it, or "" for the book
	exportAt, draftAt      int    // which of the dialog's choices is taken: the control writes it once the frame is drawn

	// The writing model: the writer the editors predict with, and why
	// there is none; a request in flight; a rewrite asked for or shown.
	pen         atomic.Pointer[aiport.Pen]
	penKey      string
	penErr      error
	autoPredict bool
	run         *aiRun
	rewriting   bool
	instruction string
	proposed    *aiport.Job
	draftOpen   bool
	words       string
	draftMode   string
}

// dragNode is a node of the chapter tree being dragged.
type dragNode string

var markKinds = map[editor.MarkKind]string{
	editor.MarkForeshadow: workspace.MarkForeshadow,
	editor.MarkAIAccepted: workspace.MarkAIAccepted,
	editor.MarkAIDraft:    workspace.MarkAIDraft,
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

// warn shows err, leaving an error shown alone when there is none.
func (a *App) warn(err error) {
	if err != nil {
		a.Error = err.Error()
	}
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
	counts, err := a.Store.Counts()
	a.warn(err)
	w.counts = counts
	w.checked = time.Time{}
	a.keepTabs()
	if _, ok := w.tree.Node(w.node); ok {
		if w.ed == nil && w.renaming == "" {
			a.openNode(w.node)
		}
		return
	}
	if w.blank && w.node == "" {
		return // every tab was closed: none opens by itself
	}
	w.node = ""
	w.tree.Walk(func(n *workspace.Node, _ int) {
		if w.node == "" && !n.Folder {
			w.node = n.ID
		}
	})
	if w.node != "" {
		a.openNode(w.node)
	}
}

func (a *App) loadTree() error {
	t, err := a.Store.Chapters()
	if err == nil {
		a.write.tree = t
	}
	return err
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
	m.Item("剪切").Hint(modKey + "X").Disabled(s == e || busy).Do(ed.Cut)
	m.Item("复制").Hint(modKey + "C").Disabled(s == e).Do(ed.Copy)
	m.Item("粘贴").Hint(modKey + "V").Disabled(busy).Do(ed.Paste)
	m.Item("全选").Hint(modKey + "A").Do(ed.SelectAll)
	m.Separator()
	if a.aiOn() {
		m.Item("续写一句").Hint(modKey + "J").Disabled(busy).Do(a.continueText)
		m.Item("AI 改写…").Disabled(s == e || busy).Do(a.openRewrite)
		m.Item("引用到讨论").Hint(modKey + "L").Disabled(s == e).Do(a.quoteToTalk)
	}
	m.Item("加入设定").Disabled(s == e).Do(a.openTake)
	a.nameMenu(m, ed.Doc.Slice(s, e))
	m.Separator()
	a.foreMenu(m)
	if ed.Marked() {
		m.Item("移除此处标记").Disabled(busy).Do(ed.Unmark)
	}
}

// openNode shows node id in its tab, a new one beside the tab shown when
// it has none, saving what was shown first. The tab left keeps its editor,
// and the one come back to has its own again: what can be undone, the
// caret, how far it is scrolled.
func (a *App) openNode(id string) {
	a.flush()
	a.leaveCheck()
	a.leaveChapter()
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
	w.sum, w.summary, w.summarySaved, w.changed = workspace.Summary{}, "", "", false
	w.current = 0
	kept := w.parked[id]
	delete(w.parked, id)
	if kept != nil && kept.lost {
		// Gone from the disk, its words are only here.
		a.unpark(kept)
		a.savePrefs()
		return
	}
	n, ok := w.tree.Node(id)
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
		a.loadSummary(id)
	default:
		// Not open before, or changed on the disk while its tab was not
		// shown: read anew.
		w.ed = a.chapterEditor(ch.Text, ch.Marks)
		if at, ok := w.carets[id]; ok {
			w.ed.MoveTo(at, false)
		}
		w.saved, w.savedVer, w.stamp = ch.Text, w.ed.Doc.Version(), ch.Stamp
		a.loadSummary(id)
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
		if _, ok := w.tree.Node(id); !ok || id == w.node {
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
// outline, else of its summary, so the folder reads as a plan.
func (a *App) gistsIn(n *workspace.Node) map[string]string {
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
			if s, err := a.Store.ReadSummary(child.ID); err == nil {
				gist = s.Text
			}
		}
		if gist = strings.Join(strings.Fields(gist), " "); gist != "" {
			gists[child.ID] = gist
		}
	}
	return gists
}

// flush saves the text and the outline shown, as leaving them does; an AI
// writing into the text stops first.
func (a *App) flush() {
	if a.Store == nil {
		return
	}
	a.stopAI()
	a.keepCaret()
	a.warn(a.saveText())
	a.warn(a.saveOutline())
	a.warn(a.saveSummary())
}

// saveNow is Ctrl+S: what the page shows goes to the disk now, though it
// would anyway as the author pauses. An AI writing or a rewrite waiting is
// left alone; what it has written is saved when it ends.
func (a *App) saveNow() {
	w := &a.write
	if w.run != nil || w.proposed != nil {
		a.notice = "AI 正在修改本章，完成后自动保存"
		return
	}
	for _, err := range []error{a.saveText(), a.saveOutline(), a.saveSummary()} {
		if err != nil {
			a.warn(err)
			return
		}
	}
	a.keepCaret()
	a.notice = "已保存。停笔片刻也会自动保存"
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
		if _, ok := w.tree.Node(id); !ok && w.tree.Root != nil {
			delete(w.carets, id) // gone from the tree
		}
	}
	a.savePrefs()
}

// saveText writes the text shown when it changed since the disk had it,
// with a snapshot when writing has gone on long enough since the last.
func (a *App) saveText() error {
	w := &a.write
	if w.ed == nil || w.external != nil || w.lost || w.ed.Doc.Version() == w.savedVer {
		return nil
	}
	text := w.ed.Doc.String()
	marks := storedMarks(w.ed.Doc.Marks())
	stamp, err := a.Store.WriteChapterOver(w.node, text, marks, w.stamp)
	if errors.Is(err, workspace.ErrChangedOutside) {
		// Changed outside since the disk was last looked at. Other words
		// there wait for the author to choose, as when the look finds
		// them; the same words under a new date are written over.
		if a.lookAtText(); w.external != nil {
			return nil
		}
		stamp, err = a.Store.WriteChapterOver(w.node, text, marks, w.stamp)
	}
	if err != nil {
		return err
	}
	w.stamp, w.saved, w.savedVer = stamp, text, w.ed.Doc.Version()
	w.changed = true
	if w.counts != nil {
		w.counts[w.node] = workspace.Count(text)
	}
	if time.Since(w.lastSnap) >= snapEvery {
		if _, err = a.Store.TakeSnapshot(w.node, workspace.SnapInterval, text, marks); err != nil {
			return err
		}
		w.lastSnap = time.Now()
	}
	return nil
}

func (a *App) saveOutline() error {
	w := &a.write
	if w.node == "" || w.outline == w.outlineSaved || w.lost {
		return nil
	}
	if err := a.Store.WriteOutline(w.node, w.outline); err != nil {
		return err
	}
	w.outlineSaved = w.outline
	return nil
}

// watch saves what paused long enough and looks at the disk, each frame
// of the writing page.
func (a *App) watch(c *ui.Context) {
	w := &a.write
	if len(w.closing) > 0 {
		return // asked whether to save: nothing is saved until the answer
	}
	if w.ed != nil && w.ed.Doc.Version() != w.savedVer && w.external == nil && !w.lost {
		if wait := saveAfter - time.Since(w.ed.LastEdit()); wait > 0 {
			c.After(wait)
		} else {
			a.warn(a.saveText())
		}
	}
	if w.outline != w.outlinePrev {
		w.outlinePrev, w.outlineAt = w.outline, time.Now()
	}
	if w.outline != w.outlineSaved {
		if wait := saveAfter - time.Since(w.outlineAt); wait > 0 {
			c.After(wait)
		} else {
			a.warn(a.saveOutline())
		}
	}
	if wait := checkEvery - time.Since(w.checked); wait > 0 {
		c.After(wait)
		return
	}
	w.checked = time.Now()
	c.After(checkEvery)
	a.checkDisk()
}

// checkDisk rereads the tree, which may have changed outside, and the
// text of the node shown.
func (a *App) checkDisk() {
	w := &a.write
	old, _ := w.tree.Node(w.node)
	if err := a.loadTree(); err != nil {
		a.warn(err)
		return
	}
	defer a.keepTabs()
	if w.node == "" || w.lost {
		return
	}
	n, ok := w.tree.Node(w.node)
	if !ok {
		w.lost, w.lostTitle = true, "找回的章节"
		if old != nil {
			w.lostTitle = old.Title
		}
		return
	}
	if n.Folder || w.external != nil {
		return
	}
	a.lookAtText()
}

// lookAtText reads the text of the chapter shown when the disk's is not
// the version last seen. Other words than those last saved wait in
// w.external for the author to choose between them and the editor's.
func (a *App) lookAtText() {
	w := &a.write
	st, err := a.Store.ChapterStamp(w.node)
	if err != nil {
		a.warn(err)
		return
	}
	if st.Size == w.stamp.Size && st.Mod.Equal(w.stamp.Mod) {
		return
	}
	ch, err := a.Store.ReadChapter(w.node)
	if err != nil {
		a.warn(err)
		return
	}
	if ch.Text == workspace.CleanProse(w.saved) {
		w.stamp = ch.Stamp
		return
	}
	w.external = &ch
}

// takeDisk replaces the text shown with the disk's, keeping the text
// shown as a snapshot.
func (a *App) takeDisk() {
	w := &a.write
	ch := w.external
	if ch == nil {
		return
	}
	// What the AI is writing or has proposed is about the text given up.
	a.stopAI()
	_, err := a.Store.TakeSnapshot(w.node, workspace.SnapExternal, w.ed.Doc.String(), storedMarks(w.ed.Doc.Marks()))
	if err != nil {
		a.warn(err)
		return
	}
	w.ed = a.chapterEditor(ch.Text, ch.Marks)
	w.saved, w.savedVer, w.stamp, w.external = ch.Text, w.ed.Doc.Version(), ch.Stamp, nil
	if w.counts != nil {
		w.counts[w.node] = workspace.Count(ch.Text)
	}
	a.notice = "已采用磁盘版本，编辑器原内容已存为快照"
}

// keepMine writes the text shown over the disk's, keeping the disk's as a
// snapshot.
func (a *App) keepMine() {
	w := &a.write
	ch := w.external
	if ch == nil {
		return
	}
	if _, err := a.Store.TakeSnapshot(w.node, workspace.SnapExternal, ch.Text, ch.Marks); err != nil {
		a.warn(err)
		return
	}
	w.external, w.savedVer, w.stamp = nil, -1, ch.Stamp
	a.warn(a.saveText())
	a.notice = "已保留编辑器版本，磁盘内容已存为快照"
}

// saveLost saves the text of a node gone from the disk as a new document.
func (a *App) saveLost() {
	w := &a.write
	id, err := a.Store.NewChapter("", w.lostTitle, false)
	if err == nil {
		_, err = a.Store.WriteChapter(id, w.ed.Doc.String(), storedMarks(w.ed.Doc.Marks()))
	}
	if err != nil {
		a.warn(err)
		return
	}
	// The new chapter takes the tab of the one that was lost.
	if i := slices.Index(w.tabs, w.node); i >= 0 {
		w.tabs[i] = id
	}
	w.ed, w.lost = nil, false
	a.warn(a.loadTree())
	a.openNode(id)
	a.notice = "已存为新章节，位于最上层末尾"
}

// writing is the writing page: the node shown between the chapter tree in
// the sidebar and the setting.
func (a *App) writing(c *ui.Context) {
	if !a.write.entered {
		a.write.entered = true
		a.enterWriting()
	}
	a.keepPen()
	a.pollAI()
	a.countAsTyped()
	a.feedNames()
	if a.write.ed != nil {
		a.write.ed.SetTypewriter(a.write.focus) // focused, the line written on stays mid-view
	}
	a.watch(c)
	a.checkWhilePausing()
	a.markDoubts()
	if c.Shortcut(ui.Cmd, ui.KeyF) && a.write.ed != nil {
		a.write.finding = true
	}
	// Escape backs out of the find bar first, then out of a pick in the
	// prose, so the bar over the pick goes with it.
	if ed := a.write.ed; ed != nil && c.Shortcut(0, ui.KeyEscape) {
		if s, e := ed.Selection(); a.write.finding {
			a.closeFind()
			ed.Focus()
		} else if s != e && ed.Focused() {
			ed.Select(ed.Caret(), ed.Caret())
		}
	}
	if a.aiOn() && c.Shortcut(ui.Cmd, ui.KeyL) {
		a.quoteToTalk()
	}
	if c.Shortcut(0, ui.KeyF11) {
		a.write.focus = !a.write.focus
	}
	if c.Shortcut(ui.Cmd, ui.KeyN) && a.write.renaming == "" {
		a.newNode(false)
	}
	if c.Shortcut(0, ui.KeyF2) && a.write.renaming == "" {
		a.renameOpen()
	}
	for key, d := range map[ui.Key]int{ui.KeyEqual: 1, ui.KeyMinus: -1, ui.Key0: 0} {
		if c.Shortcut(ui.Cmd, key) {
			a.sizeText(d)
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		a.saveNow()
	}
	if c.Shortcut(ui.Cmd, ui.KeyPageUp) {
		a.stepChapter(-1)
	}
	if c.Shortcut(ui.Cmd, ui.KeyPageDown) {
		a.stepChapter(1)
	}
	a.tabKeys(c)
	ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Stretch).Children(func() {
		main := ui.Column(c).Grow(1).Shrink(1).FillHeight()
		a.LastWrite = main.Bounds()
		main.Children(func() {
			a.writeTop(c)
			a.sheets(c)
			n, ok := a.write.tree.Node(a.write.node)
			switch {
			case a.write.lost && a.write.ed != nil:
				a.chapterView(c)
			case !ok && (a.write.tree.Root == nil || len(a.write.tree.Root.Children) == 0):
				a.emptyWriting(c)
				a.footBar(c, false)
			case !ok:
				a.pal.blank(c, icBook, "从左侧选择一章开始写作", "", nil)
				a.footBar(c, false)
			case n.Folder:
				a.folderView(c, n)
				a.footBar(c, false)
			default:
				a.chapterView(c)
			}
		})
		a.settingColumn(c)
	})
	if a.write.snapsOpen {
		a.snapshots(c)
	}
	if a.write.takeOpen {
		a.take(c)
	}
	if a.write.draftOpen {
		a.draftView(c)
	}
	a.askToSave(c)
}

// settingColumn is the setting beside the page, unless the author hid it.
func (a *App) settingColumn(c *ui.Context) {
	if a.doc.hidden || a.page == 3 && a.write.focus {
		return
	}
	win, _ := c.Size()
	width := a.panelWidth(win)
	ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
	panel := ui.Column(c).Width(width).FillHeight().Background(a.pal.side).Shrink(0)
	a.LastSetting = panel.Bounds()
	panel.Children(func() {
		if a.page == 3 && a.doc.fore {
			a.forePanel(c)
		} else if a.page == 3 && a.talkTab() {
			a.talkPanel(c)
		} else {
			a.settingPanel(c)
		}
		a.resizer(c, width)
	})
}

// paneTop is the top row of the right pane.
func paneTop(c *ui.Context) ui.Element {
	return ui.Row(c).FillWidth().Height(titleHeight).Padding(0, 20).Gap(4).AlignItems(ui.Center).Shrink(0)
}

// countAsTyped keeps the open chapter's count up with its text, so the
// counts move as the author types rather than when the text is saved.
func (a *App) countAsTyped() {
	w := &a.write
	if w.ed == nil || w.counts == nil || w.lost || w.external != nil {
		return
	}
	if w.ed.Doc == w.counted && w.ed.Doc.Version() == w.countedVer {
		return
	}
	w.counted, w.countedVer = w.ed.Doc, w.ed.Doc.Version()
	w.counts[w.node] = workspace.Count(w.ed.Doc.String())
}

func (a *App) total() int {
	n := 0
	for _, k := range a.write.counts {
		n += k
	}
	return n
}

// keepDays is how many days' counts are kept.
const keepDays = 60

// lastDays is what was written on each of the n days up to now, the last
// being today; a day the work was not open is 0.
func (a *App) lastDays(now time.Time, n int) []int {
	out := make([]int, n)
	k := a.today(now) // a new day keeps the last first
	for i := range n {
		day := now.AddDate(0, 0, i-n+1).Format(time.DateOnly)
		out[i] = a.write.days[day]
	}
	out[n-1] = k
	return out
}

// today is how much the book grew today, which the first look on a new
// day starts from. Taking chapters out counts against it.
func (a *App) today(now time.Time) int {
	w := &a.write
	if w.counts == nil {
		return 0 // not read yet: a start of nothing would count the whole book
	}
	if day := now.Format(time.DateOnly); day != w.day {
		if w.day != "" {
			// What the book has now is where the last day the work was
			// open ended.
			if w.days == nil {
				w.days = map[string]int{}
			}
			w.days[w.day] = a.total() - w.dayStart
			for len(w.days) > keepDays {
				delete(w.days, slices.Min(slices.Collect(maps.Keys(w.days))))
			}
		}
		w.day, w.dayStart = day, a.total()
		a.savePrefs()
	}
	return a.total() - w.dayStart
}

// stepChapter opens the chapter before (d < 0) or after the node open, in
// the tree's order, passing over folders; at either end it stays.
func (a *App) stepChapter(d int) {
	w := &a.write
	var order []string
	at, folder := -1, false
	w.tree.Walk(func(n *workspace.Node, _ int) {
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

// setGoal takes the day's goal as typed: a whole number of characters, or
// nothing for none. Anything else leaves the goal as it was.
func (a *App) setGoal(text string) {
	w := &a.write
	text = strings.TrimSpace(text)
	if text == "" {
		w.goal = 0
		a.savePrefs()
		return
	}
	if n, err := strconv.Atoi(text); err == nil && n >= 0 {
		w.goal = n
		a.savePrefs()
	}
}

// words says how many characters, the way the design writes them: grouped
// by thousands, and in 万 from ten thousand on.
func words(n int) string {
	if n >= 10000 {
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/10000), ".0") + " 万字"
	}
	return grouped(n) + " 字"
}

// grouped writes n with a comma between each three digits.
func grouped(n int) string {
	if n < 0 {
		return "-" + grouped(-n)
	}
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// signed is grouped with its sign.
func signed(n int) string {
	if n < 0 {
		return grouped(n)
	}
	return "+" + grouped(n)
}

func (a *App) writeTop(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	n, ok := w.tree.Node(w.node)
	top := ui.Row(c).FillWidth().Height(titleHeight).Padding(0, 16, 0, 24).Gap(8).AlignItems(ui.Center).Shrink(0)
	if w.focus {
		// No title bar over it: this row holds the window, and keeps clear
		// of the window's buttons.
		bar := c.TitleBar()
		top.Padding(0, 16+bar.Right, 0, 24+bar.Left).DragWindow()
	}
	top.Children(func() {
		ui.Row(c).Gap(2).AlignItems(ui.Center).Margin(0, 0, 0, -8).Shrink(0).Children(func() {
			if a.stepButton(c, icBack, "上一个打开的章节").Disabled(len(w.back) == 0).Clicked() {
				a.stepHistory(false)
			}
			if a.stepButton(c, icForward, "下一个打开的章节").Disabled(len(w.forward) == 0).Clicked() {
				a.stepHistory(true)
			}
		})
		if ok {
			var path []string
			for p := n.Parent; p != nil && p.Parent != nil; p = p.Parent {
				path = append([]string{p.Title}, path...)
			}
			for _, title := range path {
				ui.Text(c, title).FontSize(fsControl).TextColor(t.TextMuted).SingleLine().Shrink(1)
				glyph(c, icChevron, iconSm, a.pal.faint)
			}
			ui.Text(c, n.Title).FontSize(fsControl).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
		} else if w.lost {
			ui.Text(c, w.lostTitle).FontSize(fsControl).FontWeight(600)
		} else {
			ui.Text(c, "正文").FontSize(fsControl).FontWeight(600)
		}
		if ok && !n.Folder && w.ed != nil {
			if w.ed.Predicting() {
				ui.Text(c, "续写中…").FontSize(fsNote).TextColor(a.pal.faint).SingleLine().Shrink(0).Margin(0, 0, 0, 6)
			} else if w.ed.Ghost() != "" {
				ui.Text(c, "Tab 采纳 · Esc 放弃").FontSize(fsNote).TextColor(a.pal.faint).SingleLine().Shrink(0).Margin(0, 0, 0, 6)
			}
		}
		ui.Spacer(c)
		if ok && !n.Folder {
			busy := w.ed.ReadOnly() || w.lost || w.external != nil
			color := ui.Color{}
			if w.finding {
				color = a.pal.ink
			}
			if iconButton(c, icSearch, "查找替换", color).Tooltip("查找替换（" + modKey + "F）").Disabled(busy).Clicked() {
				if w.finding {
					a.closeFind()
				} else {
					w.finding = true
				}
			}
		}
		if a.doc.hidden && !w.focus {
			a.bookDot(c)
			if iconButton(c, icPanel, "展开右栏", ui.Color{}).Tooltip("展开右栏").Clicked() {
				a.setDocHidden(false)
			}
		}
		if w.focus {
			if iconButton(c, icShrink, "退出专注", a.pal.ink).Tooltip("退出专注，恢复章节树和右栏（F11）").Clicked() {
				w.focus = false
			}
		} else if iconButton(c, icExpand, "专注写作", ui.Color{}).Tooltip("仅显示正文，收起章节树和右栏（F11）").Clicked() {
			w.focus = true
		}
		a.backupChip(c)
		a.pendingButton(c)
	})
	a.pal.hairline(c)
}

// stepButton goes to the chapter opened before the one shown, or after it:
// smaller than the tools at the row's other end.
func (a *App) stepButton(c *ui.Context, s *ui.SVG, label string) ui.Element {
	t := a.pal.theme
	b := ui.ButtonBase(c).Size(iconLg, iconLg).Center().Radius(rControl).Shrink(0).Label(label).Tooltip(label)
	fg := t.TextMuted
	if b.Hovered() {
		b.Background(t.SurfacePressed)
		fg = t.Text
	}
	if b.IsDisabled() {
		pale(b)
	}
	b.Children(func() { glyph(c, s, iconSm, fg) })
	return b
}

// wordCount is at the right of the foot: the words of what is shown or
// selected, of the book, and of today against the goal. It opens the last
// seven days and the goal. roomy and wide say how much of it there is room
// for.
func (a *App) wordCount(c *ui.Context, roomy, wide bool) {
	t := a.pal.theme
	w := &a.write
	n, ok := w.tree.Node(w.node)
	here, unit := 0, "本章"
	if ok && n.Folder {
		unit = "本卷"
	}
	if ok {
		here = w.wordsIn(n)
	}
	if w.ed != nil {
		if sel := w.ed.SelectedText(); sel != "" {
			here, unit = workspace.Count(sel), "选中"
		}
	}
	today, tip := "", "全书 "+words(a.total())
	k := a.today(time.Now())
	switch {
	case w.goal > 0:
		today = fmt.Sprintf(" · 今天 %s / %s", signed(k), grouped(w.goal))
		tip += "，今天 " + signed(k) + " 字，目标 " + words(w.goal)
	case k != 0:
		today = " · 今天 " + signed(k)
		tip += "，今天 " + signed(k) + " 字"
	}
	if !wide {
		today = ""
	}
	color := a.pal.faint // as the design has it, met or not: the numbers say so
	// Whole, as in the prototype: the check line beside it gives way.
	counts := ui.ButtonBase(c).Row().Shrink(0).Label("字数").Tooltip(tip + "。点击设置每日目标")
	if counts.Hovered() {
		color = t.Text
	}
	counts.Children(func() {
		if ok && !roomy && unit == "选中" {
			ui.Text(c, "选中 "+words(here)).FontSize(fsNote).TextColor(color).SingleLine()
		} else if ok && !roomy {
			ui.Text(c, words(here)).FontSize(fsNote).TextColor(color).SingleLine()
		} else if ok {
			ui.Text(c, unit+" "+words(here)+" · 全书 "+words(a.total())+today).FontSize(fsNote).TextColor(color).SingleLine()
		} else {
			ui.Text(c, "全书 "+words(a.total())+today).FontSize(fsNote).TextColor(color).SingleLine()
		}
	})
	if counts.Clicked() {
		w.goalOpen = !w.goalOpen
	}
	// It opens upward: the foot is the last line of the window.
	ui.PopoverBase(c, counts, &w.goalOpen, func(panel ui.Element) {
		panel.AttachTo(counts, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 4, 0).Width(250).Radius(rCard).Padding(8).Gap(2).
			Background(a.pal.raised).Border(1, t.Border)
		a.pal.pop.on(panel)
		a.lastWeek(c)
		ui.Box(c).FillWidth().Height(1).Margin(4, 0).Background(a.pal.line).Shrink(0)
		ui.Column(c).FillWidth().Gap(6).Padding(4, 6).Children(func() {
			ui.Text(c, "每日目标").FontSize(fsNote).TextColor(t.TextMuted)
			in := ui.TextInput(c, &w.goalText).Label("每日目标字数").Placeholder("比如 2000").FillWidth().AutoFocus()
			if in.Changed() {
				a.setGoal(w.goalText)
			}
			if in.Submitted() {
				w.goalOpen = false
			}
		})
		ui.Text(c, "设置后，字数旁显示今日字数和目标。留空则不设。").FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody).FillWidth().Padding(0, 6, 4, 6)
	})
}

// lastWeek lists how much was written on each of the last seven days, today
// first.
func (a *App) lastWeek(c *ui.Context) {
	now := time.Now()
	days := a.lastDays(now, 7)
	ui.Text(c, "最近 7 天").FontSize(fsSmall).TextColor(a.pal.faint).Padding(6, 10, 2, 10)
	for i := len(days) - 1; i >= 0; i-- {
		day, k := now.AddDate(0, 0, i-len(days)+1), days[i]
		got := "0 字"
		if k != 0 {
			got = signed(k) + " 字"
		}
		ui.Row(c.Key(i)).FillWidth().Padding(4, 10).Label(fmt.Sprintf("%s 写了 %d 字", day.Format("1月2日"), k)).Children(func() {
			ui.Text(c, day.Format("1月2日")).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).Grow(1)
			ui.Text(c, got).FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
		})
	}
}

func (a *App) emptyWriting(c *ui.Context) {
	a.pal.blank(c, icBook, "暂无章节", "每章是作品文件夹 chapters 中的一个纯文本文件，可用其他软件打开。", func() {
		ui.Row(c).Gap(8).Padding(10, 0, 0, 0).Justify(ui.End).Children(func() {
			if a.pal.secondary(c, "新建文件夹").Clicked() {
				a.newNode(true)
			}
			if a.pal.primary(c, "新建第一章").Clicked() {
				a.newNode(false)
			}
		})
	})
}

// outlineBox is the outline of the node shown: 章纲 of a document, 卷纲 of
// a folder.
func (a *App) outlineBox(c *ui.Context, label string, height float32) {
	w := &a.write
	ui.TextAreaBase(c, &w.outline).Label(label).Placeholder("写下这里要发生的事，作为提纲").FillWidth().Height(height).
		FontSize(fsBody).LineHeight(lhLoose).Padding(10, 12).Radius(rControl).Background(a.pal.side).Disabled(w.lost)
}

func (a *App) chapterView(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	if w.external != nil {
		a.notice2(c, t.Warning, "本章文件已被其他程序修改。请选择保留的版本，另一版本将存为快照。", func() {
			if action(c, "使用磁盘版本", t.Warning, false).Clicked() {
				a.takeDisk()
			}
			if action(c, "保留当前版本", t.Warning, true).Clicked() {
				a.keepMine()
			}
		})
	}
	if w.lost {
		a.notice2(c, t.Danger, "磁盘上已找不到本章文件：在软件外改名、移动或删除，均视为删除。编辑器中的内容仍保留。", func() {
			if action(c, "存为新章节", t.Danger, true).Clicked() {
				a.saveLost()
			}
		})
	}
	// As wide as the prose at most, and over it, when the page is wide: the
	// chapter's title, then its outline and summary side by side, each
	// opening under the two.
	ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(24, 24, 6, 24).Shrink(0).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(editor.MaxTextWidth).Gap(8).Children(func() {
			ui.Text(c, heading(a.shownTitle())).FontSize(fsTitle).FontWeight(700).SingleLine().Ellipsis("…")
			label := "章纲"
			if strings.TrimSpace(w.outline) == "" {
				label = "章纲 · 空"
			}
			sum := a.summaryShown()
			var outline, summary ui.CollapsibleParts
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				outline = a.disclosure(c, label, &w.outlineOpen)
				if sum {
					label, _, warn := a.summaryState()
					summary = a.disclosure(c, label, &w.summaryOpen)
					if warn != "" {
						summary.Trigger.Tooltip(warn)
					}
				}
			})
			outline.Panel(func() {
				ui.Box(c).FillWidth().Padding(2, 0, 4, 0).Children(func() { a.outlineBox(c, "章纲", 96) })
			})
			if sum {
				summary.Panel(func() { a.summaryPanel(c) })
			}
		})
	})
	if w.finding && !w.ed.ReadOnly() {
		a.findBar(c)
	}
	a.aiBar(c)
	// Keyed, as the bars over it come and go: the prose keeps the keyboard.
	ui.Box(c.Key("prose")).Grow(1).FillWidth().Children(func() {
		w.ed.View(c).Fill()
		a.selectionBar(c)
		a.forePopover(c)
		a.menuAt(c, "正文", a.textMenu)
	})
	a.footBar(c, true)
}

// shownTitle is the title of the chapter shown.
func (a *App) shownTitle() string {
	w := &a.write
	if n, ok := w.tree.Node(w.node); ok {
		return n.Title
	}
	if w.lost {
		return w.lostTitle
	}
	return "正文"
}

// heading is a chapter's title over its prose: the name after 第几章, which
// the breadcrumb above already says.
func heading(title string) string {
	if _, name, ok := strings.Cut(title, "："); ok && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return title
}

// disclosure is a quiet toggle over the prose, its panel built apart from it
// so that several sit on one row.
func (a *App) disclosure(c *ui.Context, label string, open *bool) ui.CollapsibleParts {
	t := a.pal.theme
	p := ui.CollapsibleBase(c, open)
	color := t.TextMuted
	if p.Trigger.Hovered() || *open {
		color = t.Text
	}
	arrow := " ▾"
	if *open {
		arrow = " ▴"
	}
	p.Trigger.Label(label).Padding(2, 0).Children(func() {
		ui.Text(c, label+arrow).FontSize(fsNote).TextColor(color).SingleLine()
	})
	return p
}

// notice2 is a banner over the editor that asks the author something.
func (a *App) notice2(c *ui.Context, color ui.Color, text string, buttons func()) {
	t := a.pal.theme
	ui.Row(c).FillWidth().Padding(10, 20, 0, 20).Shrink(0).Children(func() {
		ui.Row(c).FillWidth().Padding(8, 8, 8, 12).Gap(10).Radius(rCard).AlignItems(ui.Center).Background(color.Alpha(0.08)).Border(1, color.Alpha(0.2)).Children(func() {
			glyph(c, icAlert, iconMd, color)
			ui.Text(c, text).FontSize(fsControl).TextColor(color.Mix(t.Text, 0.35)).Grow(1).Shrink(1)
			buttons()
		})
	})
}

// closeFind puts the find bar away, and what it found is no longer marked
// in the prose.
func (a *App) closeFind() {
	w := &a.write
	w.finding, w.wide, w.texts, w.sure = false, false, nil, false
	if w.ed != nil {
		w.ed.SetFound(nil, 0)
	}
}

func (a *App) findBar(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	found := w.ed.Doc.Find(w.query)
	if w.current >= len(found) {
		w.current = 0
	}
	w.ed.SetFound(found, w.current)
	show := func() {
		if len(found) > 0 {
			w.ed.Select(found[w.current][0], found[w.current][1])
		}
	}
	// Finding on one line and replacing on the next, so neither is
	// squeezed in a narrow window.
	bar := ui.Column(c).FillWidth().Padding(8, 16, 4, 24).Gap(6).Shrink(0)
	none := len(found) == 0
	query := func() {
		q := ui.TextInput(c, &w.query).Label("查找").Placeholder("查找").Width(150).Shrink(1).Grow(1).AutoFocus()
		if q.Submitted() && !none {
			w.current = (w.current + 1) % len(found)
			show()
		}
	}
	with := func() {
		ui.TextInput(c, &w.with).Label("替换为").Placeholder("替换为").Width(130).Shrink(1).Grow(1)
	}
	count := func() {
		switch {
		case w.query == "":
		case none:
			ui.Text(c, "未找到").FontSize(fsNote).TextColor(a.pal.faint).Shrink(0)
		default:
			ui.Textf(c, "%d / %d", w.current+1, len(found)).FontSize(fsNote).TextColor(t.TextMuted).Shrink(0)
		}
	}
	move := func() {
		if iconButton(c, icChevronUp, "上一个", ui.Color{}).Disabled(none).Clicked() {
			w.current = (w.current + len(found) - 1) % len(found)
			show()
		}
		if iconButton(c, icChevronDown, "下一个", ui.Color{}).Disabled(none).Clicked() {
			w.current = (w.current + 1) % len(found)
			show()
		}
	}
	replace := func() {
		if a.pal.secondary(c, "替换").Disabled(none || w.lost).Clicked() {
			if s, e := w.ed.Selection(); s == found[w.current][0] && e == found[w.current][1] {
				w.ed.Replace(s, e, w.with)
				if found = w.ed.Doc.Find(w.query); len(found) > 0 {
					w.current %= len(found)
					show()
				}
			} else {
				show()
			}
		}
		if w.wide {
			if a.pal.secondary(c, "全书替换").Disabled(w.query == "" || len(w.hits) == 0 || w.lost || w.external != nil).Clicked() {
				w.sure = !w.sure
			}
			return
		}
		if a.pal.secondary(c, "全部替换").Disabled(none || w.lost).Clicked() {
			n := w.ed.ReplaceAll(w.query, w.with)
			a.notice = fmt.Sprintf("已替换 %d 处，可撤销", n)
		}
	}
	wide := func() {
		if a.pal.chip(c, "全书", w.wide).Tooltip("在全部章节中查找").Clicked() {
			w.wide, w.texts, w.sure = !w.wide, nil, false
		}
	}
	closer := func() {
		if iconButton(c, icX, "关闭查找", ui.Color{}).Clicked() {
			a.closeFind()
		}
	}
	line := func(fn func()) { ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(fn) }
	bar.Children(func() {
		line(func() { query(); count(); move(); wide(); closer() })
		line(func() { with(); replace() })
		if w.wide && w.sure {
			a.confirmBookReplace(c)
		}
		if w.wide {
			a.bookResults(c)
		}
	})
}

// folderView shows a folder: its outline and what it holds.
func (a *App) folderView(c *ui.Context, n *workspace.Node) {
	t := a.pal.theme
	w := &a.write
	ui.Scroll(c.Key("folder")).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(32, 32, 32, 32).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(720).Gap(14).Children(func() {
				ui.Text(c, n.Title).FontSize(fsTitle).FontWeight(600)
				ui.Text(c, "卷纲").FontSize(fsControl).FontWeight(600).TextColor(t.TextMuted)
				a.outlineBox(c, "卷纲", 160)
				ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(8, 0, 0, 0).Children(func() {
					ui.Textf(c, "%d 项", len(n.Children)).FontSize(fsControl).FontWeight(600).TextColor(t.TextMuted).Grow(1)
					if a.pal.ghost(c, "在此新建章节", ui.Color{}).Clicked() {
						a.newNode(false)
					}
				})
				for _, child := range n.Children {
					row := ui.ButtonBase(c.Key("f-"+child.ID)).Column().FillWidth().Gap(4).Padding(10, 12).Radius(rControl).Label("打开-" + child.Title)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					row.Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
							glyph(c, nodeIcon(child), iconMd, t.TextMuted)
							ui.Text(c, child.Title).FontSize(fsBody).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
							if !child.Folder {
								ui.Textf(c, "%d 字", w.counts[child.ID]).FontSize(fsNote).TextColor(a.pal.faint)
							}
						})
						if gist := w.gists[child.ID]; gist != "" {
							ui.Text(c, gist).FontSize(fsNote).TextColor(t.TextMuted).FillWidth().Padding(0, 0, 0, 16+10).MaxLines(2).Ellipsis("…") // under the title, past the icon
						}
					})
					if row.Clicked() {
						a.openNode(child.ID)
					}
				}
			})
		})
	})
}

func nodeIcon(n *workspace.Node) *ui.SVG {
	if n.Folder {
		return icFolder
	}
	return icDoc
}

// chapterTree is the sidebar of the writing page.
func (a *App) chapterTree(c *ui.Context) {
	w := &a.write
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 4, 6, 10).Gap(2).Children(func() {
		ui.Text(c, "章节").FontSize(fsNote).TextColor(a.pal.theme.TextMuted).Grow(1)
		if iconButton(c, icFolderPlus, "新建文件夹", ui.Color{}).Clicked() {
			a.newNode(true)
		}
		if iconButton(c, icPlus, "新建章节", ui.Color{}).Tooltip("新建章节（" + modKey + "N）").Clicked() {
			a.newNode(false)
		}
	})
	ui.Scroll(c.Key("chapters")).FillWidth().Grow(1).Gap(2).Children(func() {
		if w.tree.Root == nil {
			return
		}
		var rows func(ns []*workspace.Node, depth int)
		rows = func(ns []*workspace.Node, depth int) {
			for _, n := range ns {
				a.treeRow(c, n, depth)
				if n.Folder && !w.folded[n.ID] {
					rows(n.Children, depth+1)
				}
			}
		}
		rows(w.tree.Root.Children, 0)
		// Dropping below the rows puts a node at the end of the top level.
		end := ui.Box(c).FillWidth().Height(40)
		if v, ok := ui.Drop[dragNode](end); ok {
			a.warn(a.Store.MoveChapter(string(v), "", len(w.tree.Root.Children)))
			a.warn(a.loadTree())
		}
	})
}

// trashRow opens the trash.
func (a *App) trashRow(c *ui.Context) {
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().Grow(1).Shrink(1).Justify(ui.Start).AlignItems(ui.Center).Gap(10).Padding(8, 10).Radius(rControl).Label("回收站").KeepFocus()
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		glyph(c, icTrash, iconMd, t.TextMuted)
		ui.Text(c, "回收站").FontSize(fsControl).TextColor(t.TextMuted)
	})
	if b.Clicked() {
		a.openTrash()
	}
}

func (a *App) treeRow(c *ui.Context, n *workspace.Node, depth int) {
	t := a.pal.theme
	w := &a.write
	on := n.ID == w.node
	if w.renaming == n.ID {
		// The row stays a row, its icon and indent where they are, and the
		// name is a box to type in. Enter settles it, and so does leaving the
		// box; Esc leaves the name as it was.
		row := ui.Row(c.Key("r-"+n.ID)).FillWidth().Height(hRow).AlignItems(ui.Center).Padding(0, 10, 0, 10+float32(depth)*20).Radius(rControl)
		if on {
			row.Background(t.SurfacePressed)
		}
		row.Children(func() {
			if n.Folder {
				s := icChevronDown
				if w.folded[n.ID] {
					s = icChevron
				}
				glyph(c, s, iconXs, t.Text)
			} else {
				glyph(c, nodeIcon(n), iconSm, t.Text)
			}
			// The box gives the row's gap of 6 back as its border and padding,
			// so the words in it stand where the names of the other rows do.
			ui.Row(c).Grow(1).Shrink(1).Height(hRow-6).AlignItems(ui.Center).Padding(0, 6-1).Radius(rXs).Background(a.pal.canvas).Border(1, a.pal.ink).Children(func() {
				in := ui.TextInputBase(c, &w.title).Label("章节标题").Placeholder(w.titleWas).Grow(1).Shrink(1).FontSize(fsControl).AutoFocus()
				if w.titleAll {
					w.titleAll = false
					in.SetTextSelection(0, utf8.RuneCountInString(w.title))
				}
				switch {
				case in.Shortcut(0, ui.KeyEscape):
					w.renaming, w.titleHeld = "", false
				case in.Submitted():
					// Named, the open chapter is written: the caret goes to the prose.
					if a.rename(n.ID); on && w.ed != nil {
						w.ed.Focus()
					}
				case w.titleHeld && !in.Focused():
					a.rename(n.ID)
				case in.Focused():
					w.titleHeld = true
				}
			})
		})
		return
	}
	row := ui.ButtonBase(c.Key("n-"+n.ID)).Row().FillWidth().Height(hRow).Justify(ui.Start).AlignItems(ui.Center).Gap(6).
		Padding(0, 10, 0, 10+float32(depth)*20).Radius(rControl).Label("章节-" + n.Title)
	row.Drag(dragNode(n.ID))
	_, over := ui.DragOver[dragNode](row)
	switch {
	case over:
		row.Background(a.pal.ink.Alpha(0.12))
	case on:
		row.Background(t.SurfacePressed)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	if v, ok := ui.Drop[dragNode](row); ok {
		a.dropOn(string(v), n)
	}
	row.Children(func() {
		// The top level reads darker than what is inside a folder.
		fg, weight := t.Text, 400
		if depth > 0 {
			fg = t.TextMuted
		}
		if on {
			fg, weight = t.Text, 600
		}
		if n.Folder {
			// A folder is told by its arrow, which also says it is open.
			s := icChevronDown
			if w.folded[n.ID] {
				s = icChevron
			}
			glyph(c, s, iconXs, fg)
		} else {
			glyph(c, nodeIcon(n), iconSm, fg)
		}
		ui.Text(c, n.Title).FontSize(fsControl).FontWeight(weight).TextColor(fg).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		if k := w.wordsIn(n); k > 0 {
			ui.Text(c, strings.ReplaceAll(strings.TrimSuffix(words(k), "字"), " ", "")).FontSize(fsSmall).TextColor(a.pal.faint)
		}
	})
	a.contextMenu(c, row, "章节-"+n.ID, func(m *menu) { a.nodeMenu(m, n) })
	if row.Clicked() {
		// A folder only folds: its outline opens from its menu. The tree
		// arranges, the tabs show.
		if n.Folder {
			w.folded[n.ID] = !w.folded[n.ID]
		} else {
			a.openNode(n.ID)
		}
	}
}

// renameOpen starts naming the node that is open, as F2 asks. Its row is
// brought out first: from a folded folder, and from focus mode.
func (a *App) renameOpen() {
	w := &a.write
	n, ok := w.tree.Node(w.node)
	if !ok {
		return
	}
	for p := n.Parent; p != nil; p = p.Parent {
		delete(w.folded, p.ID)
	}
	w.focus = false
	a.showTree()
	w.nameAgain(n)
}

// nameAgain starts naming n again: its name is in the box, selected.
func (w *writeView) nameAgain(n *workspace.Node) {
	w.renaming, w.title, w.titleWas, w.offered, w.titleAll = n.ID, n.Title, n.Title, "", true
}

// showTree brings the sidebar back when it was put away: a name is typed
// in the tree.
func (a *App) showTree() {
	if a.sideHidden {
		a.setSideHidden(false)
	}
}

func (a *App) nodeMenu(m *menu, n *workspace.Node) {
	w := &a.write
	siblings := n.Parent.Children
	i := slices.Index(siblings, n)
	if n.Folder {
		m.Item("编辑卷纲").Do(func() { a.openNode(n.ID) })
	}
	m.Item("重命名").Do(func() { w.nameAgain(n) })
	m.Item("新建章节").Do(func() { a.newNodeAt(false, n) })
	m.Item("新建文件夹").Do(func() { a.newNodeAt(true, n) })
	m.Separator()
	m.Item("上移").Disabled(i == 0).Do(func() { a.move(n.ID, n.Parent.ID, i-1) })
	m.Item("下移").Disabled(i == len(siblings)-1).Do(func() { a.move(n.ID, n.Parent.ID, i+1) })
	// Where else it can go: not where it is, nor inside itself.
	var into []*workspace.Node
	var depths []int
	w.tree.Walk(func(f *workspace.Node, depth int) {
		if f.Folder && f != n.Parent && !f.Within(n) {
			into, depths = append(into, f), append(depths, depth)
		}
	})
	if top := n.Parent.Parent != nil; top || len(into) > 0 {
		m.Head("移到")
		if top {
			m.Item("最上层").Do(func() { a.move(n.ID, "", len(w.tree.Root.Children)) })
		}
		for i, f := range into {
			m.Item(strings.Repeat("　", depths[i]) + f.Title).Do(func() { a.move(n.ID, f.ID, len(f.Children)) })
		}
	}
	m.Separator()
	m.Item("移到回收站").Do(func() { a.trashNode(n) })
}

func (a *App) move(id, parent string, index int) {
	a.flush()
	a.warn(a.Store.MoveChapter(id, parent, index))
	a.warn(a.loadTree())
}

// dropOn moves node id into folder target, or before document target.
func (a *App) dropOn(id string, target *workspace.Node) {
	if id == target.ID {
		return
	}
	if target.Folder {
		a.move(id, target.ID, len(target.Children))
		w := &a.write
		w.folded[target.ID] = false
		return
	}
	var rest []*workspace.Node
	for _, s := range target.Parent.Children {
		if s.ID != id {
			rest = append(rest, s)
		}
	}
	a.move(id, target.Parent.ID, slices.Index(rest, target))
}

func (a *App) rename(id string) {
	w := &a.write
	a.flush()
	if trim(w.title) == "" {
		w.title = w.titleWas
	}
	// A whole numbered title typed after the number offered replaces it.
	if rest, ok := strings.CutPrefix(w.title, w.offered); ok && w.offered != "" && numbered.MatchString(trim(rest)) {
		w.title = rest
	}
	w.offered, w.titleHeld = "", false
	err := a.Store.RenameChapter(id, trim(w.title))
	if err == nil {
		w.renaming = ""
		err = a.loadTree()
	}
	a.warn(err)
}

// newNode adds a document, or a folder, beside the node shown, or inside
// it when it is a folder, and has the author name it.
func (a *App) newNode(folder bool) {
	n, _ := a.write.tree.Node(a.write.node)
	a.newNodeAt(folder, n)
}

// newNodeAt adds it after the document n, at the end of the folder n, or at
// the end of the book without one.
func (a *App) newNodeAt(folder bool, n *workspace.Node) {
	w := &a.write
	parent, index := "", -1
	if n != nil {
		if n.Folder {
			parent = n.ID
		} else {
			parent = n.Parent.ID
			index = slices.Index(n.Parent.Children, n) + 1
		}
	}
	title := "新章节"
	if folder {
		title = "新文件夹"
	}
	id, err := a.Store.NewChapter(parent, title, folder)
	if err == nil && index >= 0 {
		err = a.Store.MoveChapter(id, parent, index)
	}
	if err == nil {
		err = a.loadTree()
	}
	// Numbered like the one before, the title is ready to go on with its
	// name: 第四章 名单 gives 第五章.
	typed := ""
	if next := titleAfter(w.tree, id); err == nil && next != "" {
		title, typed = next, next+" "
		if err = a.Store.RenameChapter(id, title); err == nil {
			err = a.loadTree()
		}
	}
	if err != nil {
		a.warn(err)
		return
	}
	if parent != "" {
		w.folded[parent] = false
	}
	if w.counts == nil {
		w.counts = map[string]int{}
	}
	a.openNode(id)
	w.focus = false
	a.showTree()
	w.renaming, w.title, w.titleWas, w.offered = id, typed, title, typed
	if typed == "" { // 新章节, 新文件夹: in the box selected, to be typed over
		w.title, w.titleAll = title, true
	}
	// The name comes first: the caret is in its box, not in the empty prose.
	if w.ed != nil {
		w.ed.Unfocus()
	}
}

// wordsIn is a chapter's count, or for a folder all its chapters' together.
func (w *writeView) wordsIn(n *workspace.Node) int {
	if !n.Folder {
		return w.counts[n.ID]
	}
	k := 0
	for _, ch := range n.Children {
		k += w.wordsIn(ch)
	}
	return k
}

func (a *App) trashNode(n *workspace.Node) {
	w := &a.write
	a.flush()
	// Where the node shown stands among the chapters, to open the one that
	// takes its place if it goes and no tab is left.
	var order []string
	w.tree.Walk(func(d *workspace.Node, _ int) {
		if !d.Folder {
			order = append(order, d.ID)
		}
	})
	cur, shown := w.tree.Node(w.node)
	gone := shown && cur.Within(n)
	at := slices.Index(w.tabs, w.node)
	if err := a.Store.TrashChapter(n.ID); err != nil {
		a.warn(err)
		return
	}
	a.warn(a.loadTree())
	counts, err := a.Store.Counts()
	a.warn(err)
	w.counts = counts
	if gone {
		// Its tab goes with it: the tab beside it is shown, or with none
		// left the chapter that stands where it stood.
		near := slices.Index(order, w.node)
		w.node, w.ed, w.external, w.lost = "", nil, nil, false
		a.keepTabs()
		next := ""
		if len(w.tabs) > 0 {
			next = w.tabs[min(max(at, 0), len(w.tabs)-1)]
		} else if near >= 0 {
			var left []string
			before := 0
			for i, id := range order {
				if _, ok := w.tree.Node(id); ok {
					left = append(left, id)
					if i < near {
						before++
					}
				}
			}
			if len(left) > 0 {
				next = left[min(before, len(left)-1)]
			}
		}
		w.stepping = true // the one gone is not gone back to
		a.openNode(next)
		w.stepping = false
	} else {
		a.keepTabs()
	}
	a.notice = fmt.Sprintf("「%s」已移到回收站", n.Title)
}

func (a *App) openTrash() {
	w := &a.write
	a.flush()
	items, err := a.Store.Trash()
	a.warn(err)
	w.trash, w.trashOpen, w.purge = items, true, ""
}

func (a *App) trashView(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	a.modal(c, &w.trashOpen, func() {
		ui.Column(c).Width(dialogSmall).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "回收站", "已删除的章节及其内容暂存于此，恢复后回到原文件夹末尾。")
			ui.Scroll(c.Key("trash")).FillWidth().MaxHeight(320).Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					a.pal.hairline(c)
					if len(w.trash) == 0 {
						ui.Text(c, "回收站为空").FontSize(fsNote).TextColor(t.TextMuted).Padding(20, 0)
					}
					for _, it := range w.trash {
						ui.Row(c.Key("t-"+it.ID)).FillWidth().AlignItems(ui.Center).Gap(10).Padding(10, 0).Children(func() {
							ui.Column(c).Grow(1).Shrink(1).Children(func() {
								ui.Text(c, it.Title).FontSize(fsBody).LineHeight(lhBody).SingleLine().Ellipsis("…")
								ui.Text(c, it.At.Local().Format("01-02 15:04")+" 删除").FontSize(fsNote).LineHeight(lhBody).TextColor(t.TextMuted)
							})
							// Deleting for good is asked twice, in the row itself.
							if w.purge == it.ID {
								if a.pal.secondary(c, "取消").Label("取消彻底删除").Clicked() {
									w.purge = ""
								}
								if a.pal.danger(c, "确认彻底删除").Clicked() {
									a.warn(a.Store.PurgeChapter(it.ID))
									a.openTrash()
								}
								return
							}
							if a.pal.secondary(c, "恢复").Label("恢复-" + it.Title).Clicked() {
								a.restore(it.ID, it.Title)
							}
							if a.pal.ghost(c, "彻底删除", ui.Color{}).Clicked() {
								w.purge = it.ID
							}
						})
						a.pal.hairline(c)
					}
				})
			})
			ui.Row(c).FillWidth().Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "关闭").Clicked() {
					w.trashOpen = false
				}
			})
		})
	})
}

func (a *App) restore(id, title string) {
	w := &a.write
	if err := a.Store.RestoreChapter(id); err != nil {
		a.warn(err)
		return
	}
	a.warn(a.loadTree())
	counts, err := a.Store.Counts()
	a.warn(err)
	w.counts = counts
	a.openTrash()
	a.notice = fmt.Sprintf("「%s」已恢复", title)
}

// snapReasons say why a snapshot was taken; with the AI off, snapReason
// tells one taken before the AI changed the prose as one before a change.
var snapReasons = map[string]string{
	workspace.SnapOpen: "打开时", workspace.SnapInterval: "写作中", workspace.SnapRestore: "恢复前",
	workspace.SnapExternal: "外部改动", workspace.SnapAI: "AI 改动前", workspace.SnapReplace: "全书替换前",
}

func (a *App) snapReason(reason string) string {
	if reason == workspace.SnapAI && !a.aiOn() {
		return "改动前"
	}
	return snapReasons[reason]
}

func (a *App) openSnapshots() {
	w := &a.write
	a.warn(a.saveText())
	snaps, err := a.Store.Snapshots(w.node)
	if err != nil {
		a.warn(err)
		return
	}
	w.snaps, w.snapsOpen, w.compare = snaps, true, true
	w.snap, w.snapText = "", ""
	if len(snaps) > 0 {
		a.pickSnapshot(snaps[0].Name)
	}
}

func (a *App) pickSnapshot(name string) {
	w := &a.write
	text, err := a.Store.SnapshotText(w.node, name)
	if err != nil {
		a.warn(err)
		return
	}
	w.snap, w.snapText = name, text
}

// snapView is the snapshot chosen as its dialog shows it: whether it is
// the text as it stands, and its paragraphs, with what differs marked when
// the two are compared.
type snapView struct {
	key   [5]any // the snapshot, comparing or not, the editor's text, the two colours
	same  bool
	paras [][]ui.Span
	long  bool         // more than snapWhole: only the paragraphs in view are built
	list  ui.ListState // where a long one is scrolled to
}

// snapWhole is the most a snapshot shows with every paragraph built, in
// bytes of text and in paragraphs: so its text selects from end to end. A
// longer one builds the paragraphs in view, and selects among those: built
// whole, a frame of it took 47 ms at sixty thousand characters, and at
// three hundred thousand the toolkit no longer kept the paragraphs' lines.
const snapWhole, snapWholeParas = 90_000, 1000

// snapView is made as the snapshot, the comparing or the text beside it
// changes, not in each frame: comparing two chapters takes long. A
// paragraph is an element of its own, for the toolkit keeps the lines of
// a text only up to a size, and lays a longer one out again in every frame.
func (a *App) snapView() *snapView {
	t := a.pal.theme
	w := &a.write
	v := &w.snapShown
	key := [5]any{w.snap, w.compare, w.ed, w.ed.Doc.Version(), [2]ui.Color{t.Danger, t.Success}}
	if v.key == key {
		return v
	}
	now := w.ed.Doc.String()
	*v = snapView{key: key, same: w.snap != "" && w.snapText == now}
	if w.snap == "" {
		return v
	}
	pieces := []piece{{same, w.snapText}}
	if w.compare {
		pieces = diffText(w.snapText, now)
	}
	size := 0
	for _, p := range pieces {
		size += len(p.text)
	}
	paras := paragraphs(pieces)
	v.long = size > snapWhole || len(paras) > snapWholeParas
	for _, para := range paras {
		spans := make([]ui.Span, len(para))
		for i, p := range para {
			switch p.kind {
			case gone:
				spans[i] = ui.Span{Text: p.text, Color: t.Danger, Strikethrough: true, Background: t.Danger.Alpha(0.12)}
			case came:
				spans[i] = ui.Span{Text: p.text, Color: t.Success, Background: t.Success.Alpha(0.12)}
			default:
				spans[i] = ui.Span{Text: p.text}
			}
		}
		v.paras = append(v.paras, spans)
	}
	return v
}

func (a *App) snapshots(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	a.modal(c, &w.snapsOpen, func() {
		shown := a.snapView()
		same := shown.same // nothing to restore
		ui.Column(c).Width(dialogWide).Height(dialogTall).Padding(dialogPad).Gap(16).Children(func() {
			when := "打开章节时、连续写作每十分钟、恢复前、AI 起草或采纳改写前、全书替换前都会保存一份。最近七天的全部保留，更早的每天保留最后一份。"
			if !a.aiOn() {
				when = strings.Replace(when, "AI 起草或采纳改写前、", "", 1)
			}
			a.pal.dialogHead(c, "快照 · "+a.shownTitle(), when)
			ui.Row(c).FillWidth().Grow(1).Gap(16).AlignItems(ui.Stretch).Children(func() {
				ui.Scroll(c.Key("snaps")).Width(200).FillHeight().Shrink(0).Children(func() {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						if len(w.snaps) == 0 {
							ui.Text(c, "本章暂无快照").FontSize(fsControl).TextColor(a.pal.faint).Padding(8)
						}
						for _, sn := range w.snaps {
							on := sn.Name == w.snap
							when := sn.At.Local().Format("01-02 15:04:05")
							b := ui.ButtonBase(c.Key("s-"+sn.Name)).Column().FillWidth().AlignItems(ui.Start).Gap(2).Padding(8, 10).Radius(rControl).Label("快照-" + when)
							if on {
								b.Background(t.SurfacePressed)
							} else if b.Hovered() {
								b.Background(t.SurfaceHover)
							}
							b.Children(func() {
								ui.Text(c, when).FontSize(fsControl)
								ui.Textf(c, "%s · %d 字", a.snapReason(sn.Reason), sn.Count).FontSize(fsNote).TextColor(t.TextMuted)
							})
							if b.Clicked() {
								a.pickSnapshot(sn.Name)
							}
						}
					})
				})
				ui.Column(c).Grow(1).Shrink(1).FillHeight().Gap(10).Children(func() {
					ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Checkbox(c, &w.compare, "与当前对比")
						if same {
							ui.Text(c, "该快照与当前正文相同").FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
						} else if w.compare {
							ui.Text(c, "红色删除线为快照中有而当前没有的内容，绿色为当前新增的内容").FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
						}
					})
					if i := slices.IndexFunc(w.snaps, func(s workspace.Snapshot) bool { return s.Name == w.snap }); i >= 0 && w.snaps[i].AI != nil && a.aiOn() {
						a.aiRecord(c, w.snaps[i].AI)
					}
					// The paragraphs select as one text, and copy a line each.
					para := func(i int) {
						switch spans := shown.paras[i]; {
						case len(spans) == 0: // an empty line keeps its height
							ui.Text(c, "").FontSize(fsRead).LineHeight(lhRead).FillWidth()
						case len(spans) == 1 && spans[0] == (ui.Span{Text: spans[0].Text}):
							ui.Text(c, spans[0].Text).FontSize(fsRead).LineHeight(lhRead).FillWidth()
						default:
							ui.RichText(c, spans...).FontSize(fsRead).LineHeight(lhRead).FillWidth()
						}
					}
					if shown.long {
						ui.Box(c).FillWidth().Grow(1).Background(a.pal.canvas).Border(1, a.pal.line).Radius(rControl).Selectable().Children(func() {
							ui.List(c, &shown.list, len(shown.paras), para).Fill().Padding(12, 16)
						})
					} else {
						ui.Scroll(c.Key("snap-text")).FillWidth().Grow(1).Background(a.pal.canvas).Border(1, a.pal.line).Radius(rControl).Children(func() {
							ui.Column(c).FillWidth().Padding(12, 16).Selectable().Children(func() {
								for i := range shown.paras {
									para(i)
								}
							})
						})
					}
				})
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "关闭").Clicked() {
					w.snapsOpen = false
				}
				if a.pal.primary(c, "恢复这个版本").Disabled(w.snap == "" || same || w.lost || w.external != nil).Clicked() {
					a.restoreSnapshot()
				}
			})
		})
	})
}

// restoreSnapshot replaces the text with the snapshot chosen, keeping the
// text replaced as a snapshot of its own.
func (a *App) restoreSnapshot() {
	w := &a.write
	i := slices.IndexFunc(w.snaps, func(s workspace.Snapshot) bool { return s.Name == w.snap })
	if i < 0 {
		return
	}
	if err := a.saveText(); err != nil {
		a.warn(err)
		return
	}
	if _, err := a.Store.TakeSnapshot(w.node, workspace.SnapRestore, w.ed.Doc.String(), storedMarks(w.ed.Doc.Marks())); err != nil {
		a.warn(err)
		return
	}
	w.ed = a.chapterEditor(w.snapText, w.snaps[i].Marks)
	w.savedVer = -1
	if err := a.saveText(); err != nil {
		a.warn(err)
		return
	}
	w.snapsOpen = false
	a.notice = "已恢复到 " + w.snaps[i].At.Local().Format("01-02 15:04") + " 的版本，恢复前的内容已存为快照"
}

// openTake starts taking the selected words into the setting.
func (a *App) openTake() {
	w := &a.write
	if w.ed == nil {
		return
	}
	q := strings.TrimSpace(w.ed.SelectedText())
	if q == "" {
		return
	}
	w.takeQuote, w.takeText, w.takeOpen = q, q, true
	if w.takeCategory == "" {
		w.takeCategory = a.firstCategory()
	}
}

func (a *App) take(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	a.modal(c, &w.takeOpen, func() {
		ui.Column(c).Width(dialogSmall).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "加入设定", "选中的原文将记为出处；设定内容可改用自己的表述。")
			labeled(c, "设定内容", func() {
				ui.TextAreaBase(c, &w.takeText).Label("设定内容").FillWidth().Height(88).FontSize(fsBody).LineHeight(lhLoose).
					Padding(8, 10).Radius(rControl).Border(1, t.Border)
			})
			labeled(c, "设定分类", func() {
				ui.Select(c, &w.takeCategory, a.categoryNames()).Label("设定分类").FillWidth()
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "取消").Clicked() {
					w.takeOpen = false
				}
				if a.pal.primary(c, "加入").Disabled(strings.TrimSpace(w.takeText) == "").Clicked() {
					a.takeIn()
				}
			})
		})
	})
}

// takeIn adds the author's entry quoting the chapter, saved first so the
// quote is in the text on disk.
func (a *App) takeIn() {
	w := &a.write
	if err := a.saveText(); err != nil {
		a.warn(err)
		return
	}
	r, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
		Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: w.takeCategory, Text: strings.TrimSpace(w.takeText),
		Sources: []workspace.Source{{Chapter: w.node, Quote: w.takeQuote}},
	}}})
	if err == nil {
		err = a.refresh()
	}
	if err != nil {
		a.warn(err)
		return
	}
	a.flash[r.Ops[0].Entry] = time.Now()
	w.takeOpen = false
	a.notice = "已加入设定正文"
}

func (a *App) exportView(c *ui.Context) {
	w := &a.write
	a.modal(c, &w.exportOpen, func() {
		ui.Column(c).Width(dialogSmall).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "导出正文", "按章节树顺序合并为一个 .txt 文件，各文件夹和章节前加上标题。")
			labeled(c, "导出范围", func() {
				parts := a.exportParts()
				folders := 0
				for _, n := range parts {
					if n.Folder {
						folders++
					}
				}
				ids, names := []string{""}, []string{"全书"}
				for _, n := range parts {
					name := "本章"
					if n.Folder {
						// Folders inside folders are told apart by their names.
						if name = "本卷"; folders > 1 {
							name = clip(n.Title, 8)
						}
					}
					ids, names = append(ids, n.ID), append(names, name)
				}
				w.exportFrom = ids[min(w.exportAt, len(ids)-1)]
				a.pal.segments(c, &w.exportAt, "导出范围", names).FillWidth()
			})
			ui.Checkbox(c, &w.indent, "段首加两个全角空格")
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "取消").Clicked() {
					w.exportOpen = false
				}
				if a.pal.primary(c, "导出").Clicked() {
					a.export(c)
				}
			})
		})
	})
}

// exportParts are what can be exported besides the book: the volume the
// open node is in, and the open node itself.
func (a *App) exportParts() []*workspace.Node {
	w := &a.write
	n, ok := w.tree.Node(w.node)
	if !ok || a.page != 3 {
		return nil // off the page of prose there is the book to export
	}
	var parts []*workspace.Node
	if p := n.Parent; p != nil && p.Parent != nil {
		parts = append(parts, p)
	}
	return append(parts, n)
}

// export writes the work, or the part chosen, as one text where the author
// picks, or into the work's 导出 folder when there is no dialog.
func (a *App) export(c *ui.Context) {
	a.flush()
	w := &a.write
	if _, ok := w.tree.Node(w.exportFrom); !ok {
		w.exportFrom = ""
	}
	text, err := a.Store.Export(w.exportFrom, w.indent)
	if err != nil {
		a.warn(err)
		return
	}
	name := a.Store.Work().Name + ".txt"
	if n, ok := w.tree.Node(w.exportFrom); ok {
		name = a.Store.Work().Name + "-" + n.Title + ".txt"
	}
	write := func(path string) {
		if path == "" {
			return
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			a.warn(err)
			return
		}
		a.write.exportOpen = false
		a.notice = "已导出到 " + path
	}
	if a.SaveFile == nil {
		dir := filepath.Join(a.Store.Root(), "导出")
		if err := os.MkdirAll(dir, 0755); err != nil {
			a.warn(err)
			return
		}
		write(filepath.Join(dir, name))
		return
	}
	save, sv := a.SaveFile, c.Services()
	go func() {
		path, err := save("导出", name)
		a.picks <- func() {
			if err != nil {
				a.warn(err)
				return
			}
			write(path)
		}
		sv.Invalidate()
	}()
}
