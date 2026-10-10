package desk

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
	"writer/writing"
)

// When a chapter is checked against the setting, as the work's
// preferences keep it.
const (
	checkOnLeave  = ""       // as the author leaves a chapter changed
	checkOnDemand = "manual" // only when asked
	checkOnPause  = "always" // whenever the writing pauses
)

// checkPause is how long the writing rests before a chapter is checked as
// it is written.
const checkPause = 8 * time.Second

var checkWhens = []struct{ when, label, short, sub string }{
	{checkOnLeave, "写完一章就查", "写完一章就查", "推荐，每章用一次 AI"},
	{checkOnDemand, "我点了才查", "点了才查", "平时不用 AI"},
	{checkOnPause, "边写边查", "边写边查", "停笔就查，最费额度"},
}

// checks is the chapters waiting to be checked against the setting, one
// at a time in the background, what the checks found, and the page that
// shows it.
type checks struct {
	queue   []string
	running string
	cancel  context.CancelFunc
	done    chan checkDone
	failed  map[string]string // why a chapter's last check failed
	tried   map[string]string // the prose a pause last had checked, by chapter
	total   int               // the checks asked for since the queue was empty

	of   *workspace.Store // the work all is of
	all  map[string]workspace.Check
	tree workspace.Tree
	when string

	tab      int
	shown    string // the finding the page shows
	whenOpen bool
	askOpen  bool
	due      []string // the chapters a check of the book would read

	prose, proseOf string // the chapter the page quotes, and its ID
	live           string // the open chapter's text, as of liveVer
	liveOf         string
	liveVer        int
	doubtKey       string
}

type checkDone struct {
	chapter string
	store   *workspace.Store
	check   workspace.Check
	err     error
}

// checkItem is a finding with where it is.
type checkItem struct {
	node, title string
	ch          int
	here        bool // in the chapter open
	f           workspace.Finding
	entry       workspace.Entry
}

func chapterLabel(ch int) string { return fmt.Sprintf("第%d章", ch) }

// entryTitle names an entry the way the setting lists it: 人物 · 林晚.
func entryTitle(e workspace.Entry) string {
	if len(e.Names) > 0 {
		return e.Category + " · " + e.Names[0]
	}
	return e.Category
}

// loadChecks reads what the checks of the work found.
func (a *App) loadChecks() {
	k := &a.ck
	all, err := a.Store.Checks()
	a.warn(err)
	k.all = all
	if t, err := a.Store.Chapters(); err == nil {
		k.tree = t
	}
	k.doubtKey = ""
}

// queueCheck asks for chapter's check unless it is waiting or running.
func (a *App) queueCheck(chapter string) {
	k := &a.ck
	if k.running == chapter || slices.Contains(k.queue, chapter) {
		return
	}
	if k.running == "" && len(k.queue) == 0 {
		k.total = 0
	}
	k.queue = append(k.queue, chapter)
	k.total++
	delete(k.failed, chapter)
}

// hasSetting tells whether there is a setting to check prose against.
func (a *App) hasSetting() bool {
	return slices.ContainsFunc(a.set.Entries, func(e workspace.Entry) bool { return e.Place == workspace.PlaceBody })
}

// leaveCheck is called as the author leaves the chapter shown, before its
// summary is seen to: what the prose no longer says is no longer found,
// and a chapter changed is checked again, when that is when to.
func (a *App) leaveCheck() {
	w := &a.write
	k := &a.ck
	if w.node == "" || w.lost || a.Store == nil {
		return
	}
	a.pruneCheck(w.node, w.saved)
	if !w.changed || k.when != checkOnLeave || !a.hasSetting() || workspace.Count(w.saved) < writing.MinCheck {
		return
	}
	if had, ok := k.all[w.node]; !ok || had.Stale(w.saved) {
		a.queueCheck(w.node)
	}
}

