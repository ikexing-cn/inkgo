package desk

import (
	"fmt"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

func (a *App) trashNode(n *workspace.Node) {
	w := &a.write
	a.flush()
	// Where the node shown stands among the chapters, to open the one that
	// takes its place if it goes and no tab is left.
	var order []string
	a.book.Tree().Walk(func(d *workspace.Node, _ int) {
		if !d.Folder {
			order = append(order, d.ID)
		}
	})
	cur, shown := a.book.Tree().Node(w.node)
	gone := shown && cur.Within(n)
	at := slices.Index(w.tabs, w.node)
	if err := a.book.TrashChapter(n.ID); err != nil {
		a.warn(err)
		return
	}
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
				if _, ok := a.book.Tree().Node(id); ok {
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
	a.status.did(fmt.Sprintf("「%s」已移到回收站", n.Title))
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
			a.pal.dialogHead(c, "回收站", "恢复后回到原文件夹末尾。")
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
								ui.Text(c, when(it.At, time.Now())+" 删除").FontSize(fsNote).LineHeight(lhBody).TextColor(t.TextMuted)
							})
							// Deleting for good is asked twice, in the row itself.
							if w.purge == it.ID {
								if a.pal.secondary(c, "取消").Label("取消彻底删除").Clicked() {
									w.purge = ""
								}
								if a.pal.danger(c, "确认彻底删除").Clicked() {
									a.warn(a.book.PurgeChapter(it.ID))
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
	if err := a.book.RestoreChapter(id); err != nil {
		a.warn(err)
		return
	}
	a.openTrash()
	a.status.did(fmt.Sprintf("「%s」已恢复", title))
}
