package desk

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"writer/internal/textclip"
	"writer/workspace"
)

// When a chapter is checked against the setting, as the work's
// preferences keep it.
const (
	checkOnLeave  = ""       // as the author leaves a chapter changed
	checkOnDemand = "manual" // only when asked
	checkOnPause  = "always" // whenever the writing pauses
)

var checkWhens = []struct{ when, label, short, sub string }{
	{checkOnLeave, "写完一章后检查", "写完一章后检查", "推荐，每章调用一次 AI"},
	{checkOnDemand, "手动检查", "手动检查", "平时不调用 AI"},
	{checkOnPause, "边写边检查", "边写边检查", "停笔即检查，调用最多"},
}

// checks is the chapters waiting to be checked against the setting, one
// at a time in the background, what the checks found, and the page that
// shows it.
type checks struct {
	queue  workspace.CheckQueue
	runner checkRunner

	of   *workspace.Store // the work the queue is of
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

// checkItem is a finding with where it is.
type checkItem struct {
	node, title string
	here        bool // in the chapter open
	f           workspace.Finding
	entry       workspace.Entry
	also        []checkItem // a new name found in later chapters too, listed here once
}

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
	a.warn(a.book.ReloadChecks())
	a.warn(a.book.ReloadTree())
	k.doubtKey = ""
}

// hasSetting tells whether there is a setting to check prose against.
func (a *App) hasSetting() bool {
	return slices.ContainsFunc(a.book.Setting().Entries, func(e workspace.Entry) bool { return e.Place == workspace.PlaceBody })
}

// leaveCheck is called as the author leaves the chapter shown: what the
// prose no longer says is no longer found, and a chapter changed is
// checked again, when that is when to; it is not changed again until it
// is. A chapter
// about to be shown, its text not read yet, is no chapter left; with the
// AI off, the checks are left as they were.
func (a *App) leaveCheck() {
	w := &a.write
	k := &a.ck
	if w.node == "" || w.ed == nil || w.lost || a.Store == nil || !a.aiOn() {
		return
	}
	a.pruneStoredCheck(w.node, w.saved)
	changed := w.changed
	w.changed = false
	if !changed || k.when != checkOnLeave || !a.hasSetting() {
		return
	}
	k.queue.EnqueueStale(a.book.Checks(), w.node, w.saved, a.ai.MinCheck())
}

// pruneStoredCheck drops from chapter's check what its prose, text, no longer
// says and the author did not call meant.
func (a *App) pruneStoredCheck(chapter, text string) {
	k := &a.ck
	had, ok := a.book.Checks()[chapter]
	if !ok {
		return
	}
	had, changed := had.Prune(text)
	if !changed {
		return
	}
	if err := a.book.SaveCheck(chapter, had); err != nil {
		a.warn(err)
		return
	}
	k.doubtKey = ""
}

// pruneStoredChecks does so for every chapter with something found, as of the
// disk.
func (a *App) pruneStoredChecks() {
	for _, id := range workspace.PrunableChapters(a.book.Checks()) {
		if ch, err := a.Store.ReadChapter(id); err == nil {
			a.pruneStoredCheck(id, ch.Text)
		}
	}
}

// pollChecks takes in a check done, and starts the next.
func (a *App) pollChecks(c *ui.Context) {
	k := &a.ck
	if k.of != a.Store {
		k.of = a.Store
		k.shown, k.prose, k.proseOf, k.tab = "", "", "", 0
		k.queue.WorkChanged()
		a.loadChecks()
	}
	if d, ok := k.runner.finished(); ok {
		k.queue.Finished()
		if d.store == a.Store {
			a.saveCheckResult(d)
		}
	}
	id, ok := k.queue.Next()
	if !ok {
		return
	}
	pw, err := a.ai.Pen(a.Store)
	if err != nil {
		k.queue.CannotStart(id, err)
		return
	}
	k.runner.start(pw, a.Store, id, c.Services())
}

// saveCheckResult saves what a check found, with what the author called meant
// while it ran.
func (a *App) saveCheckResult(d checkDone) {
	k := &a.ck
	result, keep := k.queue.Keep(d.chapter, a.book.Checks()[d.chapter], d.check, d.err)
	if !keep {
		return
	}
	if err := a.book.SaveCheck(d.chapter, result); err != nil {
		k.queue.Fail(d.chapter, err)
		return
	}
	k.doubtKey = ""
}