// pruneCheck drops from chapter's check what its prose, text, no longer
// says and the author did not call meant.
func (a *App) pruneCheck(chapter, text string) {
	k := &a.ck
	had, ok := k.all[chapter]
	if !ok {
		return
	}
	kept := slices.DeleteFunc(slices.Clone(had.Findings), func(f workspace.Finding) bool { return !strings.Contains(text, f.Quote) })
	if len(kept) == len(had.Findings) {
		return
	}
	had.Findings = kept
	if err := a.Store.WriteCheck(chapter, had); err != nil {
		a.warn(err)
		return
	}
	k.all[chapter] = had
	k.doubtKey = ""
}

// pruneChecks does so for every chapter with something found, as of the
// disk.
func (a *App) pruneChecks() {
	for id, had := range a.ck.all {
		if !slices.ContainsFunc(had.Findings, func(f workspace.Finding) bool { return !f.Meant }) {
			continue
		}
		if ch, err := a.Store.ReadChapter(id); err == nil {
			a.pruneCheck(id, ch.Text)
		}
	}
}

// pollChecks takes in a check done, and starts the next.
func (a *App) pollChecks(c *ui.Context) {
	k := &a.ck
	if k.done == nil {
		k.done = make(chan checkDone, 1)
		k.failed, k.tried = map[string]string{}, map[string]string{}
	}
	if k.of != a.Store {
		k.of = a.Store
		k.shown, k.prose, k.proseOf, k.tab = "", "", "", 0
		clear(k.failed)
		clear(k.tried)
		a.loadChecks()
	}
	select {
	case d := <-k.done:
		k.running, k.cancel = "", nil
		if d.store == a.Store {
			a.keepCheck(d)
		}
	default:
	}
	if k.running != "" || len(k.queue) == 0 {
		return
	}
	id := k.queue[0]
	k.queue = k.queue[1:]
	pw, err := a.Pen(a.Store)
	if err != nil {
		k.failed[id] = err.Error()
		k.queue = nil
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	k.running, k.cancel = id, cancel
	store := a.Store
	go func() {
		check, err := pw.Check(ctx, id)
		cancel()
		k.done <- checkDone{chapter: id, store: store, check: check, err: err}
		c.Invalidate()
	}()
}

// keepCheck saves what a check found, with what the author called meant
// while it ran.
func (a *App) keepCheck(d checkDone) {
	k := &a.ck
	if d.err != nil {
		k.failed[d.chapter] = d.err.Error()
		return
	}
	delete(k.failed, d.chapter)
	for _, old := range k.all[d.chapter].Findings {
		if !old.Meant {
			continue
		}
		for i, f := range d.check.Findings {
			if f.Kind == old.Kind && f.Quote == old.Quote && f.Entry == old.Entry && f.Name == old.Name {
				d.check.Findings[i].Meant = true
			}
		}
	}
	if err := a.Store.WriteCheck(d.chapter, d.check); err != nil {
		k.failed[d.chapter] = err.Error()
		return
	}
	a.loadChecks()
}

// stopChecks drops the checks waiting, as the work closes.
func (a *App) stopChecks() {
	k := &a.ck
	if k.cancel != nil {
		k.cancel()
	}
	k.queue, k.total = nil, 0
}

// checkWhilePausing checks the chapter shown once the writing rests, when
// that is when to: each state of the prose at most once.
func (a *App) checkWhilePausing() {
	w := &a.write
	k := &a.ck
	if k.when != checkOnPause || w.ed == nil || w.node == "" || w.lost || w.ed.Doc.Version() != w.savedVer ||
		time.Since(w.ed.LastEdit()) < checkPause || !a.hasSetting() {
		return
	}
	hash := workspace.ProseHash(w.saved)
	if k.tried[w.node] == hash || k.all[w.node].Of == hash || workspace.Count(w.saved) < writing.MinCheck {
		return
	}
	k.tried[w.node] = hash
	a.queueCheck(w.node)
}

// liveText is the open chapter's text as it is being written.
func (a *App) liveText() (string, bool) {
	w := &a.write
	k := &a.ck
	if w.ed == nil || w.node == "" || !w.entered {
		return "", false
	}
	if k.liveOf != w.node || k.liveVer != w.ed.Doc.Version() || k.live == "" {
		k.live, k.liveOf, k.liveVer = w.ed.Doc.String(), w.node, w.ed.Doc.Version()
	}
	return k.live, true
}

// checkItems is what the checks found and still holds, in the book's
// order: with meant, also what the author said is as meant.
func (a *App) checkItems(meant bool) []checkItem {
	k := &a.ck
	tree := k.tree
	if a.write.entered {
		tree = a.write.tree
	}
	live, open := a.liveText()
	var out []checkItem
	n := 0
	tree.Walk(func(nd *workspace.Node, _ int) {
		if nd.Folder {
			return
		}
		n++
		here := open && nd.ID == a.write.node
		for _, f := range k.all[nd.ID].Findings {
			if f.Meant && !meant || here && !strings.Contains(live, f.Quote) {
				continue
			}
			it := checkItem{node: nd.ID, title: nd.Title, ch: n, here: here, f: f}
			switch f.Kind {
			case workspace.FindConflict:
				e, ok := a.set.Entry(f.Entry)
				if !ok || f.Says != "" && !strings.Contains(e.Text, f.Says) {
					continue // the setting was changed
				}
				it.entry = e
			case workspace.FindName:
				for _, e := range a.set.Entries {
					if slices.Contains(e.Names, f.Name) {
						it.entry = e
					}
				}
				if it.entry.ID == "" {
					continue
				}
			case workspace.FindNew:
				if slices.ContainsFunc(a.set.Entries, func(e workspace.Entry) bool {
					return e.Place != workspace.PlaceDropped && slices.Contains(e.Names, f.Name)
				}) {
					continue // taken into the setting
				}
			}
			out = append(out, it)
		}
	})
	return out
}

// setMeant records that the author means what it found.
func (a *App) setMeant(it checkItem) {
	k := &a.ck
	had := k.all[it.node]
	for i := range had.Findings {
		if had.Findings[i].ID == it.f.ID {
			had.Findings[i].Meant = true
		}
	}
	if err := a.Store.WriteCheck(it.node, had); err != nil {
		a.warn(err)
		return
	}
	k.all[it.node] = had
	k.doubtKey = ""
}

// checkDue lists the chapters a check of the book would read: those long
// enough, never checked or changed since.
func (a *App) checkDue() []string {
	var due []string
	a.ck.tree.Walk(func(nd *workspace.Node, _ int) {
		if nd.Folder {
			return
		}
		ch, err := a.Store.ReadChapter(nd.ID)
		if err != nil || workspace.Count(ch.Text) < writing.MinCheck {
			return
		}
		if had, ok := a.ck.all[nd.ID]; !ok || had.Stale(ch.Text) {
			due = append(due, nd.ID)
		}
	})
	return due
}

// openCheck shows the page of the checks, at finding id when given.
func (a *App) openCheck(id string) {
	a.write.pendOpen = false
	a.page = pageCheck
	if id != "" {
		a.ck.shown, a.ck.tab = id, 0
	}
}

// checkPending is the findings as the pending list has them.
func (a *App) checkPending() (pendGroup, bool) {
	g := pendGroup{name: "设定对不上", color: a.pal.amber}
	for _, it := range a.checkItems(false) {
		p := pendItem{key: "检查/" + it.node + "/" + it.f.Kind + "/" + it.f.Quote, text: it.f.Title, sub: chapterLabel(it.ch), node: it.node, here: it.here, check: it.f.ID}
		if it.f.Kind == workspace.FindNew {
			p.text = fmt.Sprintf("「%s」还不在设定里", it.f.Name)
		}
		if it.here {
			p.sub += " · 本章"
		}
		if !a.write.ignored[p.key] {
			g.items = append(g.items, p)
		}
	}
	return g, len(g.items) > 0
}

// markDoubts puts the light dashed line under what the checks found in the
// chapter shown.
func (a *App) markDoubts() {
	w := &a.write
	k := &a.ck
	text, ok := a.liveText()
	if !ok {
		return
	}
	items := slices.DeleteFunc(a.checkItems(false), func(it checkItem) bool { return !it.here })
	key := fmt.Sprint(w.node, " ", k.liveVer)
	for _, it := range items {
		key += " " + it.f.ID
	}
	if key == k.doubtKey {
		return
	}
	k.doubtKey = key
	var ranges [][2]int
	for _, it := range items {
		if at := strings.Index(text, it.f.Quote); at >= 0 {
			s := len([]rune(text[:at]))
			ranges = append(ranges, [2]int{s, s + len([]rune(it.f.Quote))})
		}
	}
	w.ed.SetDoubts(ranges)
}

// checkState says in a line where the check of the chapter shown stands,
// and whether there is something of it to look at.
func (a *App) checkState() (line string, found int, can bool) {
	w := &a.write
	k := &a.ck
	for _, it := range a.checkItems(false) {
		if it.here {
			found++
		}
	}
	name := "这一章"
	if n := a.chapterNo(w.node); n > 0 {
		name = chapterLabel(n)
	}
	text, _ := a.liveText()
	had, ok := k.all[w.node]
	switch {
	case k.running == w.node:
		return "正在检查" + name + "…", 0, false
	case slices.Contains(k.queue, w.node):
		return name + "排队等检查", 0, false
	case k.failed[w.node] != "":
		return "检查没成功：" + k.failed[w.node], 0, true
	case !a.hasSetting():
		return "设定还是空的，没有可以对照的", 0, false
	case workspace.Count(text) < writing.MinCheck:
		return fmt.Sprintf("满 %d 字后可以检查", writing.MinCheck), 0, false
	case !ok:
		return name + "还没查过", 0, true
	case found > 0 && had.Stale(text):
		return fmt.Sprintf("%s有 %d 处要看，正文又改过了", name, found), found, true
	case found > 0:
		return fmt.Sprintf("%s查完了，有 %d 处要看", name, found), found, false
	case had.Stale(text):
		return name + "改过，还没再查", 0, true
	}
	return name + "查完了，没有对不上的", 0, false
}

// checkBar is the line at the foot of a chapter: where its check stands,
// when chapters are checked, and whether the text is saved.
func (a *App) checkBar(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	k := &a.ck
	a.pal.hairline(c)
	ui.Row(c).FillWidth().Height(hRow).Gap(8).Padding(0, 24).AlignItems(ui.Center).Shrink(0).Children(func() {
		line, found, can := a.checkState()
		if found > 0 {
			b := ui.ButtonBase(c).Label("看检查结果").Tooltip("去「检查」页逐条看").Shrink(1)
			b.Children(func() { ui.Text(c, line).FontSize(fsNote).TextColor(t.Text).SingleLine().Ellipsis("…") })
			if b.Clicked() {
				for _, it := range a.checkItems(false) {
					if it.here {
						a.openCheck(it.f.ID)
						break
					}
				}
			}
		} else {
			ui.Text(c, line).FontSize(fsNote).TextColor(t.Text).SingleLine().Ellipsis("…").Shrink(1)
		}
		if can {
			b := ui.ButtonBase(c).Label("现在查").Tooltip("用一次 AI 对照设定检查这一章").Shrink(0)
			color := t.TextMuted
			if b.Hovered() {
				color = t.Text
			}
			b.Children(func() { ui.Text(c, "现在查").FontSize(fsNote).TextColor(color).Underline().SingleLine() })
			if b.Clicked() {
				if err := a.saveText(); err != nil {
					a.warn(err)
				} else {
					a.queueCheck(w.node)
				}
			}
		}
		ui.Text(c, "·").FontSize(fsNote).TextColor(a.pal.faint)
		short := checkWhens[0].short
		for _, o := range checkWhens {
			if o.when == k.when {
				short = o.short
			}
		}
		b := ui.ButtonBase(c).Label("什么时候检查").Shrink(0)
		color := t.TextMuted
		if b.Hovered() {
			color = t.Text
		}
		b.Children(func() { ui.Text(c, short+" ▴").FontSize(fsNote).TextColor(color).SingleLine() })
		if b.Clicked() {
			k.whenOpen = !k.whenOpen
		}
		ui.PopoverBase(c, b, &k.whenOpen, func(panel *ui.Element) {
			panel.AttachTo(b, ui.AnchorTopLeft, ui.AnchorBottomLeft).Margin(0, 0, 10, 0).Width(340).Radius(rCard).Padding(14).Gap(12).
				Background(a.pal.canvas).Border(1, t.Border)
			a.pal.pop.on(panel)
			ui.Text(c, "什么时候检查").FontSize(fsBody).FontWeight(600)
			a.whenRows(c)
		})
		ui.Spacer(c)
		saved := "已保存"
		if w.ed != nil && w.ed.Doc.Version() != w.savedVer {
			saved = "还没保存"
		}
		ui.Text(c, saved).FontSize(fsNote).TextColor(a.pal.faint).SingleLine().Shrink(0)
	})
}

// whenRows are the three times to check, one chosen.
func (a *App) whenRows(c *ui.Context) {
	t := a.pal.theme
	k := &a.ck
	for _, o := range checkWhens {
		on := o.when == k.when
		ring := a.pal.faint
		if on {
			ring = a.pal.ink
		}
		b := ui.ButtonBase(c).Row().FillWidth().Gap(10).AlignItems(ui.Start).Justify(ui.Start).Label(o.label)
		b.Children(func() {
			ui.Box(c).Size(14, 14).Margin(4, 0, 0, 0).Radius(rRound).Border(1.5, ring).Center().Shrink(0).Children(func() {
				if on {
					dot(c, a.pal.ink, 6)
				}
			})
			ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
				ui.Text(c, o.label).FontSize(fsBody)
				ui.Text(c, o.sub).FontSize(fsNote).TextColor(t.TextMuted)
			})
		})
		if b.Clicked() && !on {
			k.when = o.when
			a.savePrefs()
		}
	}
}

