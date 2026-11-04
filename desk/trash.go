package desk

import (
	"slices"

	"writer/locales"
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
		w.node, w.cur().ed, w.cur().external, w.cur().lost = "", nil, nil, false
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
	a.status.Did(locales.SideMovedToTrash(n.Title))
}