// stopChecks drops the checks waiting and stops the one running, as the
// work closes or the AI is turned off.
func (a *App) stopChecks() {
	k := &a.ck
	k.runner.stop()
	k.queue.Stop()
}

// checkWhilePausing checks the chapter shown once the writing rests, when
// that is when to: each state of the prose at most once.
func (a *App) checkWhilePausing() {
	w := &a.write
	k := &a.ck
	if k.when != checkOnPause || !a.aiOn() || w.ed == nil || w.node == "" || w.lost || w.ed.Doc.Version() != w.savedVer || !a.hasSetting() {
		return
	}
	k.queue.AfterPause(w.node, w.saved, a.book.Checks()[w.node], a.ai.MinCheck(), w.ed.LastEdit(), time.Now())
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
// order: with meant, also what the author said is as meant. A new name the
// author left out of the setting once is left out in every chapter.
func (a *App) checkItems(meant bool) []checkItem {
	tree := a.book.Tree()
	live, open := a.liveText()
	left := map[string]bool{}
	for _, had := range a.book.Checks() {
		for _, f := range had.Findings {
			if f.Kind == workspace.FindNew && f.Meant {
				left[f.Name] = true
			}
		}
	}
	var out []checkItem
	o := tree.Order()
	tree.Walk(func(nd *workspace.Node, _ int) {
		if nd.Folder {
			return
		}
		here := open && nd.ID == a.write.node
		for _, f := range a.book.Checks()[nd.ID].Findings {
			if f.Meant && !meant || here && !strings.Contains(live, f.Quote) {
				continue
			}
			it := checkItem{node: nd.ID, title: nd.Title, here: here, f: f}
			switch f.Kind {
			case workspace.FindConflict:
				e, ok := a.book.Setting().Entry(f.Entry)
				e.Text, _, _ = e.Reading(o, nd.ID) // the chapter was checked against the entry as it reads there
				if !ok || f.Says != "" && !strings.Contains(e.Text, f.Says) {
					continue // the setting was changed
				}
				it.entry = e
			case workspace.FindName:
				for _, e := range a.book.Setting().Entries {
					if slices.Contains(e.Names, f.Name) {
						it.entry = e
					}
				}
				if it.entry.ID == "" {
					continue
				}
			case workspace.FindNew:
				if slices.ContainsFunc(a.book.Setting().Entries, func(e workspace.Entry) bool {
					return e.Place != workspace.PlaceDropped && slices.Contains(e.Names, f.Name)
				}) || left[f.Name] && !meant {
					continue // taken into the setting, or left out of it
				}
			}
			out = append(out, it)
		}
	})
	return out
}

// listedChecks is what the checks found as the author goes through it: a
// new name found in several chapters is one, where it first comes, since
// it is taken into the setting or left out once.
func (a *App) listedChecks() []checkItem {
	var out []checkItem
	first := map[string]int{}
	for _, it := range a.checkItems(false) {
		if it.f.Kind == workspace.FindNew {
			if i, ok := first[it.f.Name]; ok {
				out[i].also = append(out[i].also, it)
				continue
			}
			first[it.f.Name] = len(out)
		}
		out = append(out, it)
	}
	return out
}

// setMeant records that the author means what it found.
func (a *App) setMeant(it checkItem) {
	k := &a.ck
	had := a.book.Checks()[it.node]
	for i := range had.Findings {
		if had.Findings[i].ID == it.f.ID {
			had.Findings[i].Meant = true
		}
	}
	if err := a.book.SaveCheck(it.node, had); err != nil {
		a.warn(err)
		return
	}
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
	return workspace.DueChapters(a.book.Checks(), a.book.Tree(), texts, a.ai.MinCheck())
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
	for _, it := range a.listedChecks() {
		p := pendItem{key: "检查/" + it.node + "/" + it.f.Kind + "/" + it.f.Quote, text: it.f.Title, sub: it.title, node: it.node, here: it.here, check: it.f.ID}
		for _, o := range it.also {
			if o.here && !p.here { // a name found here too is of this chapter
				p.sub, p.node, p.here, p.check = o.title, o.node, true, o.f.ID
			}
		}
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
	const name = "本章"
	text, _ := a.liveText()
	had, ok := a.book.Checks()[w.node]
	switch {
	case k.queue.Running == w.node:
		return "正在检查" + name + "…", 0, false, ""
	case slices.Contains(k.queue.Pending, w.node):
		return name + "等待检查", 0, false, ""
	case k.queue.Failed[w.node] != "":
		line := "检查失败：" + k.queue.Failed[w.node]
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
const partNote = "后文超出模型的上下文上限，可在设置中「AI」配置里调整"

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
	all := a.listedChecks()
	var shown []checkItem
	for _, it := range all {
		if kind := checkKinds[k.tab].kind; kind == "" || kind == it.f.Kind {
			shown = append(shown, it)
		}
	}
	at := slices.IndexFunc(shown, func(it checkItem) bool {
		return it.f.ID == k.shown || slices.ContainsFunc(it.also, func(o checkItem) bool { return o.f.ID == k.shown })
	})
	if at < 0 {
		at = min(0, len(shown)-1)
	}
	if at >= 0 {
		k.shown = shown[at].f.ID
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
				sub := it.title
				switch it.f.Kind {
				case workspace.FindConflict:
					sub += " ↔ 设定「" + entryTitle(it.entry) + "」"
				case workspace.FindName:
					sub += " · 应为「" + it.f.Name + "」"
				case workspace.FindNew:
					sub = it.f.Category + " · 「" + sub + "」起出现"
					if len(it.also) > 0 {
						sub += fmt.Sprintf("，共 %d 章", len(it.also)+1)
					}
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
	if k.queue.Running != "" {
		left := len(k.queue.Pending)
		_, title := a.checkAt(k.queue.Running)
		line := "正在检查「" + title + "」"
		if left > 0 {
			line += fmt.Sprintf("，还有 %d 章", left)
		}
		ui.Text(c, line).FontSize(fsNote).TextColor(t.TextMuted).SingleLine()
		if a.pal.ghost(c, "停止", ui.Color{}).Clicked() {
			k.queue.Stop()
			k.runner.stop()
		}
		return
	}
	last, lastTitle, done, failed := 0, "", 0, 0
	for id := range a.book.Checks() {
		no, title := a.checkAt(id)
		if no > last {
			last, lastTitle = no, title
		}
		if _, bad := k.queue.Failed[id]; no > 0 && !bad {
			done++
		}
	}
	for id := range k.queue.Failed {
		if no, _ := a.checkAt(id); no > 0 {
			failed++
		}
	}
	switch {
	case failed > 0:
		// 已检查至 would read as all before it checked.
		ui.Textf(c, "已检查 %d 章，%d 章失败", done, failed).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
	case last > 0:
		ui.Text(c, "已检查至「"+lastTitle+"」").FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1)
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
			a.status.hint("设定为空，无可对照内容")
		case len(k.due) == 0:
			a.status.hint("各章均已检查，正文无新改动")
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
					a.ck.queue.Enqueue(id)
				}
			}
		})
	})
}