// checkKinds are the page's tabs.
var checkKinds = []struct{ kind, label string }{
	{"", "全部"}, {workspace.FindConflict, "对不上"}, {workspace.FindName, "名字写错"}, {workspace.FindNew, "新出现"},
}

func kindLabel(kind string) string {
	for _, k := range checkKinds {
		if k.kind == kind {
			return k.label
		}
	}
	return ""
}

// checkPage is the page 检查: what the checks found on the left, the one
// chosen on the right, the prose beside the setting.
func (a *App) checkPage(c *ui.Context) {
	k := &a.ck
	all := a.checkItems(false)
	var shown []checkItem
	for _, it := range all {
		if kind := checkKinds[k.tab].kind; kind == "" || kind == it.f.Kind {
			shown = append(shown, it)
		}
	}
	at := slices.IndexFunc(shown, func(it checkItem) bool { return it.f.ID == k.shown })
	if at < 0 && len(shown) > 0 {
		at, k.shown = 0, shown[0].f.ID
	}
	_, right := a.chromeRoom(c)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(360).Shrink(0).FillHeight().Children(func() { a.checkList(c, all, shown) })
		ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
		ui.Column(c).Grow(1).Shrink(1).FillHeight().Padding(32, 32, 20, 32).Gap(20).Children(func() {
			if right > 0 {
				ui.Box(c).Height(20).Shrink(0)
			}
			if at < 0 {
				a.checkEmpty(c, len(all))
				return
			}
			a.checkDetail(c, shown, at)
		})
	})
}

