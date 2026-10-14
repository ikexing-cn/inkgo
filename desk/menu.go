package desk

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// The app draws its own menus: a panel of rows, the same wherever one
// opens, as the design has them. One is open at a time.

const (
	menuWide = 200 // a menu where the pointer asked for one
	menuSide = 120 // the list a row of it opens beside it
	menuEdge = 8   // what a menu keeps clear of the window's edges
)

// menuState is the menu that is open.
type menuState struct {
	owner string  // what it belongs to, "" while none is open
	drop  bool    // it hangs from what was pressed; else it is at x, y
	x, y  float32 // where it was asked for, in the window
	sub   string  // the row of it whose side list is open
	seen  bool    // it was drawn in the last frame
}

// menu lists the rows of a menu: build names them in order, each with what
// picking it does. It holds nothing of the frame it is drawn in: the rows
// are drawn once build has listed them (menuRows), and the one picked then
// does what it was given to Do. What build would draw itself goes in a Draw
// row.
type menu struct {
	rows []*menuItem
}

// The kinds of a menu's rows.
const (
	rowItem = iota // one to pick
	rowLine        // a line between groups of rows
	rowHead        // the name of the rows under it
	rowSub         // one that opens a list beside the menu
	rowDraw        // what the menu's owner draws itself
)

// menuItem is a row of a menu. Its methods set it up and return it, for
// chaining.
type menuItem struct {
	kind        int
	label, hint string
	off, on     bool
	do          func()              // what picking a rowItem does
	build       func(m *menu)       // fills the list a rowSub opens
	draw        func(c *ui.Context) // draws a rowDraw
}

func (m *menu) add(it *menuItem) *menuItem {
	m.rows = append(m.rows, it)
	return it
}

// Item adds a row saying label.
func (m *menu) Item(label string) *menuItem { return m.add(&menuItem{label: label}) }

// Disabled pales the row when d: it cannot be picked.
func (it *menuItem) Disabled(d bool) *menuItem { it.off = d; return it }

// Hint puts s at the row's right: its shortcut, or how many it leads to.
func (it *menuItem) Hint(s string) *menuItem { it.hint = s; return it }

// On marks the row as the one in use.
func (it *menuItem) On(on bool) *menuItem { it.on = on; return it }

// Do has do run as the row is picked, the menu closed by then. A row given
// nothing to do closes the menu all the same.
func (it *menuItem) Do(do func()) { it.do = do }

// Separator adds a line between groups of rows.
func (m *menu) Separator() { m.add(&menuItem{kind: rowLine}) }

// Head names the rows under it.
func (m *menu) Head(label string) { m.add(&menuItem{kind: rowHead, label: label}) }

// Submenu adds a row that opens a list beside the menu, which build fills:
// as the pointer comes over the row, or as it is picked.
func (m *menu) Submenu(label string, build func(m *menu)) {
	m.add(&menuItem{kind: rowSub, label: label, build: build})
}

// Draw adds what draw draws where a row would be: a choice that is not a
// row to pick.
func (m *menu) Draw(draw func(c *ui.Context)) { m.add(&menuItem{kind: rowDraw, draw: draw}) }

// menuRows draws the rows build lists, and has the one picked do its part.
// list says they are the list beside a menu, which opens none of its own;
// of a menu's rows it returns the one whose list is open, and where it is.
func (a *App) menuRows(c *ui.Context, build func(m *menu), list bool) (open *menuItem, at ui.Rect) {
	m := &menu{}
	build(m)
	var picked *menuItem
	// The first row that can be picked has the focus as the menu opens: the
	// arrows then go through the rows and Enter picks one, and the focus
	// goes back where it was as the menu closes.
	held := false
	hold := func(row ui.Element, off bool) {
		if !off && !held {
			held = true
			row.AutoFocus()
		}
	}
	for _, it := range m.rows {
		switch it.kind {
		case rowLine:
			a.pal.popLine(c)
		case rowHead:
			a.pal.popHead(c, it.label)
		case rowDraw:
			it.draw(c)
		case rowSub:
			on := !list && a.menu.sub == it.label
			row := a.pal.popRow(c, icChevron, it.label, "", false, on)
			hold(row, false)
			if !list && !on && (row.Hovered() || row.Clicked()) {
				a.menu.sub, on = it.label, true
			}
			if on {
				open, at = it, row.Bounds()
			}
		case rowItem:
			row := a.pal.popRow(c, nil, it.label, it.hint, it.off, it.on)
			hold(row, it.off)
			if !list && a.menu.sub != "" && row.Hovered() {
				// The pointer went on to another row of the menu: the list
				// beside it goes.
				a.menu.sub = ""
				c.Invalidate()
			}
			if row.Clicked() {
				picked = it
			}
		}
	}
	if picked == nil {
		return open, at
	}
	a.menu = menuState{}
	if picked.do != nil {
		picked.do()
	}
	return nil, ui.Rect{}
}

// menuBeside draws the list the row at opens, beside it: to its left when
// flip says there is no room on the right.
func (a *App) menuBeside(c *ui.Context, row *menuItem, at ui.Rect, flip bool) {
	// Its first row beside the row that opened it.
	x := at.X + at.W + 4
	if flip {
		x = at.X - 4 - menuSide
	}
	list := a.pal.popPanel(c.Key("菜单旁-" + a.menu.owner)).Absolute().Left(x).Width(menuSide)
	y := at.Y - 4
	if now := list.Bounds(); at.W == 0 || now.H == 0 {
		list.Opacity(0) // not laid out yet: placed in the next frame
		c.Invalidate()
	} else {
		// Level with its row, unless that takes it out of the window.
		_, h := c.Size()
		y = max(menuEdge, min(y, float32(h)-now.H-menuEdge))
	}
	list.Top(y)
	list.Children(func() { a.menuRows(c, row.build, true) })
}

