package desk

import (
	"fmt"
	"maps"
	"slices"

	"github.com/egoist/mygo/ui"

	"writer/editor"
	"writer/workspace"
)

// pendShown is how many items a group lists before it is opened.
const pendShown = 3

// pendItem is a thing for the author to look at: where it is, and the key
// ignoring it records.
type pendItem struct {
	key, text, sub string
	node           string
	start, end     int
	here           bool   // in the chapter open
	retry          bool   // a backup failed: choosing it backs up again
	fore           string // the foreshadowing it is about
	check          string // the finding of a check it is
}

// pendGroup is the items of one kind.
type pendGroup struct {
	name  string
	color ui.Color
	items []pendItem
}

// scanMarks reads the marks of the book's chapters, as of the disk.
func (a *App) scanMarks() {
	a.warn(a.book.ReloadMarks())
}

// pendingGroups gathers what is pending and not ignored, by kind: what the
// checks against the setting found, the foreshadowings lost or waiting too long, the open chapter's as it is
// being written, and a backup that failed.
func (a *App) pendingGroups() []pendGroup {
	w := &a.write
	var items []pendItem
	for _, f := range a.foreshadows() {
		it := pendItem{node: f.Node, start: f.Start, end: f.End, here: f.Here, sub: f.Title, fore: f.ID}
		switch st := f.Status(); {
		case st == workspace.ForeLost:
			it.key = "伏笔/失去锚点/" + f.Node + "/" + f.Quote
			it.text = fmt.Sprintf("「%s」原句已删", clip(foreName(f), 16))
		case f.Late(a.write.lateAfter):
			it.key = "伏笔/没回收/" + f.Node + "/" + f.Quote
			it.text = fmt.Sprintf("「%s」%d 章未回收", clip(foreName(f), 16), f.Gap)
			it.sub += " 埋下"
		default:
			continue
		}
		if f.Here {
			it.sub += " · 本章"
		}
		if !w.ignored[it.key] {
			items = append(items, it)
		}
	}
	slices.SortStableFunc(items, func(x, y pendItem) int {
		if x.here == y.here {
			return 0
		} else if x.here {
			return -1
		}
		return 1
	})
	var groups []pendGroup
	if g, ok := a.checkPending(); ok {
		groups = append(groups, g)
	}
	if len(items) > 0 {
		groups = append(groups, pendGroup{name: "伏笔", color: a.pal.faint, items: items})
	}
	if g, ok := a.backupPending(); ok {
		groups = append(groups, g)
	}
	return groups
}

// liveMarks is the marks of the text being written, each quoting what it
// covers now.
func liveMarks(d *editor.Doc) []workspace.ChapterMark {
	ms := storedMarks(d.Marks())
	for i := range ms {
		if !ms[i].Orphaned {
			ms[i].Quote = d.Slice(ms[i].Start, ms[i].End)
		}
	}
	return ms
}

// pendingButton shows how many things are pending, opening their list;
// it is not there while there are none.
func (a *App) pendingButton(c *ui.Context) {
	w := &a.write
	groups := a.pendingGroups()
	total, here := 0, 0
	for _, g := range groups {
		total += len(g.items)
		for _, it := range g.items {
			if it.here {
				here++
			}
		}
	}
	if total == 0 {
		w.pendOpen = false
		return
	}
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().Height(hControlSm).Margin(0, 0, 0, 6).Gap(6).AlignItems(ui.Center).Padding(0, 8).Radius(rControl).Shrink(0).
		Border(1, t.Border).Label("待处理")
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		dot(c, a.pal.amber, 6)
		ui.Textf(c, "待处理 %d", total).FontSize(fsNote).SingleLine()
	})
	if b.Clicked() {
		w.pendOpen = !w.pendOpen
	}
	ui.PopoverBase(c, b, &w.pendOpen, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(4, 0, 0, 0).Width(360).Radius(rCard).Clip().
			Background(a.pal.raised).Border(1, t.Border)
		a.pal.pop.on(panel)
		a.pendingList(c, groups, total, here)
	})
}