// checkList is the page's left: how far the checks got, the kinds, and
// the findings.
func (a *App) checkList(c *ui.Context, all, shown []checkItem) {
	t := a.pal.theme
	k := &a.ck
	// The page has no top row: its heads hold the window.
	ui.Column(c).FillWidth().Padding(24, 20, 12, 20).Gap(12).Shrink(0).DragWindow().Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "检查").FontSize(fsTitle).FontWeight(700).Grow(1)
			a.checkProgress(c)
		})
		seg := ui.SegmentedBase(c, &k.tab, len(checkKinds))
		seg.Track.Label("检查分类").FillWidth().Padding(2).Gap(2).Radius(rControl).Border(1, t.Border).Children(func() {
			for i, kind := range checkKinds {
				n := 0
				for _, it := range all {
					if kind.kind == "" || kind.kind == it.f.Kind {
						n++
					}
				}
				s := seg.Segment(i).Grow(1).Center().Padding(4, 0).Radius(rControl).Label(kind.label)
				color := t.TextMuted
				if i == k.tab {
					s.Background(t.SurfacePressed)
					color = t.Text
				}
				s.Children(func() { ui.Textf(c, "%s %d", kind.label, n).FontSize(fsNote).TextColor(color).SingleLine() })
			}
		})
	})
	ui.Scroll(c).Key("checks").FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(0, 10, 12, 10).Gap(2).Children(func() {
			for _, it := range shown {
				sub := chapterLabel(it.ch)
				switch it.f.Kind {
				case workspace.FindConflict:
					sub += " ↔ 设定「" + entryTitle(it.entry) + "」"
				case workspace.FindName:
					sub += " · 应为「" + it.f.Name + "」"
				case workspace.FindNew:
					sub = it.f.Category + " · " + sub + "出现"
				}
				b := ui.ButtonBase(c).Key("ck-" + it.f.ID).Column().FillWidth().Gap(4).Padding(10).Radius(rControl).Label("检查-" + it.f.Title)
				if it.f.ID == k.shown {
					b.Background(t.SurfacePressed)
				} else if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				b.Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, it.f.Title).FontSize(fsBody).FontWeight(500).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
						ui.Text(c, kindLabel(it.f.Kind)).FontSize(fsSmall).TextColor(a.pal.faint).SingleLine().Shrink(0)
					})
					ui.Text(c, sub).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth()
				})
				if b.Clicked() {
					k.shown = it.f.ID
				}
			}
		})
	})
}

