package desk

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/locales"
	"writer/workspace"
)

// pendShown is how many items a group lists before it is opened.
const pendShown = 3

// pendItem is a thing for the author to look at: where it is, and the key
// ignoring it records.
type pendItem struct {
	key, text, sub string
	here           bool // in the chapter open
	open           func(ui.Services)
}

// pendGroup is the items of one kind.
type pendGroup struct {
	id    string // the same in every language: what pendMore is kept by
	name  string
	color ui.Color
	items []pendItem
}

// scanMarks reads the marks of the book's chapters, as of the disk.
func (a *App) scanMarks() {
	a.warn(a.book.ReloadMarks())
}

// pendingBackupKey is what the backup failure in the list shows: its
// settings, its failure, and the minute its last-backup line says.
type pendingBackupKey struct {
	ok      bool
	dir     string
	last    time.Time
	failed  bool
	trouble string
	minute  int64
}

// pendingKey is everything pendingGroups reads: the book and the open
// chapter, the AI, the late-after setting, the ignored keys, the backup
// failure and the colours.
type pendingKey struct {
	locale  string
	book    *book.Book
	version int
	ed      *editor.Editor
	doc     int
	node    string
	live    bool
	ai      bool
	late    int
	ignored int
	backup  pendingBackupKey
	pal     *kit.Palette
}

func (a *App) pendingKey() pendingKey {
	w := &a.write
	doc := 0
	if w.cur().ed != nil {
		doc = w.cur().ed.Doc.Version()
	}
	b := pendingBackupKey{}
	if s, ok := a.backupSettings(); ok {
		b.ok, b.dir, b.last, b.failed = true, s.Dir, s.Last, a.bk.policy.Failed()
		if b.failed {
			b.trouble, _, _ = a.bk.policy.Trouble(s.Dir)
			b.minute = time.Now().Unix() / 60
		}
	}
	return pendingKey{i18n.Locale(), a.book, a.book.Version(), w.cur().ed, doc, w.node, a.liveChapter(),
		a.aiOn(), w.lateAfter, w.ignoredVer, b, a.pal}
}

// pendingGroups gathers what is pending and not ignored, by kind: what the
// checks against the setting found, the foreshadowings lost or waiting too long, the open chapter's as it is
// being written, and a backup that failed.
func (a *App) pendingGroups() []pendGroup {
	return a.write.pending.get(a.pendingKey(), a.buildPendingGroups)
}

func (a *App) buildPendingGroups() []pendGroup {
	w := &a.write
	var items []pendItem
	for _, f := range a.foreshadows() {
		it := pendItem{here: f.Here, sub: f.Title}
		switch st := f.Status(); {
		case st == workspace.ForeLost:
			it.key = "伏笔/失去锚点/" + f.Node + "/" + f.Quote
			it.text = locales.StatusQuoteGone(kit.Clip(foreName(f), 16))
		case f.Late(a.write.lateAfter):
			it.key = "伏笔/没回收/" + f.Node + "/" + f.Quote
			it.text = locales.StatusUnpaid(kit.Clip(foreName(f), 16), f.Gap)
			it.sub = locales.StatusPlantedIn(it.sub)
		default:
			continue
		}
		if f.Here {
			it.sub += locales.StatusHereTag()
		}
		it.open = func(_ ui.Services) {
			if w.node != f.Node {
				a.openNode(f.Node)
			}
			if w.node == f.Node && w.cur().ed != nil {
				w.cur().ed.Select(f.Start, f.End)
				w.cur().ed.Focus()
			}
			if f.ID != "" && !a.doc.hidden {
				a.showFore(f.ID)
			}
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
		groups = append(groups, pendGroup{id: "marks", name: locales.StatusGroupMarks(), color: a.pal.Faint, items: items})
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
	t := a.pal.Theme
	b := ui.ButtonBase(c).Row().Height(kit.HControlSm).Gap(6).AlignItems(ui.Center).Padding(0, 8).Radius(kit.RControl).Shrink(1).MinWidth(40).
		Border(1, t.Border).Label(locales.StatusPending()).Tooltip(locales.StatusPendingCount(total)).KeepFocus()
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		kit.Dot(c, a.pal.Amber, 6)
		ui.Text(c, locales.StatusPendingCount(total)).FontSize(kit.FsNote).SingleLine().Ellipsis("…").Shrink(1)
	})
	if b.Clicked() {
		w.pendOpen = !w.pendOpen
	}
	kit.Popover(c, b, &w.pendOpen, 1, 0, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(4, 0, 0, 0).Width(360).Radius(kit.RCard).Clip().
			Background(a.pal.Raised).Border(1, t.Border)
		a.pal.Pop.On(panel)
		a.pendingList(c, groups, total, here)
	})
}

