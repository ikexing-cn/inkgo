package desk

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
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
	{checkOnLeave, "写完一章后检查", "写完一章后检查", "推荐，每章调用一次 AI"},
	{checkOnDemand, "手动检查", "手动检查", "平时不调用 AI"},
	{checkOnPause, "边写边检查", "边写边检查", "停笔即检查，调用最多"},
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
	t, err := a.Store.Chapters()
	a.warn(err)
	if err == nil {
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
// and a chapter changed is checked again, when that is when to. A chapter
// about to be shown, its text not read yet, is no chapter left; with the
// AI off, the checks are left as they were.
func (a *App) leaveCheck() {
	w := &a.write
	k := &a.ck
	if w.node == "" || w.ed == nil || w.lost || a.Store == nil || !a.aiOn() {
		return
	}
	a.pruneCheck(w.node, w.saved)
	if !w.changed || k.when != checkOnLeave || !a.hasSetting() || workspace.Count(w.saved) < a.ai.MinCheck() {
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
	pw, err := a.ai.Pen(a.Store)
	if err != nil {
		k.failed[id] = err.Error()
		k.queue = nil
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	k.running, k.cancel = id, cancel
	store, sv := a.Store, c.Services()
	go func() {
		check, err := pw.Check(ctx, id)
		if errors.Is(ctx.Err(), context.Canceled) {
			err = context.Canceled // stopped, not failed
		}
		cancel()
		k.done <- checkDone{chapter: id, store: store, check: check, err: err}
		sv.Invalidate()
	}()
}

// keepCheck saves what a check found, with what the author called meant
// while it ran.
func (a *App) keepCheck(d checkDone) {
	k := &a.ck
	if errors.Is(d.err, context.Canceled) {
		return // stopped: the chapter stands as it did
	}
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

// stopChecks drops the checks waiting and stops the one running, as the
// work closes or the AI is turned off.
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
	if k.when != checkOnPause || !a.aiOn() || w.ed == nil || w.node == "" || w.lost || w.ed.Doc.Version() != w.savedVer ||
		time.Since(w.ed.LastEdit()) < checkPause || !a.hasSetting() {
		return
	}
	hash := workspace.ProseHash(w.saved)
	if k.tried[w.node] == hash || k.all[w.node].Of == hash || workspace.Count(w.saved) < a.ai.MinCheck() {
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
	texts, err := a.Store.Texts() // the book in one pass
	if err != nil {
		a.warn(err)
		return nil
	}
	var due []string
	a.ck.tree.Walk(func(nd *workspace.Node, _ int) {
		text, ok := texts[nd.ID]
		if nd.Folder || !ok || workspace.Count(text) < a.ai.MinCheck() {
			return
		}
		if had, ok := a.ck.all[nd.ID]; !ok || had.Stale(text) {
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
	g := pendGroup{name: "设定不一致", color: a.pal.amber}
	if !a.aiOn() {
		return g, false
	}
	for _, it := range a.checkItems(false) {
		p := pendItem{key: "检查/" + it.node + "/" + it.f.Kind + "/" + it.f.Quote, text: it.f.Title, sub: chapterLabel(it.ch), node: it.node, here: it.here, check: it.f.ID}
		if it.f.Kind == workspace.FindNew {
			p.text = fmt.Sprintf("「%s」尚未加入设定", it.f.Name)
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
// chapter shown, while the AI is on.
func (a *App) markDoubts() {
	w := &a.write
	k := &a.ck
	text, ok := a.liveText()
	if !ok {
		return
	}
	items := slices.DeleteFunc(a.checkItems(false), func(it checkItem) bool { return !it.here || !a.aiOn() })
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
// whether there is something of it to look at, and what more there is to
// say of the line, if anything.
func (a *App) checkState() (line string, found int, can bool, tip string) {
	w := &a.write
	k := &a.ck
	for _, it := range a.checkItems(false) {
		if it.here {
			found++
		}
	}
	name := "本章"
	if n := a.chapterNo(w.node); n > 0 {
		name = chapterLabel(n)
	}
	text, _ := a.liveText()
	had, ok := k.all[w.node]
	switch {
	case k.running == w.node:
		return "正在检查" + name + "…", 0, false, ""
	case slices.Contains(k.queue, w.node):
		return name + "等待检查", 0, false, ""
	case k.failed[w.node] != "":
		line := "检查失败：" + k.failed[w.node]
		return line, 0, true, line // what went wrong is long: in full on hover
	case !a.hasSetting():
		return "设定为空，无可对照内容", 0, false, ""
	case workspace.Count(text) < a.ai.MinCheck():
		return fmt.Sprintf("满 %d 字后可检查", a.ai.MinCheck()), 0, false, ""
	case !ok:
		return name + "尚未检查", 0, true, ""
	case found > 0 && had.Stale(text):
		return fmt.Sprintf("%s有 %d 处待处理，正文已再次修改", name, found), found, true, ""
	case found > 0 && had.Read > 0:
		return fmt.Sprintf("%s仅检查前 %d 字，有 %d 处待处理", name, had.Read, found), found, false, ""
	case found > 0:
		return fmt.Sprintf("%s检查完成，有 %d 处待处理", name, found), found, false, ""
	case had.Stale(text):
		return name + "已修改，尚未重新检查", 0, true, ""
	case had.Read > 0:
		// The rest was not looked at: not to be told as nothing found.
		return fmt.Sprintf("%s仅检查前 %d 字，未发现不一致", name, had.Read), 0, false, partNote
	}
	return name + "检查完成，未发现不一致", 0, false, ""
}

// partNote says why the AI was given only the start of a chapter, and what
// to do about it, where a line says it was.
const partNote = "后文超出上下文预算，可在设置中提高预算"

// footBar is the line at the foot of the writing page: of a chapter, with
// the AI on, where its check stands and when chapters are checked; at the
// right, the words.
func (a *App) footBar(c *ui.Context, chapter bool) {
	t := a.pal.theme
	w := &a.write
	k := &a.ck
	a.pal.hairline(c)
	foot := ui.Row(c).FillWidth().Height(hRow).Gap(8).Padding(0, 24).AlignItems(ui.Center).Shrink(0)
	roomy := foot.Bounds().W == 0 || foot.Bounds().W >= 600 // the counts give way to the check
	wide := foot.Bounds().W == 0 || foot.Bounds().W >= 760  // and today's to the book's
	foot.Children(func() {
		if !chapter || !a.aiOn() {
			ui.Spacer(c)
			a.wordCount(c, roomy, wide)
			return
		}
		line, found, can, tip := a.checkState()
		if found > 0 {
			b := a.pal.ghost(c, line, ui.Color{}).Label("查看检查结果").Tooltip("前往「检查」页逐条查看").Shrink(1000)
			if b.Clicked() {
				for _, it := range a.checkItems(false) {
					if it.here {
						a.openCheck(it.f.ID)
						break
					}
				}
			}
		} else {
			said := ui.Text(c, line).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1000)
			if tip != "" {
				said.Tooltip(tip)
			}
		}
		if can {
			b := ui.ButtonBase(c).Label("立即检查").Tooltip("调用一次 AI，对照设定检查本章").Shrink(0)
			color := t.TextMuted
			if b.Hovered() {
				color = t.Text
			}
			b.Children(func() { ui.Text(c, "立即检查").FontSize(fsNote).TextColor(color).Underline().SingleLine() })
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
		b := ui.ButtonBase(c).Label("检查时机").Shrink(0)
		color := t.TextMuted
		if b.Hovered() {
			color = t.Text
		}
		b.Children(func() { ui.Text(c, short+" ▴").FontSize(fsNote).TextColor(color).SingleLine() })
		if b.Clicked() {
			k.whenOpen = !k.whenOpen
		}
		ui.PopoverBase(c, b, &k.whenOpen, func(panel ui.Element) {
			panel.AttachTo(b, ui.AnchorTopLeft, ui.AnchorBottomLeft).Margin(0, 0, 4, 0).Width(270).Radius(rCard).Padding(10, 6, 6, 6).Gap(4).
				Background(a.pal.raised).Border(1, t.Border)
			a.pal.pop.on(panel)
			ui.Text(c, "检查时机").FontSize(fsBody).FontWeight(600).Padding(0, 8, 4, 8)
			ui.Column(c).FillWidth().Gap(2).Children(func() { a.whenRows(c) })
		})
		ui.Spacer(c)
		a.wordCount(c, roomy, wide)
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
		b := ui.ButtonBase(c).Row().FillWidth().Gap(10).Padding(6, 8).Radius(rControl).AlignItems(ui.Start).Justify(ui.Start).Label(o.label)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
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
	{"", "全部"}, {workspace.FindConflict, "不一致"}, {workspace.FindName, "名字有误"}, {workspace.FindNew, "新出现"},
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
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(360).Shrink(0).FillHeight().Children(func() { a.checkList(c, all, shown) })
		ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
		ui.Column(c).Grow(1).Shrink(1).FillHeight().Padding(32, 32, 20, 32).Gap(20).Children(func() {
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
	ui.Column(c).FillWidth().Padding(14, 20, 10, 20).Gap(10).Shrink(0).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "检查").FontSize(fsTitle).FontWeight(700).Grow(1)
			a.checkProgress(c)
		})
		names := make([]string, len(checkKinds))
		for i, kind := range checkKinds {
			n := 0
			for _, it := range all {
				if kind.kind == "" || kind.kind == it.f.Kind {
					n++
				}
			}
			names[i] = fmt.Sprintf("%s %d", kind.label, n)
		}
		ui.Row(c).Children(func() { a.pal.segments(c, &k.tab, "检查分类", names) })
	})
	ui.Scroll(c.Key("checks")).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(0, 14, 12, 14).Gap(2).Children(func() {
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
				b := ui.ButtonBase(c.Key("ck-"+it.f.ID)).Row().FillWidth().AlignItems(ui.Center).Gap(10).Padding(8, 10).Radius(rControl).Label("检查-" + it.f.Title)
				if it.f.ID == k.shown {
					b.Background(t.SurfacePressed)
				} else if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				b.Children(func() {
					ui.Column(c).Grow(1).Shrink(1).Basis(0).Children(func() {
						ui.Text(c, it.f.Title).FontSize(fsBody).LineHeight(lhBody).SingleLine().Ellipsis("…").FillWidth()
						ui.Text(c, sub).FontSize(fsNote).LineHeight(lhBody).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth()
					})
					ui.Text(c, kindLabel(it.f.Kind)).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
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
		line := "正在检查" + chapterLabel(a.checkNo(k.running))
		if left > 0 {
			line += fmt.Sprintf("，还有 %d 章", left)
		}
		ui.Text(c, line).FontSize(fsNote).TextColor(t.TextMuted).SingleLine()
		if a.pal.ghost(c, "停止", ui.Color{}).Clicked() {
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
		ui.Text(c, "已检查至"+chapterLabel(last)).FontSize(fsNote).TextColor(t.TextMuted).SingleLine()
	}
	label := "重新检查"
	if last == 0 {
		label = "检查全书"
	}
	b := a.pal.ghost(c, label, ui.Color{}).Label(label)
	if b.Clicked() {
		k.due = a.checkDue()
		switch {
		case !a.hasSetting():
			a.notice = "设定为空，无可对照内容"
		case len(k.due) == 0:
			a.notice = "各章均已检查，正文无新改动"
		default:
			k.askOpen = true
		}
	}
	ui.PopoverBase(c, b, &k.askOpen, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(6, 0, 0, 0).Width(280).Radius(rCard).Padding(14).Gap(12).
			Background(a.pal.canvas).Border(1, t.Border)
		a.pal.pop.on(panel)
		ui.Textf(c, "%d 章未检查或有改动", len(k.due)).FontSize(fsBody).FontWeight(600)
		ui.Textf(c, "每章调用一次 AI，共 %d 次。", len(k.due)).FontSize(fsNote).TextColor(t.TextMuted)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c)
			if a.pal.secondary(c, "取消").Clicked() {
				k.askOpen = false
			}
			if a.pal.primary(c, "开始检查").Clicked() {
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
	words, how := "没有发现不一致", "写完一章后自动检查，结果显示在此处。"
	switch {
	case all > 0:
		words, how = "这一类没有待处理的", ""
	case !a.hasSetting():
		words, how = "设定为空", "请先在讨论中确定设定，才能对照检查。"
	case len(k.all) == 0 && k.running == "":
		words, how = "尚未检查", "点击左上角「检查全书」，或写完一章后自动检查。"
	case k.running != "":
		words, how = "正在检查…", "检查结果将显示在左侧。"
	case k.when == checkOnDemand:
		how = "当前为「手动检查」，需要时点击「重新检查」。"
	}
	for id, why := range k.failed {
		how = chapterLabel(a.checkNo(id)) + "检查失败：" + why
	}
	a.pal.blank(c, icDone, words, how, nil)
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
	ui.Column(c).Grow(1).Shrink(1).Basis(0).Gap(6).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, head).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
			if link != "" {
				clicked = a.pal.ghost(c, link, ui.Color{}).Clicked()
			}
		})
		ui.RichText(c, spans...).FontSize(fsRead).LineHeight(lhRead).TextColor(t.TextMuted).FillWidth().
			Padding(12, 14).Radius(rCard).Background(a.pal.side).Border(1, a.pal.line).Selectable()
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
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, it.f.Title).FontSize(fsDialog).FontWeight(700).Grow(1).Shrink(1)
		ui.Textf(c, "%d / %d", at+1, len(shown)).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
		if a.pal.ghost(c, "下一条", ui.Color{}).Disabled(len(shown) < 2).Clicked() {
			next()
		}
	})
	ui.Scroll(c.Key("check-" + it.f.ID)).FillWidth().Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			para, no := paragraphOf(k.prose, it.f.Quote)
			head := "正文 · " + chapterLabel(it.ch)
			if no > 0 {
				head += fmt.Sprintf(" 第 %d 段", no)
			}
			ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Start).Children(func() {
				if a.checkCard(c, head, "查看原文", a.quoted(para, it.f.Quote)) {
					toProse()
				}
				switch it.f.Kind {
				case workspace.FindConflict:
					if a.checkCard(c, "设定 · "+entryTitle(it.entry), "查看设定", a.quoted(it.entry.Text, it.f.Says)) {
						toEntry()
					}
				case workspace.FindName:
					if a.checkCard(c, "设定 · "+entryTitle(it.entry), "查看设定", a.quoted(it.entry.Text, it.f.Name)) {
						toEntry()
					}
				case workspace.FindNew:
					text := it.f.Name
					if it.f.Text != "" {
						text += "：" + it.f.Text
					}
					a.checkCard(c, "可加入设定 · "+it.f.Category, "", a.quoted(text, it.f.Name))
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
				ui.Text(c, "其他章节的类似情况").FontSize(fsNote).TextColor(t.TextMuted)
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					for _, o := range alike {
						said := "尚未处理"
						if o.f.Meant {
							said = "已标为「无需修改」"
						}
						ui.Row(c).FillWidth().Gap(12).Padding(8, 12).Radius(rControl).Background(t.SurfaceHover).AlignItems(ui.Center).Children(func() {
							ui.Text(c, chapterLabel(o.ch)).FontSize(fsControl).TextColor(t.TextMuted).SingleLine().Shrink(0)
							ui.Text(c, o.f.Quote).FontSize(fsControl).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
							ui.Text(c, said).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
						})
					}
				})
			})
		})
	})
	a.pal.hairline(c)
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		if a.pal.ghost(c, "稍后再说", ui.Color{}).Disabled(len(shown) < 2).Clicked() {
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
			meant("无需修改")
			if a.pal.secondary(c, "修改设定").Clicked() {
				toEntry()
			}
			if a.pal.primary(c, "修改正文").Clicked() {
				toProse()
			}
		case workspace.FindName:
			meant("无需修改")
			if a.pal.primary(c, "修改正文").Clicked() {
				toProse()
			}
		case workspace.FindNew:
			meant("忽略")
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