// checkProgress says how far the checks got, and checks the book again.
func (a *App) checkProgress(c *ui.Context) {
	t := a.pal.theme
	k := &a.ck
	if k.running != "" {
		left := len(k.queue)
		line := "正在查" + chapterLabel(a.checkNo(k.running))
		if left > 0 {
			line += fmt.Sprintf("，还有 %d 章", left)
		}
		ui.Text(c, line).FontSize(fsNote).TextColor(t.TextMuted).SingleLine()
		if ghost(c, "停下", ui.Color{}).Clicked() {
			k.queue = nil
			if k.cancel != nil {
				k.cancel()
			}
		}
		return
	}
	last := 0
	for id := range k.all {
		last = max(last, a.checkNo(id))
	}
	if last > 0 {
		ui.Text(c, "查到"+chapterLabel(last)).FontSize(fsNote).TextColor(t.TextMuted).SingleLine()
	}
	label := "再查一次"
	if last == 0 {
		label = "检查全书"
	}
	b := ghost(c, label, ui.Color{}).Label(label)
	if b.Clicked() {
		k.due = a.checkDue()
		switch {
		case !a.hasSetting():
			a.notice = "设定还是空的，没有可以对照的"
		case len(k.due) == 0:
			a.notice = "各章都查过了，正文没有再改"
		default:
			k.askOpen = true
		}
	}
	ui.PopoverBase(c, b, &k.askOpen, func(panel *ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(6, 0, 0, 0).Width(280).Radius(rCard).Padding(14).Gap(12).
			Background(a.pal.canvas).Border(1, t.Border)
		a.pal.pop.on(panel)
		ui.Textf(c, "有 %d 章没查过或改过", len(k.due)).FontSize(fsBody).FontWeight(600)
		ui.Textf(c, "每章用一次 AI，一共 %d 次。", len(k.due)).FontSize(fsNote).TextColor(t.TextMuted)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c)
			if a.pal.small(c, "取消", false).Clicked() {
				k.askOpen = false
			}
			if a.pal.small(c, "开始检查", true).Clicked() {
				k.askOpen = false
				for _, id := range k.due {
					a.queueCheck(id)
				}
			}
		})
	})
}