// contextMenu gives el a menu that build fills: where el is clicked with
// the secondary button, or under it for the menu key while it has the
// focus. key tells el's menu from the others.
func (a *App) contextMenu(c *ui.Context, el ui.Element, key string, build func(m *menu)) {
	if el.RightClicked() {
		at := el.Bounds()
		x, y, _ := el.PointerPosition()
		a.menu = menuState{owner: key, x: at.X + x, y: at.Y + y}
	} else if el.Shortcut(0, ui.KeyContextMenu) || el.Shortcut(ui.Shift, ui.KeyF10) {
		at := el.Bounds()
		a.menu = menuState{owner: key, x: at.X, y: at.Y + at.H}
	}
	a.menuAt(c, key, build)
}

// menuAt draws the menu key where it was asked for, while it is open. A
// press anywhere else puts it away and does nothing more, as Escape does.
func (a *App) menuAt(c *ui.Context, key string, build func(m *menu)) {
	if a.menu.owner != key || a.menu.drop {
		return
	}
	a.menu.seen = true
	ui.Overlay(c, func() {
		// Each part has a name: the window's overlays are told apart by
		// their order otherwise, and one coming or going, as the list
		// beside the menu, would hand a part the box of another.
		veil := ui.Box(c.Key("菜单底-" + key)).Absolute().Left(0).Top(0).Right(0).Bottom(0)
		shut := veil.Clicked() || veil.RightClicked()
		w, h := c.Size()
		panel := a.pal.popPanel(c.Key("菜单-" + key)).Absolute().Width(menuWide)
		x, y := a.menu.x, a.menu.y
		if now := panel.Bounds(); now.H == 0 {
			panel.Opacity(0) // not laid out yet: placed in the next frame
			c.Invalidate()
		} else {
			// The menu stays in the window whole.
			x = max(menuEdge, min(x, float32(w)-now.W-menuEdge))
			y = max(menuEdge, min(y, float32(h)-now.H-menuEdge))
		}
		panel.Left(x).Top(y)
		var open *menuItem
		var at ui.Rect
		panel.Children(func() { open, at = a.menuRows(c, build, false) })
		if open != nil {
			a.menuBeside(c, open, at, x+menuWide+menuSide+menuEdge > float32(w))
		}
		// Tab stays in the menu, as in the system's: the arrows go
		// through its rows.
		panel.OverlayShortcut(0, ui.KeyTab)
		panel.OverlayShortcut(ui.Shift, ui.KeyTab)
		if shut || panel.OverlayShortcut(0, ui.KeyEscape) {
			a.menu = menuState{}
		}
	})
}

// dropMenu hangs a menu from b, which opens it and puts it away: place
// sets where the panel goes and how wide it is, build fills it. key tells
// b's menu from the others.
func (a *App) dropMenu(c *ui.Context, b ui.Element, key string, place func(panel ui.Element), build func(m *menu)) {
	if b.Clicked() {
		if a.menu.owner == key {
			a.menu = menuState{}
		} else {
			a.menu = menuState{owner: key, drop: true}
		}
	}
	if a.menu.owner != key || !a.menu.drop {
		return
	}
	a.menu.seen = true
	open := true
	// Named, as the parts of the other menus are: a dialog a row opens is
	// not taken for the menu going on, and takes the focus as its own.
	ui.PopoverBase(c.Key("菜单-"+key), b, &open, func(panel ui.Element) {
		a.pal.popLook(panel)
		place(panel)
		a.menuRows(c, build, false)
	})
	if !open && a.menu.owner == key {
		a.menu = menuState{}
	}
}

// menuOpen reports whether the menu key is open: what opens it shows it.
func (a *App) menuOpen(key string) bool { return a.menu.owner == key }

// popPanel is the panel of a menu.
func (p *palette) popPanel(c *ui.Context) ui.Element {
	return p.popLook(ui.Column(c))
}

// popLook gives panel the look of a menu.
func (p *palette) popLook(panel ui.Element) ui.Element {
	panel.Padding(4).Radius(rCard).Background(p.raised).Border(1, p.theme.Border).Role(ui.RoleMenu).FocusGroup(ui.Vertical)
	p.pop.on(panel)
	return panel
}

// popRow is a row of a menu: a picture if any, what it does, and at its
// right its shortcut, a count, or why it cannot be done now. on is the
// row in use, or the one whose list is open.
func (p *palette) popRow(c *ui.Context, pic *ui.SVG, label, hint string, off, on bool) ui.Element {
	t := p.theme
	b := ui.ButtonBase(c).Row().FillWidth().Height(hRow).Padding(0, 10).Gap(8).AlignItems(ui.Center).Radius(rControl).Shrink(0).
		Role(ui.RoleMenuItem).Label("菜单-" + strings.TrimSuffix(label, "…")).Disabled(off)
	switch {
	case off:
		pale(b)
	case on || b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		if pic != nil && pic != icChevron {
			glyph(c, pic, iconMd, t.TextMuted)
		}
		// Its own ink: a disabled button would pale its words a second time.
		ui.Text(c, label).FontSize(fsControl).TextColor(t.Text).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
		if hint != "" {
			ui.Text(c, hint).FontSize(fsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
		}
		if pic == icChevron {
			glyph(c, pic, iconSm, p.faint)
		}
	})
	return b
}

// popHead names the rows under it.
func (p *palette) popHead(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(fsSmall).TextColor(p.faint).Padding(6, 10, 2, 10).Shrink(0)
}

// popLine parts groups of rows.
func (p *palette) popLine(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Margin(4, 0).Background(p.line).Shrink(0)
}