// checkAt is which chapter of the book node is, as the checks number
// them, and its title; a chapter no longer in the book is 0, 已删除的章.
func (a *App) checkAt(node string) (no int, title string) {
	n := 0
	title = "已删除的章"
	a.book.Tree().Walk(func(nd *workspace.Node, _ int) {
		if !nd.Folder {
			n++
			if nd.ID == node {
				no, title = n, nd.Title
			}
		}
	})
	return no, title
}

// checkEmpty is the page's right with nothing to show.
func (a *App) checkEmpty(c *ui.Context, all int) {
	k := &a.ck
	pic, words, how := icDone, "没有发现不一致", "写完一章后自动检查，结果显示在此处。"
	switch {
	case all > 0:
		words, how = "这一类没有待处理的", ""
	case !a.hasSetting():
		words, how = "设定为空", "请先在讨论中确定设定，才能对照检查。"
	case len(a.book.Checks()) == 0 && k.queue.Running == "":
		words, how = "尚未检查", "点击左上角「检查全书」，或写完一章后自动检查。"
	case k.queue.Running != "":
		words, how = "正在检查…", "检查结果将显示在左侧。"
	case k.when == checkOnDemand:
		how = "当前为「手动检查」，需要时点击「重新检查」。"
	}
	// One that failed is not told as one that found nothing; the first in
	// the book is named, so it is the same one every frame.
	if lost := a.firstFailed(); lost != "" && k.queue.Running == "" {
		_, title := a.checkAt(lost)
		pic, words, how = icAlert, "检查未完成", "「"+title+"」检查失败："+k.queue.Failed[lost]
	}
	a.pal.blank(c, pic, words, how, nil)
}