// checkNo is which chapter of the book node is, as the checks number them.
func (a *App) checkNo(node string) int {
	n, at := 0, 0
	a.ck.tree.Walk(func(nd *workspace.Node, _ int) {
		if !nd.Folder {
			n++
			if nd.ID == node {
				at = n
			}
		}
	})
	return at
}

// checkEmpty is the page's right with nothing to show.
func (a *App) checkEmpty(c *ui.Context, all int) {
	k := &a.ck
	words, how := "没有对不上的地方", "写完一章会自动检查，结果列在这里。"
	switch {
	case all > 0:
		words, how = "这一类没有要看的", ""
	case !a.hasSetting():
		words, how = "设定还是空的", "先在讨论里把设定定下来，才有可以对照的。"
	case len(k.all) == 0 && k.running == "":
		words, how = "还没检查过", "点左上的「检查全书」，或者写完一章自动检查。"
	case k.running != "":
		words, how = "正在检查…", "查完的结果会列在左边。"
	case k.when == checkOnDemand:
		how = "现在是「我点了才查」，要查时点「再查一次」。"
	}
	for id, why := range k.failed {
		how = chapterLabel(a.checkNo(id)) + "没查成：" + why
	}
	ui.Column(c).Fill().Center().Gap(6).Children(func() {
		ui.Text(c, words).FontSize(fsBody)
		if how != "" {
			ui.Text(c, how).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).LineHeight(lhBody)
		}
	})
}

