package desk

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/editor"
	"writer/workspace"
	"writer/writing"
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
	ed      *editor.Editor
	// What the disk has of the node shown, as of the last read or write.
	saved    string
	savedVer int
	stamp    workspace.Stamp
	lastSnap time.Time
	checked  time.Time
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
	offered               string         // the numbered title a new node's name began with
	days                  map[string]int // what was written each day before, by local date
	size                  int            // the text's size, or 0 for the default
	carets                map[string]int // where the caret was in each chapter, as it was left
	unsummarized          []string       // the chapters before without a summary, as a draft opens
	recapped              int            // and how many before have one
	// The disk's text, changed outside while the node was open; lost, the
	// node gone from the disk.
	external  *workspace.Chapter
	lost      bool
	lostTitle string

	renaming, title string
	titleWas        string // the name kept when the title is left empty
	focus           bool   // the page alone, without the tree and the column
	finding         bool
	query, with     string
	current         int
	wide            bool              // finding in the whole book
	sure            bool              // asking before replacing through the book
	texts           map[string]string // the chapters' text while finding in the book
	hits            []bookHit
	hitsFor         [3]any // the query, chapter and edit the hits were found for

	snapsOpen bool
	snaps     []workspace.Snapshot
	snap      string // the snapshot chosen
	snapText  string
	compare   bool

	trashOpen bool
	trash     []workspace.Trashed
	purge     string // the trashed node whose purge awaits confirmation

	takeOpen               bool
	takeQuote, takeText    string
	takeCategory           string
	exportOpen, exportDone bool
	indent                 bool
	exportFrom             string // the node exported with what is in it, or "" for the book

	// The writing model: the writer the editors predict with, and why
	// there is none; a request in flight; a rewrite asked for or shown.
	pen         atomic.Pointer[writing.Writer]
	penKey      string
	penErr      error
	autoPredict bool
	run         *aiRun
	rewriting   bool
	instruction string
	proposed    *writing.Job
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
}

func storedMarks(ms []editor.Mark) []workspace.ChapterMark {
	var out []workspace.ChapterMark
	for _, m := range ms {
		out = append(out, workspace.ChapterMark{Kind: markKinds[m.Kind], Start: m.Start, End: m.End, Note: m.Note, Orphaned: m.Orphaned})
	}
	return out
}