// pendingList is the list the button opens: the things pending, all of
// them or the open chapter's, by kind.
func (a *App) pendingList(c *ui.Context, groups []pendGroup, total, here int) {
	t := a.pal.theme
	w := &a.write
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(12, 14, 8, 14).Children(func() {
		ui.Text(c, "待处理").FontSize(fsBody).FontWeight(600).Grow(1)
		a.pal.segments(c, &w.pendHere, "待处理范围", []string{fmt.Sprintf("全部 %d", total), fmt.Sprintf("本章 %d", here)})
	})
	var shown []pendItem
	ui.Box(c).FillWidth().Height(1).Background(a.pal.line)
	ui.Scroll(c.Key("pending")).FillWidth().MaxHeight(520).Children(func() {
		for _, g := range groups {
			items := g.items
			if w.pendHere == 1 {
				items = slices.DeleteFunc(slices.Clone(items), func(it pendItem) bool { return !it.here })
			}
			if len(items) == 0 {
				continue
			}
			shown = append(shown, items...)
			ui.Column(c).FillWidth().Padding(0, 0, 6, 0).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(10, 20, 4, 20).Children(func() {
					dot(c, g.color, 6)
					ui.Text(c, g.name).FontSize(fsNote).TextColor(t.TextMuted)
					ui.Textf(c, "%d", len(items)).FontSize(fsNote).TextColor(a.pal.faint)
				})
				open := w.pendMore[g.name]
				for i, it := range items {
					if i == pendShown && !open {
						break
					}
					a.pendingRow(c, it)
				}
				if len(items) > pendShown {
					label := fmt.Sprintf("还有 %d 条", len(items)-pendShown)
					if open {
						label = "收起"
					}
					more := ui.ButtonBase(c).Row().Justify(ui.Start).Margin(0, 8).Padding(8, 12).Label(g.name + label)
					color := t.TextMuted
					if more.Hovered() {
						color = t.Text
					}
					more.Children(func() { ui.Text(c, label).FontSize(fsNote).TextColor(color) })
					if more.Clicked() {
						if w.pendMore == nil {
							w.pendMore = map[string]bool{}
						}
						w.pendMore[g.name] = !open
					}
				}
			})
		}
		if len(shown) == 0 {
			ui.Text(c, "本章没有待处理项").FontSize(fsControl).TextColor(t.TextMuted).Padding(20)
		}
	})
	ui.Box(c).FillWidth().Height(1).Background(a.pal.line)
	ui.Row(c).FillWidth().Padding(12, 20).Children(func() {
		ui.Spacer(c)
		all := ui.ButtonBase(c).Label("全部忽略").Disabled(len(shown) == 0)
		color := t.TextMuted
		if all.Hovered() {
			color = t.Text
		}
		all.Children(func() { ui.Text(c, "全部忽略").FontSize(fsNote).TextColor(color) })
		if all.Clicked() {
			a.ignore(shown)
		}
	})
}

// pendingRow is one thing pending; choosing it goes to where it is.
func (a *App) pendingRow(c *ui.Context, it pendItem) {
	t := a.pal.theme
	r := ui.ButtonBase(c).Row().Gap(12).AlignItems(ui.Center).Margin(0, 8).Padding(10, 12).Radius(rControl).Label(it.text)
	if r.Hovered() {
		r.Background(t.SurfaceHover)
	}
	r.Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, it.text).FontSize(fsBody).LineHeight(lhBody).MaxLines(2).Ellipsis("…").FillWidth()
			ui.Text(c, it.sub).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth()
		})
		glyph(c, icChevron, iconSm, a.pal.faint)
	})
	if r.Clicked() && it.retry {
		a.write.pendOpen = false
		a.runBackup(c.Services())
	} else if r.Clicked() {
		a.goTo(it)
	}
}

// goTo shows where a pending thing is: the text a mark covers, selected.
func (a *App) goTo(it pendItem) {
	w := &a.write
	w.pendOpen = false
	if it.check != "" {
		a.openCheck(it.check)
		return
	}
	if w.node != it.node {
		a.openNode(it.node)
	}
	if w.node == it.node && w.ed != nil {
		w.ed.Select(it.start, it.end)
		w.ed.Focus()
	}
	if it.fore != "" && !a.doc.hidden {
		a.showFore(it.fore)
	}
}

// ignore puts items out of the list for good.
func (a *App) ignore(items []pendItem) {
	w := &a.write
	if w.ignored == nil {
		w.ignored = map[string]bool{}
	}
	for _, it := range items {
		w.ignored[it.key] = true
	}
	a.savePrefs()
}

// ignoredKeys is what is ignored, in order to be kept.
func (w *writeView) ignoredKeys() []string { return slices.Sorted(maps.Keys(w.ignored)) }