// quoted is text in spans with its part, quote, as the design marks what
// is at odds; long text is cut around it.
func (a *App) quoted(text, quote string) []ui.Span {
	const around = 120
	i := strings.Index(text, quote)
	if quote == "" || i < 0 {
		return []ui.Span{{Text: text}}
	}
	before, after := []rune(text[:i]), []rune(text[i+len(quote):])
	if len(before) > around {
		before = append([]rune("…"), before[len(before)-around:]...)
	}
	if len(after) > around {
		after = append(after[:around], '…')
	}
	return []ui.Span{{Text: string(before)}, {Text: quote, Color: a.pal.theme.Text, Background: a.pal.theme.SurfacePressed}, {Text: string(after)}}
}

// checkCard is one side of the comparison: what it is, a link to it, and
// its words.
func (a *App) checkCard(c *ui.Context, head, link string, spans []ui.Span) (clicked bool) {
	t := a.pal.theme
	ui.Column(c).Grow(1).Shrink(1).Basis(0).Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, head).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
			if link != "" {
				b := ui.ButtonBase(c).Label(link).Shrink(0)
				b.Children(func() { ui.Text(c, link).FontSize(fsNote).TextColor(t.Text).SingleLine() })
				clicked = b.Clicked()
			}
		})
		ui.RichText(c, spans...).FontSize(fsRead).LineHeight(lhRead).TextColor(t.TextMuted).FillWidth().
			Padding(14, 16).Radius(rCard).Border(1, a.pal.line).Selectable()
	})
	return clicked
}

// paragraphOf is the paragraph of text that has quote, and which it is.
func paragraphOf(text, quote string) (string, int) {
	n := 0
	for _, p := range strings.Split(text, "\n") {
		if strings.TrimSpace(p) == "" {
			continue
		}
		n++
		if strings.Contains(p, quote) {
			return strings.TrimSpace(p), n
		}
	}
	return quote, 0
}