// pendingList is the list the button opens: the things pending, all of
// them or the open chapter's, by kind.
func (a *App) pendingList(c *ui.Context, groups []pendGroup, total, here int) {
	t := a.pal.Theme
	w := &a.write
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(12, 14, 8, 14).Children(func() {
		ui.Text(c, locales.StatusPending()).FontSize(kit.FsBody).FontWeight(600).Grow(1)
		a.pal.Segments(c, &w.pendHere, locales.StatusPendingScope(), []string{locales.StatusAllCount(total), locales.StatusHereCount(here)})
	})
	var shown []pendItem
	ui.Box(c).FillWidth().Height(1).Background(a.pal.Line)
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
					kit.Dot(c, g.color, 6)
					ui.Text(c, g.name).FontSize(kit.FsNote).TextColor(t.TextMuted)
					ui.Textf(c, "%d", len(items)).FontSize(kit.FsNote).TextColor(a.pal.Faint)
				})
				open := w.pendMore[g.id]
				for i, it := range items {
					if i == pendShown && !open {
						break
					}
					a.pendingRow(c, it)
				}
				if len(items) > pendShown {
					label := locales.StatusMorePending(len(items) - pendShown)
					if open {
						label = locales.CommonCollapse()
					}
					more := ui.ButtonBase(c).Row().Justify(ui.Start).Margin(0, 8).Padding(8, 12).Label(g.name + label)
					color := t.TextMuted
					if more.Hovered() {
						color = t.Text
					}
					more.Children(func() { ui.Text(c, label).FontSize(kit.FsNote).TextColor(color) })
					if more.Clicked() {
						if w.pendMore == nil {
							w.pendMore = map[string]bool{}
						}
						w.pendMore[g.id] = !open
					}
				}
			})
		}
		if len(shown) == 0 {
			ui.Text(c, locales.StatusNoneHere()).FontSize(kit.FsControl).TextColor(t.TextMuted).Padding(20)
		}
	})
	ui.Box(c).FillWidth().Height(1).Background(a.pal.Line)
	ui.Row(c).FillWidth().Padding(12, 20).Children(func() {
		ui.Spacer(c)
		all := ui.ButtonBase(c).Label(locales.StatusIgnoreAll()).Disabled(len(shown) == 0)
		color := t.TextMuted
		if all.Hovered() {
			color = t.Text
		}
		all.Children(func() { ui.Text(c, locales.StatusIgnoreAll()).FontSize(kit.FsNote).TextColor(color) })
		if all.Clicked() {
			a.ignore(shown)
		}
	})
}

// pendingRow is one thing pending; choosing it goes to where it is.
func (a *App) pendingRow(c *ui.Context, it pendItem) {
	t := a.pal.Theme
	r := ui.ButtonBase(c).Row().Gap(12).AlignItems(ui.Center).Margin(0, 8).Padding(10, 12).Radius(kit.RControl).Label("待处理-" + it.text)
	if r.Hovered() {
		r.Background(t.SurfaceHover)
	}
	kit.PressRow(c, r)
	r.Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, it.text).FontSize(kit.FsBody).LineHeight(kit.LhBody).MaxLines(2).Ellipsis("…").FillWidth()
			ui.Text(c, it.sub).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").FillWidth()
		})
		kit.Glyph(c, kit.IcChevron, kit.IconSm, a.pal.Faint)
	})
	if r.Clicked() {
		a.write.pendOpen = false
		it.open(c.Services())
	}
}

// ignore hides only these items from the list, leaving their marks alone.
func (a *App) ignore(items []pendItem) {
	w := &a.write
	st := a.Store
	was := maps.Clone(w.ignored)
	if w.ignored == nil {
		w.ignored = map[string]bool{}
	}
	for _, it := range items {
		w.ignored[it.key] = true
	}
	w.ignoredVer++
	if err := a.savePrefs(); err != nil {
		w.ignored = was
		w.ignoredVer++
		return
	}
	w.pendOpen = false
	a.status.Did(locales.StatusIgnored(len(items)), func() error {
		if a.Store != st {
			return fmt.Errorf("%s", locales.StatusWorkSwitchedUndo())
		}
		now := w.ignored
		w.ignored = was
		w.ignoredVer++
		if err := a.savePrefs(); err != nil {
			w.ignored = now
			w.ignoredVer++
			return err
		}
		return nil
	})
}

// ignoredKeys is what is ignored, in order to be kept.
func (w *writeView) ignoredKeys() []string { return slices.Sorted(maps.Keys(w.ignored)) }