func editorMarks(ms []workspace.ChapterMark) []editor.Mark {
	var out []editor.Mark
	for _, m := range ms {
		for k, name := range markKinds {
			if name == m.Kind {
				out = append(out, editor.Mark{Kind: k, Start: m.Start, End: m.End, Note: m.Note, Orphaned: m.Orphaned})
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
	if _, ok := w.tree.Node(w.node); ok {
		if w.ed == nil && w.renaming == "" {
			a.openNode(w.node)
		}
		return
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
	var ed *editor.Editor
	ed = editor.New(doc, editor.Options{Prose: true, Placeholder: "从这里开始写…", Font: ui.Font{Size: float32(a.write.textSize())},
		Predictor: pen{writer: &a.write.pen, chapter: a.write.node}, PredictDelay: saveAfter, PredictOnDemand: !a.write.autoPredict,
		GhostNote: func() string {
			if pw := a.write.pen.Load(); pw != nil {
				return pw.Model.Name
			}
			return ""
		},
		Menu: func(m *ui.Menu) {
			s, e := ed.Selection()
			busy := ed.ReadOnly()
			if m.Item("续写一句").Shortcut(ui.Cmd, ui.KeyJ).Disabled(busy).Chosen() {
				a.continueText()
			}
			if m.Item("AI 改写…").Disabled(s == e || busy).Chosen() {
				a.openRewrite()
			}
			if m.Item("拿去讨论").Shortcut(ui.Cmd, ui.KeyL).Disabled(s == e).Chosen() {
				a.quoteToTalk()
			}
			if m.Item("加入设定").Disabled(s == e).Chosen() {
				a.openTake()
			}
		}})
	return ed
}

// openNode shows node id, saving what was shown first.
func (a *App) openNode(id string) {
	a.flush()
	a.leaveChapter()
	w := &a.write
	w.node, w.ed, w.external, w.lost = id, nil, nil, false
	w.outline, w.outlineSaved, w.outlinePrev = "", "", ""
	w.sum, w.summary, w.summarySaved, w.changed = workspace.Summary{}, "", "", false
	w.current = 0
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
	if !n.Folder {
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
	a.savePrefs()
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
		a.notice = "AI 还在改这一章，等它结束会自动保存"
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
	stamp, err := a.Store.WriteChapter(w.node, text, marks)
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
	a.notice = "已换成磁盘上的版本，编辑器里原来的内容存了快照"
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
	w.external, w.savedVer = nil, -1
	a.warn(a.saveText())
	a.notice = "已保留编辑器里的版本，磁盘上的内容存了快照"
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
	w.ed, w.lost = nil, false
	a.warn(a.loadTree())
	a.openNode(id)
	a.notice = "已存为新章节，放在最上层的末尾"
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
	a.watch(c)
	if c.Shortcut(ui.Cmd, ui.KeyF) && a.write.ed != nil {
		a.write.finding = true
	}
	if c.Shortcut(ui.Cmd, ui.KeyL) {
		a.quoteToTalk()
	}
	if c.Shortcut(0, ui.KeyF11) {
		a.write.focus = !a.write.focus
	}
	if c.Shortcut(ui.Cmd, ui.KeyN) && a.write.renaming == "" {
		a.newNode(false)
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
	ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Stretch).Children(func() {
		main := ui.Column(c).Grow(1).Shrink(1).FillHeight()
		a.LastWrite = main.Bounds()
		main.Children(func() {
			a.writeTop(c)
			n, ok := a.write.tree.Node(a.write.node)
			switch {
			case a.write.lost && a.write.ed != nil:
				a.chapterView(c)
			case !ok && len(a.write.tree.Root.Children) == 0:
				a.emptyWriting(c)
			case !ok:
				ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
					ui.Text(c, "从左边选一章开始写").FontSize(14).TextColor(a.pal.faint)
				})
			case n.Folder:
				a.folderView(c, n)
			default:
				a.chapterView(c)
			}
		})
		a.settingColumn(c)
	})
	if a.write.snapsOpen {
		a.snapshots(c)
	}
	if a.write.trashOpen {
		a.trashView(c)
	}
	if a.write.takeOpen {
		a.take(c)
	}
	if a.write.exportOpen {
		a.exportView(c)
	}
	if a.write.draftOpen {
		a.draftView(c)
	}
}

// settingColumn is the setting beside the page, unless the author hid it.
func (a *App) settingColumn(c *ui.Context) {
	if a.doc.hidden || a.page == 3 && a.write.focus {
		return
	}
	win, _ := c.Size()
	width := a.panelWidth(win)
	ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
	panel := ui.Column(c).Width(width).FillHeight().Padding(18, 18, 12, 18).Gap(12).Background(a.pal.side).Shrink(0)
	a.LastSetting = panel.Bounds()
	panel.Children(func() {
		if a.page == 3 && a.doc.talk {
			a.talkPanel(c)
		} else {
			a.settingPanel(c)
		}
		a.resizer(c, width)
	})
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

// weekChart draws the last seven days' writing as bars, those that met the
// goal in the brand color.
func (a *App) weekChart(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	now := time.Now()
	days := a.lastDays(now, 7)
	top := max(slices.Max(days), w.goal, 1)
	const tall = 48
	ui.Text(c, "最近 7 天").FontSize(12).FontWeight(600).TextColor(t.TextMuted).Padding(6, 0, 0, 0)
	ui.Row(c).FillWidth().Gap(4).AlignItems(ui.End).Children(func() {
		for i, k := range days {
			day := now.AddDate(0, 0, i-len(days)+1)
			color := t.TextMuted.Alpha(0.35)
			if w.goal > 0 && k >= w.goal {
				color = a.pal.brand
			}
			ui.Column(c).Key(i).Grow(1).Gap(3).AlignItems(ui.Center).Label(fmt.Sprintf("%s 写了 %d 字", day.Format("1月2日"), k)).Tooltip(fmt.Sprintf("%s：%+d 字", day.Format("1月2日"), k)).Children(func() {
				h := float32(max(k, 0)) / float32(top) * tall
				ui.Box(c).Height(tall - h)
				ui.Box(c).FillWidth().Height(max(h, 2)).Radius(2).Background(color)
				ui.Textf(c, "%d", day.Day()).FontSize(10.5).TextColor(a.pal.faint)
			})
		}
	})
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

func (a *App) writeTop(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	n, ok := w.tree.Node(w.node)
	top := ui.Row(c).FillWidth().Height(52).Padding(0, 16, 0, 24).Gap(6).AlignItems(ui.Center).Shrink(0)
	roomy := top.Bounds().W == 0 || top.Bounds().W >= 600 // the counts give way to the title
	wide := top.Bounds().W == 0 || top.Bounds().W >= 760  // and today's to the book's
	top.Children(func() {
		if ok {
			var path []string
			for p := n.Parent; p != nil && p.Parent != nil; p = p.Parent {
				path = append([]string{p.Title}, path...)
			}
			for _, title := range path {
				ui.Text(c, title).FontSize(13).TextColor(t.TextMuted).SingleLine().Shrink(1)
				glyph(c, icChevron, 14, a.pal.faint)
			}
			ui.Text(c, n.Title).FontSize(13.5).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1)
		} else if w.lost {
			ui.Text(c, w.lostTitle).FontSize(13.5).FontWeight(600)
		} else {
			ui.Text(c, "正文").FontSize(13.5).FontWeight(600)
		}
		ui.Spacer(c)
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
		today, tip := "", fmt.Sprintf("全书 %d 字", a.total())
		k := a.today(time.Now())
		met := w.goal > 0 && k >= w.goal
		switch {
		case w.goal > 0:
			today = fmt.Sprintf(" · 今天 %+d / %d", k, w.goal)
			tip += fmt.Sprintf("，今天 %+d 字，目标 %d 字", k, w.goal)
		case k != 0:
			today = fmt.Sprintf(" · 今天 %+d", k)
			tip += fmt.Sprintf("，今天 %+d 字", k)
		}
		if !wide {
			today = ""
		}
		color := a.pal.faint
		if met {
			color = a.pal.brand
		}
		counts := ui.ButtonBase(c).Row().Padding(3, 6).Radius(6).Shrink(1).Label("字数").Tooltip(tip + "。点击设每日目标")
		if counts.Hovered() {
			counts.Background(t.SurfaceHover)
		}
		counts.Children(func() {
			if ok && !roomy && unit == "选中" {
				ui.Textf(c, "选中 %d 字", here).FontSize(12).TextColor(color).SingleLine()
			} else if ok && !roomy {
				ui.Textf(c, "%d 字", here).FontSize(12).TextColor(color).SingleLine()
			} else if ok {
				ui.Textf(c, "%s %d 字 · 全书 %d 字%s", unit, here, a.total(), today).FontSize(12).TextColor(color).SingleLine()
			} else {
				ui.Textf(c, "全书 %d 字%s", a.total(), today).FontSize(12).TextColor(color).SingleLine()
			}
		})
		if counts.Clicked() {
			w.goalOpen = !w.goalOpen
		}
		ui.Popover(c, counts, &w.goalOpen, func() {
			ui.Column(c).Width(220).Gap(6).Padding(8).Children(func() {
				ui.Text(c, "每日目标").FontSize(13).FontWeight(600)
				in := ui.TextInput(c, &w.goalText).Label("每日目标字数").Placeholder("比如 2000").FillWidth().AutoFocus()
				if in.Changed() {
					a.setGoal(w.goalText)
				}
				if in.Submitted() {
					w.goalOpen = false
				}
				ui.Text(c, "今天写到这个数，顶栏的字数变成主题色。留空就不设。").FontSize(12).TextColor(a.pal.faint).LineHeight(1.5).FillWidth()
				a.weekChart(c)
			})
		})
		ui.Box(c).Width(6)
		if ok && !n.Folder {
			s, e := w.ed.Selection()
			busy := w.ed.ReadOnly() || w.lost || w.external != nil
			if w.ed.Predicting() {
				ui.Text(c, "续写中…").FontSize(12).TextColor(a.pal.brand)
			} else if w.ed.Ghost() != "" {
				ui.Text(c, "Tab 采纳 · Esc 放弃").FontSize(12).TextColor(a.pal.brand)
			}
			if iconButton(c, icSparkle, "续写一句", ui.Color{}).Tooltip("在光标处续写一句（" + modKey + "J），按 Tab 采纳").Disabled(busy).Clicked() {
				a.continueText()
			}
			if iconButton(c, icWand, "AI 改写", ui.Color{}).Tooltip("让 AI 改写选中的部分").Disabled(s == e || busy).Clicked() {
				a.openRewrite()
			}
			draftTip := "按章纲起草这一章"
			if strings.TrimSpace(w.outline) == "" {
				draftTip = "先写章纲，才能按章纲起草"
			}
			if iconButton(c, icFeather, "按章纲起草", ui.Color{}).Tooltip(draftTip).Disabled(busy || strings.TrimSpace(w.outline) == "").Clicked() {
				a.openDraft()
			}
			if iconButton(c, icQuote, "加入设定", ui.Color{}).Tooltip("把选中的原文加入设定").Disabled(s == e || w.lost).Clicked() {
				a.openTake()
			}
			color := ui.Color{}
			if w.finding {
				color = a.pal.brand
			}
			if iconButton(c, icSearch, "查找替换", color).Tooltip("查找替换（" + modKey + "F）").Disabled(busy).Clicked() {
				w.finding = !w.finding
				if !w.finding {
					w.wide, w.texts = false, nil
					w.ed.SetFound(nil, 0)
				}
			}
			if iconButton(c, icClock, "快照", ui.Color{}).Tooltip("这一章的快照").Disabled(busy).Clicked() {
				a.openSnapshots()
			}
		}
		if iconButton(c, icDownload, "导出全书或章节", ui.Color{}).Tooltip("导出全书、本卷或本章").Clicked() {
			w.exportOpen, w.exportDone, w.exportFrom = true, false, ""
		}
		if a.doc.hidden && !w.focus {
			a.bookDot(c)
			if iconButton(c, icPanel, "展开设定", ui.Color{}).Tooltip("展开设定文档").Clicked() {
				a.setDocHidden(false)
			}
		}
		if w.focus {
			if iconButton(c, icShrink, "退出专注", a.pal.brand).Tooltip("退出专注，回到章节树和右栏（F11）").Clicked() {
				w.focus = false
			}
		} else if iconButton(c, icExpand, "专注写作", ui.Color{}).Tooltip("只留正文，收起章节树和右栏（F11）").Clicked() {
			w.focus = true
		}
	})
	a.pal.hairline(c)
}

func (a *App) emptyWriting(c *ui.Context) {
	t := a.pal.theme
	ui.Column(c).Grow(1).FillWidth().AlignItems(ui.Center).Padding(96, 24, 0, 24).Gap(14).Children(func() {
		ui.Box(c).Size(44, 44).Radius(12).Background(a.pal.brand.Alpha(0.1)).Center().Children(func() {
			glyph(c, icBook, 22, a.pal.brand)
		})
		ui.Text(c, "还没有章节").FontSize(18).FontWeight(600)
		ui.Text(c, "每一章是作品文件夹 chapters 里的一个纯文本文件，离开软件也能打开。").FontSize(13).TextColor(t.TextMuted)
		ui.Row(c).Gap(10).Padding(8, 0, 0, 0).Children(func() {
			if a.pal.primary(c, "新建第一章").Clicked() {
				a.newNode(false)
			}
			if a.pal.secondary(c, "新建文件夹").Clicked() {
				a.newNode(true)
			}
		})
	})
}

// outlineBox is the outline of the node shown: 章纲 of a document, 卷纲 of
// a folder.
func (a *App) outlineBox(c *ui.Context, label string, height float32) {
	w := &a.write
	ui.TextAreaBase(c, &w.outline).Label(label).Placeholder("写下这里要发生什么，作为你自己的提纲").FillWidth().Height(height).
		FontSize(13.5).LineHeight(1.7).Padding(10, 12).Radius(8).Background(a.pal.side).Disabled(w.lost)
}

func (a *App) chapterView(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	if w.external != nil {
		a.notice2(c, t.Warning, "磁盘上的这一章被别的程序改过了。保留哪一边？另一边会存成快照，不会丢。", func() {
			if action(c, "用磁盘上的", t.Warning, false).Clicked() {
				a.takeDisk()
			}
			if action(c, "保留我的", t.Warning, true).Clicked() {
				a.keepMine()
			}
		})
	}
	if w.lost {
		a.notice2(c, t.Danger, "磁盘上找不到这一章了：在软件外改名、移动或删除，都会被当成删掉了。编辑器里的文字还在。", func() {
			if action(c, "存为新章节", t.Danger, true).Clicked() {
				a.saveLost()
			}
		})
	}
	// As wide as the prose at most, and over it, when the page is wide.
	ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(10, 24, 0, 24).Shrink(0).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(editor.MaxTextWidth).Children(func() {
			label := "章纲"
			if strings.TrimSpace(w.outline) == "" {
				label = "章纲（空）"
			}
			ui.Collapsible(c, label, &w.outlineOpen, func() {
				ui.Box(c).FillWidth().Padding(6, 0, 4, 0).Children(func() { a.outlineBox(c, "章纲", 96) })
			})
			a.summaryBox(c)
		})
	})
	if w.finding && !w.ed.ReadOnly() {
		a.findBar(c)
	}
	a.aiBar(c)
	w.ed.View(c).Grow(1).FillWidth()
}

// notice2 is a banner over the editor that asks the author something.
func (a *App) notice2(c *ui.Context, color ui.Color, text string, buttons func()) {
	t := a.pal.theme
	ui.Row(c).FillWidth().Padding(10, 20, 0, 20).Shrink(0).Children(func() {
		ui.Row(c).FillWidth().Padding(8, 8, 8, 12).Gap(10).Radius(9).AlignItems(ui.Center).Background(color.Alpha(0.08)).Border(1, color.Alpha(0.2)).Children(func() {
			glyph(c, icAlert, 15, color)
			ui.Text(c, text).FontSize(13).TextColor(color.Mix(t.Text, 0.35)).Grow(1).Shrink(1)
			buttons()
		})
	})
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
			ui.Text(c, "没有找到").FontSize(12.5).TextColor(a.pal.faint).Shrink(0)
		default:
			ui.Textf(c, "%d / %d", w.current+1, len(found)).FontSize(12.5).TextColor(t.TextMuted).Shrink(0)
		}
	}
	move := func() {
		if ghost(c, "上一个", ui.Color{}).Disabled(none).Clicked() {
			w.current = (w.current + len(found) - 1) % len(found)
			show()
		}
		if ghost(c, "下一个", ui.Color{}).Disabled(none).Clicked() {
			w.current = (w.current + 1) % len(found)
			show()
		}
	}
	replace := func() {
		if ghost(c, "替换", ui.Color{}).Disabled(none || w.lost).Clicked() {
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
			if ghost(c, "全书替换", ui.Color{}).Disabled(w.query == "" || len(w.hits) == 0 || w.lost || w.external != nil).Clicked() {
				w.sure = !w.sure
			}
			return
		}
		if ghost(c, "全部替换", ui.Color{}).Disabled(none || w.lost).Clicked() {
			n := w.ed.ReplaceAll(w.query, w.with)
			a.notice = fmt.Sprintf("已替换 %d 处，可以撤销", n)
		}
	}
	wide := func() {
		color := ui.Color{}
		if w.wide {
			color = a.pal.brand
		}
		if ghost(c, "全书", color).Tooltip("在所有章节里找").Clicked() {
			w.wide, w.texts, w.sure = !w.wide, nil, false
		}
	}
	closer := func() {
		if iconButton(c, icX, "关闭查找", ui.Color{}).Clicked() {
			w.finding, w.wide, w.texts, w.sure = false, false, nil, false
			w.ed.SetFound(nil, 0)
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
	ui.Scroll(c).Key("folder").Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(28, 28, 28, 28).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(720).Gap(14).Children(func() {
				ui.Text(c, n.Title).FontSize(22).FontWeight(600)
				ui.Text(c, "卷纲").FontSize(13).FontWeight(600).TextColor(t.TextMuted)
				a.outlineBox(c, "卷纲", 160)
				ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(8, 0, 0, 0).Children(func() {
					ui.Textf(c, "%d 项", len(n.Children)).FontSize(13).FontWeight(600).TextColor(t.TextMuted).Grow(1)
					if ghost(c, "在这里新建章节", ui.Color{}).Clicked() {
						a.newNode(false)
					}
				})
				for _, child := range n.Children {
					row := ui.ButtonBase(c).Key("f-"+child.ID).Row().FillWidth().Justify(ui.Start).AlignItems(ui.Center).Gap(10).Padding(9, 12).Radius(8).Label("打开-" + child.Title)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					row.Children(func() {
						glyph(c, nodeIcon(child), 16, t.TextMuted)
						ui.Text(c, child.Title).FontSize(14).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
						if !child.Folder {
							ui.Textf(c, "%d 字", w.counts[child.ID]).FontSize(12).TextColor(a.pal.faint)
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
	t := a.pal.theme
	w := &a.write
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 4, 0, 8).Gap(2).Children(func() {
		ui.Text(c, "章节").FontSize(11.5).FontWeight(600).TextColor(a.pal.faint).Grow(1)
		if iconButton(c, icFolderPlus, "新建文件夹", ui.Color{}).Clicked() {
			a.newNode(true)
		}
		if iconButton(c, icPlus, "新建章节", ui.Color{}).Tooltip("新建章节（" + modKey + "N）").Clicked() {
			a.newNode(false)
		}
	})
	ui.Scroll(c).Key("chapters").FillWidth().Grow(1).Gap(1).Children(func() {
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
	b := ui.ButtonBase(c).Row().FillWidth().Justify(ui.Start).AlignItems(ui.Center).Gap(10).Padding(7, 10).Radius(7).Label("回收站")
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		glyph(c, icTrash, 15, t.TextMuted)
		ui.Text(c, "回收站").FontSize(13).TextColor(t.TextMuted)
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
		ui.Row(c).Key("r-"+n.ID).FillWidth().AlignItems(ui.Center).Gap(4).Padding(2, 4, 2, 8+float32(depth)*14).Children(func() {
			in := ui.TextInput(c, &w.title).Label("章节标题").Placeholder(w.titleWas).Grow(1).AutoFocus()
			if in.Submitted() || iconButton(c, icCheck, "确定标题", ui.Color{}).Clicked() {
				a.rename(n.ID)
			}
			if in.Shortcut(0, ui.KeyEscape) {
				w.renaming = ""
			}
		})
		return
	}
	row := ui.ButtonBase(c).Key("n-"+n.ID).Row().FillWidth().Justify(ui.Start).AlignItems(ui.Center).Gap(6).
		Padding(6, 8, 6, 8+float32(depth)*14).Radius(7).Label("章节-" + n.Title)
	row.Drag(dragNode(n.ID))
	_, over := ui.DragOver[dragNode](row)
	switch {
	case over:
		row.Background(a.pal.brand.Alpha(0.12))
	case on:
		row.Background(t.SurfacePressed)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	if v, ok := ui.Drop[dragNode](row); ok {
		a.dropOn(string(v), n)
	}
	row.ContextMenu(func(m *ui.Menu) { a.nodeMenu(m, n) })
	row.Children(func() {
		fg := t.TextMuted
		if on {
			fg = t.Text
		}
		if n.Folder {
			s := icChevronDown
			if w.folded[n.ID] {
				s = icChevron
			}
			glyph(c, s, 13, a.pal.faint)
		}
		glyph(c, nodeIcon(n), 15, fg)
		ui.Text(c, n.Title).FontSize(13).TextColor(fg).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		if k := w.wordsIn(n); k > 0 {
			ui.Textf(c, "%d", k).FontSize(11).TextColor(a.pal.faint)
		}
	})
	if row.Clicked() {
		if n.Folder && on {
			w.folded[n.ID] = !w.folded[n.ID]
		} else {
			w.folded[n.ID] = false
			a.openNode(n.ID)
		}
	}
}

// nodeMenu is what a node's context menu offers.
func (a *App) nodeMenu(m *ui.Menu, n *workspace.Node) {
	w := &a.write
	siblings := n.Parent.Children
	i := slices.Index(siblings, n)
	if m.Item("重命名").Chosen() {
		w.renaming, w.title, w.titleWas, w.offered = n.ID, n.Title, n.Title, ""
	}
	if m.Item("新建章节").Chosen() {
		a.openNode(n.ID)
		a.newNode(false)
	}
	if m.Item("新建文件夹").Chosen() {
		a.openNode(n.ID)
		a.newNode(true)
	}
	m.Separator()
	if m.Item("上移").Disabled(i == 0).Chosen() {
		a.move(n.ID, n.Parent.ID, i-1)
	}
	if m.Item("下移").Disabled(i == len(siblings)-1).Chosen() {
		a.move(n.ID, n.Parent.ID, i+1)
	}
	m.Submenu("移到", func(m *ui.Menu) {
		if m.Item("最上层").Disabled(n.Parent.Parent == nil).Chosen() {
			a.move(n.ID, "", len(w.tree.Root.Children))
		}
		w.tree.Walk(func(f *workspace.Node, depth int) {
			if !f.Folder || f.Within(n) {
				return
			}
			if m.Item(strings.Repeat("　", depth) + f.Title).Disabled(f == n.Parent).Chosen() {
				a.move(n.ID, f.ID, len(f.Children))
			}
		})
	})
	m.Separator()
	if m.Item("移到回收站").Chosen() {
		a.trashNode(n)
	}
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
	w.offered = ""
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
	w := &a.write
	parent, index := "", -1
	if n, ok := w.tree.Node(w.node); ok {
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
	w.renaming, w.title, w.titleWas, w.offered = id, typed, title, typed
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
	if cur, ok := w.tree.Node(w.node); ok && cur.Within(n) {
		w.node, w.ed, w.external, w.lost = "", nil, nil, false
	}
	if err := a.Store.TrashChapter(n.ID); err != nil {
		a.warn(err)
		return
	}
	a.warn(a.loadTree())
	counts, err := a.Store.Counts()
	a.warn(err)
	w.counts = counts
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
	ui.Modal(c, &w.trashOpen, func() {
		ui.Column(c).Width(520).Gap(14).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "回收站").FontSize(17).FontWeight(600)
				ui.Text(c, "删掉的章节连同它里面的内容先放在这里，恢复后回到原来所在文件夹的末尾。").FontSize(13).TextColor(t.TextMuted).LineHeight(1.6)
			})
			if len(w.trash) == 0 {
				ui.Text(c, "回收站是空的").FontSize(13).TextColor(a.pal.faint).Padding(12, 0)
			}
			ui.Scroll(c).Key("trash").FillWidth().MaxHeight(360).Children(func() {
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					for _, it := range w.trash {
						ui.Row(c).Key("t-"+it.ID).FillWidth().AlignItems(ui.Center).Gap(10).Padding(8, 4).Children(func() {
							s := icDoc
							if it.Folder {
								s = icFolder
							}
							glyph(c, s, 15, t.TextMuted)
							ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
								ui.Text(c, it.Title).FontSize(13.5).SingleLine().Ellipsis("…")
								ui.Text(c, it.At.Local().Format("01-02 15:04")+" 删除").FontSize(11.5).TextColor(a.pal.faint)
							})
							if ghost(c, "恢复", ui.Color{}).Label("恢复-" + it.Title).Clicked() {
								a.restore(it.ID, it.Title)
							}
							if w.purge == it.ID {
								if action(c, "确认彻底删除", t.Danger, true).Clicked() {
									a.warn(a.Store.PurgeChapter(it.ID))
									a.openTrash()
								}
							} else if action(c, "彻底删除", t.Danger, false).Clicked() {
								w.purge = it.ID
							}
						})
					}
				})
			})
			ui.Row(c).FillWidth().Children(func() {
				ui.Spacer(c)
				if ghost(c, "关闭", ui.Color{}).Clicked() {
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

// snapReasons say why a snapshot was taken.
var snapReasons = map[string]string{
	workspace.SnapOpen: "打开时", workspace.SnapInterval: "写作中", workspace.SnapRestore: "恢复前",
	workspace.SnapExternal: "外部改动", workspace.SnapAI: "AI 改动前", workspace.SnapReplace: "全书替换前",
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

func (a *App) snapshots(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	ui.Modal(c, &w.snapsOpen, func() {
		same := w.snap != "" && w.snapText == w.ed.Doc.String() // nothing to restore
		ui.Column(c).Width(min(900, max(a.LastWrite.W, 640))).Height(560).Gap(12).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "快照").FontSize(17).FontWeight(600)
				ui.Text(c, "打开章节、连续写作每十分钟、恢复之前、AI 起草或采纳改写之前、全书替换之前都会存一份。最近七天的全部保留，更早的每天留最后一份。").FontSize(13).TextColor(t.TextMuted)
			})
			ui.Row(c).FillWidth().Grow(1).Gap(14).AlignItems(ui.Stretch).Children(func() {
				ui.Scroll(c).Key("snaps").Width(210).FillHeight().Shrink(0).Children(func() {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						if len(w.snaps) == 0 {
							ui.Text(c, "还没有快照").FontSize(13).TextColor(a.pal.faint).Padding(8)
						}
						for _, sn := range w.snaps {
							on := sn.Name == w.snap
							when := sn.At.Local().Format("01-02 15:04:05")
							b := ui.ButtonBase(c).Key("s-"+sn.Name).Column().FillWidth().AlignItems(ui.Start).Gap(2).Padding(8, 10).Radius(7).Label("快照-" + when)
							if on {
								b.Background(t.SurfacePressed)
							} else if b.Hovered() {
								b.Background(t.SurfaceHover)
							}
							b.Children(func() {
								ui.Text(c, when).FontSize(13).FontWeight(map[bool]int{true: 600, false: 400}[on])
								ui.Textf(c, "%s · %d 字", snapReasons[sn.Reason], sn.Count).FontSize(11.5).TextColor(a.pal.faint)
							})
							if b.Clicked() {
								a.pickSnapshot(sn.Name)
							}
						}
					})
				})
				ui.Column(c).Grow(1).Shrink(1).FillHeight().Gap(8).Children(func() {
					ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Toggle(c, &w.compare, "和现在对比")
						if same {
							ui.Text(c, "这一份和现在的正文一样").FontSize(12).TextColor(a.pal.faint).Padding(0, 0, 0, 8)
						} else if w.compare {
							ui.Text(c, "红色划掉的是快照里有、现在没有的，绿色是现在新写的").FontSize(12).TextColor(a.pal.faint).Padding(0, 0, 0, 8)
						}
					})
					if i := slices.IndexFunc(w.snaps, func(s workspace.Snapshot) bool { return s.Name == w.snap }); i >= 0 && w.snaps[i].AI != nil {
						a.aiRecord(c, w.snaps[i].AI)
					}
					ui.Scroll(c).Key("snap-text").FillWidth().Grow(1).Background(a.pal.side).Radius(10).Children(func() {
						ui.Box(c).FillWidth().Padding(14, 16).Children(func() {
							switch {
							case w.snap == "":
							case w.compare:
								var spans []ui.Span
								for _, p := range diffText(w.snapText, w.ed.Doc.String()) {
									switch p.kind {
									case gone:
										spans = append(spans, ui.Span{Text: p.text, Color: t.Danger, Strikethrough: true, Background: t.Danger.Alpha(0.1)})
									case came:
										spans = append(spans, ui.Span{Text: p.text, Color: t.Success, Background: t.Success.Alpha(0.12)})
									default:
										spans = append(spans, ui.Span{Text: p.text})
									}
								}
								ui.RichText(c, spans...).FontSize(14).LineHeight(1.8).FillWidth().Selectable()
							default:
								ui.Text(c, w.snapText).FontSize(14).LineHeight(1.8).FillWidth().Selectable()
							}
						})
					})
				})
			})
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if ghost(c, "关闭", ui.Color{}).Clicked() {
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
	a.notice = "已恢复到 " + w.snaps[i].At.Local().Format("01-02 15:04") + " 的版本，恢复前的内容也存了快照"
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
		w.takeCategory = workspace.Other
	}
}

func (a *App) take(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	ui.Modal(c, &w.takeOpen, func() {
		ui.Column(c).Width(560).Gap(14).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "加入设定").FontSize(17).FontWeight(600)
				ui.Text(c, "选中的原文作为出处记下来；设定可以换成你自己的说法。").FontSize(13).TextColor(t.TextMuted)
			})
			ui.Box(c).FillWidth().Padding(10, 12).Radius(8).Background(a.pal.side).Children(func() {
				ui.Text(c, w.takeQuote).FontSize(13).TextColor(t.TextMuted).LineHeight(1.7).FillWidth().Selectable()
			})
			ui.TextAreaBase(c, &w.takeText).Label("设定内容").FillWidth().Height(76).FontSize(13.5).LineHeight(1.65).
				Padding(10, 12).Radius(8).Border(1, t.Border)
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Select(c, &w.takeCategory, a.categoryNames()).Label("设定分类").Width(140)
				ui.Spacer(c)
				if ghost(c, "取消", ui.Color{}).Clicked() {
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
	t := a.pal.theme
	w := &a.write
	ui.Modal(c, &w.exportOpen, func() {
		ui.Column(c).Width(460).Gap(14).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "导出正文").FontSize(17).FontWeight(600)
				ui.Text(c, "按章节树的顺序拼成一个 .txt，每个文件夹和章节前写上标题。").FontSize(13).TextColor(t.TextMuted)
			})
			ui.RadioGroup(c, func() {
				ui.Radio(c, &w.exportFrom, "", "全书")
				for _, n := range a.exportParts() {
					unit := "本章"
					if n.Folder {
						unit = "本卷"
					}
					ui.Radio(c, &w.exportFrom, n.ID, unit+"「"+n.Title+"」")
				}
			}).Label("导出范围")
			ui.Checkbox(c, &w.indent, "段首加两个全角空格")
			ui.Row(c).FillWidth().Gap(10).Children(func() {
				ui.Spacer(c)
				if ghost(c, "取消", ui.Color{}).Clicked() {
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
	if !ok {
		return nil
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
	save := a.SaveFile
	go func() {
		path, err := save("导出", name)
		a.picks <- func() {
			if err != nil {
				a.warn(err)
				return
			}
			write(path)
		}
		c.Invalidate()
	}()
}