// firstFailed is the first chapter in the book whose check failed, or "".
func (a *App) firstFailed() string {
	first := ""
	a.book.Tree().Walk(func(nd *workspace.Node, _ int) {
		if _, ok := a.ck.queue.Failed[nd.ID]; ok && first == "" && !nd.Folder {
			first = nd.ID
		}
	})
	return first
}

// quoted is text in spans with its part, quote, as the design marks what
// is at odds; long text is cut around it.
func (a *App) quoted(text, quote string) []ui.Span {
	const around = 120
	i := strings.Index(text, quote)
	if quote == "" || i < 0 {
		return []ui.Span{{Text: text}}
	}
	before, after := text[:i], text[i+len(quote):]
	return []ui.Span{{Text: textclip.Tail(before, around)}, {Text: quote, Color: a.pal.theme.Text, Background: a.pal.theme.SurfacePressed}, {Text: textclip.Clip(after, around)}}
}

// checkCard is one side of the comparison: what it is, a link to it, and
// its words.
func (a *App) checkCard(c *ui.Context, head, link string, spans []ui.Span) (clicked bool) {
	t := a.pal.theme
	ui.Column(c).Grow(1).Shrink(1).Basis(0).Gap(6).Children(func() {
		// As tall without the link as with it, so cards side by side line up.
		ui.Row(c).FillWidth().MinHeight(22).Gap(8).AlignItems(ui.Center).Children(func() {
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

// overlap tells whether two parts of text are in one sentence of it:
// findings on one entry are alike where they hold to the same of it, not
// where one is of her fear of the dark and the other of her left hand. A
// part not given, or not found, may be any.
func overlap(text, x, y string) bool {
	i, j := strings.Index(text, x), strings.Index(text, y)
	if x == "" || y == "" || i < 0 || j < 0 {
		return true
	}
	s1, e1 := sentences(text, i, i+len(x))
	s2, e2 := sentences(text, j, j+len(y))
	return s1 < e2 && s2 < e1
}

// sentences widens text[from:to] to the sentences it is in.
func sentences(text string, from, to int) (int, int) {
	const ends = "。！？；\n"
	if k := strings.LastIndexAny(text[:from], ends); k >= 0 {
		_, n := utf8.DecodeRuneInString(text[k:])
		from = k + n
	} else {
		from = 0
	}
	if k := strings.IndexAny(text[to:], ends); k >= 0 {
		to += k
	} else {
		to = len(text)
	}
	return from, to
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
		a.doc.route.openEntry(it.entry.ID)
		a.doc.tab, a.doc.hidden = paneSetting, false
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
			head := "正文 · " + it.title
			if no > 0 {
				head += fmt.Sprintf(" · 第 %d 段", no)
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
			head, alike := "其他章节中也出现", it.also
			if it.f.Kind == workspace.FindConflict {
				head = "其他章节的类似情况"
				for _, o := range a.checkItems(true) {
					if o.f.Kind == workspace.FindConflict && o.f.Entry == it.f.Entry && o.f.ID != it.f.ID && overlap(it.entry.Text, it.f.Says, o.f.Says) {
						alike = append(alike, o)
					}
				}
			}
			if len(alike) == 0 {
				return
			}
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, head).FontSize(fsNote).TextColor(t.TextMuted)
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					for _, o := range alike {
						ui.Row(c).FillWidth().Gap(12).Padding(8, 12).Radius(rControl).Background(t.SurfaceHover).AlignItems(ui.Center).Children(func() {
							ui.Text(c, o.title).FontSize(fsControl).TextColor(t.TextMuted).SingleLine().Shrink(0)
							ui.Text(c, o.f.Quote).FontSize(fsControl).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
							if o.f.Kind == workspace.FindConflict {
								said := "尚未处理"
								if o.f.Meant {
									said = "已标为「无需修改」"
								}
								ui.Text(c, said).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
							}
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
	r, err := a.book.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
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
	a.status.did("已加入设定")
}