// checkDetail is the finding chosen: the prose beside the setting, what
// was found alike before, and what to do about it.
func (a *App) checkDetail(c *ui.Context, shown []checkItem, at int) {
	t := a.pal.theme
	k := &a.ck
	it := shown[at]
	if k.proseOf != it.node {
		k.prose, k.proseOf = "", it.node
		if ch, err := a.Store.ReadChapter(it.node); err == nil {
			k.prose = ch.Text
		}
	}
	next := func() { k.shown = shown[(at+1)%len(shown)].f.ID }
	toProse := func() { a.findInChapter(it.node, it.f.Quote) }
	toEntry := func() {
		a.findInChapter(it.node, it.f.Quote)
		a.doc.shown, a.doc.talk, a.doc.fore, a.doc.hidden = it.entry.ID, false, false, false
		a.savePrefs()
	}
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Shrink(0).DragWindow().Children(func() {
		ui.Text(c, it.f.Title).FontSize(fsDialog).FontWeight(600).Grow(1).Shrink(1)
		ui.Textf(c, "%d / %d", at+1, len(shown)).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
		if ghost(c, "下一条", ui.Color{}).Disabled(len(shown) < 2).Clicked() {
			next()
		}
	})
	ui.Scroll(c).Key("check-" + it.f.ID).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Gap(20).Children(func() {
			para, no := paragraphOf(k.prose, it.f.Quote)
			head := "正文 · " + chapterLabel(it.ch)
			if no > 0 {
				head += fmt.Sprintf(" 第 %d 段", no)
			}
			ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Start).Children(func() {
				if a.checkCard(c, head, "看原文", a.quoted(para, it.f.Quote)) {
					toProse()
				}
				switch it.f.Kind {
				case workspace.FindConflict:
					if a.checkCard(c, "设定 · "+entryTitle(it.entry), "看设定", a.quoted(it.entry.Text, it.f.Says)) {
						toEntry()
					}
				case workspace.FindName:
					if a.checkCard(c, "设定 · "+entryTitle(it.entry), "看设定", a.quoted(it.entry.Text, it.f.Name)) {
						toEntry()
					}
				case workspace.FindNew:
					text := it.f.Name
					if it.f.Text != "" {
						text += "：" + it.f.Text
					}
					a.checkCard(c, "可以加入设定 · "+it.f.Category, "", a.quoted(text, it.f.Name))
				}
			})
			if it.f.Kind != workspace.FindConflict {
				return
			}
			var alike []checkItem
			for _, o := range a.checkItems(true) {
				if o.f.Kind == workspace.FindConflict && o.f.Entry == it.f.Entry && o.f.ID != it.f.ID {
					alike = append(alike, o)
				}
			}
			if len(alike) == 0 {
				return
			}
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, "别的章也有类似的地方").FontSize(fsNote).TextColor(t.TextMuted)
				ui.Column(c).FillWidth().Children(func() {
					a.pal.hairline(c)
					for _, o := range alike {
						said := "还没处理"
						if o.f.Meant {
							said = "你标过「是故意的」"
						}
						ui.Row(c).FillWidth().Gap(12).Padding(8, 0).AlignItems(ui.Center).Children(func() {
							ui.Text(c, chapterLabel(o.ch)).FontSize(fsControl).TextColor(t.TextMuted).Width(56).SingleLine().Shrink(0)
							ui.Text(c, o.f.Quote).FontSize(fsControl).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
							ui.Text(c, said).FontSize(fsControl).TextColor(a.pal.faint).SingleLine().Shrink(0)
						})
						a.pal.hairline(c)
					}
				})
			})
		})
	})
	a.pal.hairline(c)
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		if ghost(c, "以后再说", ui.Color{}).Disabled(len(shown) < 2).Clicked() {
			next()
		}
		ui.Spacer(c)
		meant := func(label string) {
			if a.pal.secondary(c, label).Clicked() {
				if len(shown) > 1 {
					next()
				}
				a.setMeant(it)
			}
		}
		switch it.f.Kind {
		case workspace.FindConflict:
			meant("是故意的")
			if a.pal.secondary(c, "改设定").Clicked() {
				toEntry()
			}
			if a.pal.primary(c, "改正文").Clicked() {
				toProse()
			}
		case workspace.FindName:
			meant("是故意的")
			if a.pal.primary(c, "改正文").Clicked() {
				toProse()
			}
		case workspace.FindNew:
			meant("不用记")
			if a.pal.primary(c, "加入设定").Clicked() {
				a.takeFound(it)
			}
		}
	})
}

// takeFound adds what a check found new to the setting, as the author's.
func (a *App) takeFound(it checkItem) {
	text := it.f.Text
	if text == "" {
		text = it.f.Name
	}
	r, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
		Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: it.f.Category, Text: text, Names: []string{it.f.Name},
		Sources: []workspace.Source{{Chapter: it.node, Quote: it.f.Quote}},
	}}})
	if err == nil {
		err = a.refresh()
	}
	if err != nil {
		a.warn(err)
		return
	}
	a.flash[r.Ops[0].Entry] = time.Now()
	a.notice = "已加入设定"
}

// pageCheck is the page of what the checks found.
const pageCheck = 5
