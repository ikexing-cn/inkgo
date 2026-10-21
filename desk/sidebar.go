package desk

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// dragNode is a node of the chapter tree being dragged.
type dragNode string

func (a *App) loadTree() error {
	t, err := a.Store.Chapters()
	if err == nil {
		a.write.tree = t
	}
	return err
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
			if a.page == pagePlan {
				a.page = 3
			}
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
