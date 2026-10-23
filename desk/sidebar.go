package desk

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/workspace"
)

// dragNode is a node of the chapter tree being dragged.
type dragNode string

func (a *App) loadTree() error {
	return a.book.ReloadTree()
}

func nodeIcon(n *workspace.Node) *ui.SVG {
	if n.Folder {
		return kit.IcFolder
	}
	return kit.IcDoc
}

// chapterTree is the sidebar of the writing page.
func (a *App) chapterTree(c *ui.Context) {
	w := &a.write
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 4, 6, 10).Gap(2).Children(func() {
		ui.Text(c, "章节").FontSize(kit.FsNote).TextColor(a.pal.Theme.TextMuted).Grow(1)
		if kit.IconButton(c, kit.IcFolderPlus, "新建文件夹", ui.Color{}).Clicked() {
			a.newNode(true)
		}
		if kit.IconButton(c, kit.IcPlus, "新建章节", ui.Color{}).Tooltip("新建章节（" + modKey + "N）").Clicked() {
			a.newNode(false)
		}
	})
	ui.Scroll(c.Key("chapters")).FillWidth().Grow(1).Gap(2).Children(func() {
		if a.book.Tree().Root == nil {
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
		rows(a.book.Tree().Root.Children, 0)
		// Dropping below the rows puts a node at the end of the top level.
		end := ui.Box(c).FillWidth().Height(40)
		if v, ok := ui.Drop[dragNode](end); ok {
			a.warn(a.book.MoveChapter(string(v), "", len(a.book.Tree().Root.Children)))
		}
	})
}

// trashRow opens the trash.
func (a *App) trashRow(c *ui.Context) {
	t := a.pal.Theme
	b := ui.ButtonBase(c).Row().Grow(1).Shrink(1).Justify(ui.Start).AlignItems(ui.Center).Gap(10).Padding(8, 10).Radius(kit.RControl).Label("回收站").KeepFocus()
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		kit.Glyph(c, kit.IcTrash, kit.IconMd, t.TextMuted)
		ui.Text(c, "回收站").FontSize(kit.FsControl).TextColor(t.TextMuted)
	})
	if b.Clicked() {
		a.openTrash()
	}
}

func (a *App) treeRow(c *ui.Context, n *workspace.Node, depth int) {
	t := a.pal.Theme
	w := &a.write
	on := n.ID == w.node
	if w.renaming == n.ID {
		// The row stays a row, its icon and indent where they are, and the
		// name is a box to type in. Enter settles it, and so does leaving the
		// box; Esc leaves the name as it was.
		row := ui.Row(c.Key("r-"+n.ID)).FillWidth().Height(kit.HRow).AlignItems(ui.Center).Padding(0, 10, 0, 10+float32(depth)*20).Radius(kit.RControl)
		if on {
			row.Background(t.SurfacePressed)
		}
		row.Children(func() {
			if n.Folder {
				s := kit.IcChevronDown
				if w.folded[n.ID] {
					s = kit.IcChevron
				}
				kit.Glyph(c, s, kit.IconXs, t.Text)
			} else {
				kit.Glyph(c, nodeIcon(n), kit.IconSm, t.Text)
			}
			// The box gives the row's gap of 6 back as its border and padding,
			// so the words in it stand where the names of the other rows do.
			ui.Row(c).Grow(1).Shrink(1).Height(kit.HRow-6).AlignItems(ui.Center).Padding(0, 6-1).Radius(kit.RXs).Background(a.pal.Canvas).Border(1, a.pal.Ink).Children(func() {
				in := ui.TextInputBase(c, &w.title).Label("章节标题").Placeholder(w.titleWas).Grow(1).Shrink(1).FontSize(kit.FsControl).AutoFocus()
				if w.titleAll {
					w.titleAll = false
					in.SetTextSelection(0, utf8.RuneCountInString(w.title))
				}
				switch {
				case in.Shortcut(0, ui.KeyEscape):
					w.renaming, w.titleHeld = "", false
				case in.Submitted():
					// Named, the open chapter is written: the caret goes to the prose.
					if a.rename(n.ID); on && w.cur().ed != nil {
						w.cur().ed.Focus()
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
	row := ui.ButtonBase(c.Key("n-"+n.ID)).Row().FillWidth().Height(kit.HRow).Justify(ui.Start).AlignItems(ui.Center).Gap(6).
		Padding(0, 10, 0, 10+float32(depth)*20).Radius(kit.RControl).Label("章节-" + n.Title)
	row.Drag(dragNode(n.ID))
	_, over := ui.DragOver[dragNode](row)
	switch {
	case over:
		row.Background(a.pal.Ink.Alpha(0.12))
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
			s := kit.IcChevronDown
			if w.folded[n.ID] {
				s = kit.IcChevron
			}
			kit.Glyph(c, s, kit.IconXs, fg)
		} else {
			kit.Glyph(c, nodeIcon(n), kit.IconSm, fg)
		}
		ui.Text(c, n.Title).FontSize(kit.FsControl).FontWeight(weight).TextColor(fg).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		if k := a.wordsIn(n); k > 0 {
			ui.Text(c, strings.ReplaceAll(strings.TrimSuffix(kit.Words(k), "字"), " ", "")).FontSize(kit.FsSmall).TextColor(a.pal.Faint)
		}
	})
	a.menu.ContextMenu(c, a.pal, row, "章节-"+n.ID, func(m *kit.Menu) { a.nodeMenu(m, n) })
	if row.Clicked() {
		// A folder only folds: its outline opens from its menu. The tree
		// arranges, the tabs show.
		if n.Folder {
			w.folded[n.ID] = !w.folded[n.ID]
		} else {
			if a.page == pagePlan {
				a.page = pageWrite
			}
			a.openNode(n.ID)
		}
	}
}

// renameOpen starts naming the node that is open, as F2 asks. Its row is
// brought out first: from a folded folder, and from focus mode.
func (a *App) renameOpen() {
	w := &a.write
	n, ok := a.book.Tree().Node(w.node)
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

func (a *App) nodeMenu(m *kit.Menu, n *workspace.Node) {
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
	a.book.Tree().Walk(func(f *workspace.Node, depth int) {
		if f.Folder && f != n.Parent && !f.Within(n) {
			into, depths = append(into, f), append(depths, depth)
		}
	})
	if top := n.Parent.Parent != nil; top || len(into) > 0 {
		m.Head("移到")
		if top {
			m.Item("最上层").Do(func() { a.move(n.ID, "", len(a.book.Tree().Root.Children)) })
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
	a.warn(a.book.MoveChapter(id, parent, index))
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
	if kit.Trim(w.title) == "" {
		w.title = w.titleWas
	}
	// A whole numbered title typed after the number offered replaces it.
	if rest, ok := strings.CutPrefix(w.title, w.offered); ok && w.offered != "" && workspace.TitleNumber(kit.Trim(rest)) != "" {
		w.title = rest
	}
	w.offered, w.titleHeld = "", false
	err := a.book.RenameChapter(id, kit.Trim(w.title))
	if err == nil {
		w.renaming = ""
	}
	a.warn(err)
}

// newNode adds a document, or a folder, beside the node shown, or inside
// it when it is a folder, and has the author name it.
func (a *App) newNode(folder bool) {
	n, _ := a.book.Tree().Node(a.write.node)
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
	id, err := a.book.NewChapter(parent, title, folder)
	if err == nil && index >= 0 {
		err = a.book.MoveChapter(id, parent, index)
	}
	// Numbered like the one before, the title is ready to go on with its
	// name: 第四章 名单 gives 第五章.
	typed := ""
	if next := a.book.Tree().TitleAfter(id); err == nil && next != "" {
		title, typed = next, next+" "
		err = a.book.RenameChapter(id, title)
	}
	if err != nil {
		a.warn(err)
		return
	}
	if parent != "" {
		w.folded[parent] = false
	}
	a.openNode(id)
	w.focus = false
	a.showTree()
	w.renaming, w.title, w.titleWas, w.offered = id, typed, title, typed
	if typed == "" { // 新章节, 新文件夹: in the box selected, to be typed over
		w.title, w.titleAll = title, true
	}
	// The name comes first: the caret is in its box, not in the empty prose.
	if w.cur().ed != nil {
		w.cur().ed.Unfocus()
	}
}
