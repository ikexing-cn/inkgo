package kit

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// The app draws its own menus: a panel of rows, the same wherever one
// opens, as the design has them. One is open at a time.

const (
	MenuWide = 200 // a menu where the pointer asked for one
	menuSide = 120 // the list a row of it opens beside it
	MenuEdge = 8   // what a menu keeps clear of the window's edges
)

// MenuState holds the one open menu and its submenu across frames.
type MenuState struct {
	Owner string  // what it belongs to, "" while none is open
	Drop  bool    // it hangs from what was pressed; else it is at x, y
	X, Y  float32 // where it was asked for, in the window
	sub   string  // the row of it whose side list is open
	Seen  bool    // it was drawn in the last frame
}

// menu lists the rows of a menu: build names them in order, each with what
// picking it does. It holds nothing of the frame it is drawn in: the rows
// are drawn once build has listed them (menuRows), and the one picked then
// does what it was given to Do. What build would draw itself goes in a Draw
// row.
type Menu struct {
	Rows []*MenuItem
}

// The kinds of a menu's rows.
const (
	RowItem = iota // one to pick
	rowLine        // a line between groups of rows
	rowHead        // the name of the rows under it
	rowSub         // one that opens a list beside the menu
	rowDraw        // what the menu's owner draws itself
)

// menuItem is a row of a menu. Its methods set it up and return it, for
// chaining.
type MenuItem struct {
	Kind    int
	Label   string
	hint    string
	off, on bool
	do      func()              // what picking a rowItem does
	build   func(m *Menu)       // fills the list a rowSub opens
	draw    func(c *ui.Context) // draws a rowDraw
}

func (m *Menu) add(it *MenuItem) *MenuItem {
	m.Rows = append(m.Rows, it)
	return it
}

// Item adds a row saying label.
func (m *Menu) Item(label string) *MenuItem { return m.add(&MenuItem{Label: label}) }

// Disabled pales the row when d: it cannot be picked.
func (it *MenuItem) Disabled(d bool) *MenuItem { it.off = d; return it }

// Hint puts s at the row's right: its shortcut, or how many it leads to.
func (it *MenuItem) Hint(s string) *MenuItem { it.hint = s; return it }

func (it *MenuItem) HintText() string { return it.hint }

// On marks the row as the one in use.
func (it *MenuItem) On(on bool) *MenuItem { it.on = on; return it }

// Do has do run as the row is picked, the menu closed by then. A row given
// nothing to do closes the menu all the same.
func (it *MenuItem) Do(do func()) { it.do = do }

// Separator adds a line between groups of rows.
func (m *Menu) Separator() { m.add(&MenuItem{Kind: rowLine}) }

// Head names the rows under it.
func (m *Menu) Head(label string) { m.add(&MenuItem{Kind: rowHead, Label: label}) }

// Submenu adds a row that opens a list beside the menu, which build fills:
// as the pointer comes over the row, or as it is picked.
func (m *Menu) Submenu(label string, build func(m *Menu)) {
	m.add(&MenuItem{Kind: rowSub, Label: label, build: build})
}

// Draw adds what draw draws where a row would be: a choice that is not a
// row to pick.
func (m *Menu) Draw(draw func(c *ui.Context)) { m.add(&MenuItem{Kind: rowDraw, draw: draw}) }

// menuRows draws the rows build lists, and has the one picked do its part.
// list says they are the list beside a menu, which opens none of its own;
// of a menu's rows it returns the one whose list is open, and where it is.
func (m *MenuState) menuRows(c *ui.Context, p *Palette, build func(m *Menu), list bool) (open *MenuItem, at ui.Rect) {
	rows := &Menu{}
	build(rows)
	var picked *MenuItem
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
	for _, it := range rows.Rows {
		switch it.Kind {
		case rowLine:
			p.PopLine(c)
		case rowHead:
			p.PopHead(c, it.Label)
		case rowDraw:
			it.draw(c)
		case rowSub:
			on := !list && m.sub == it.Label
			row := p.popRow(c, IcChevron, it.Label, "", false, on)
			hold(row, false)
			if !list && !on && (row.Hovered() || row.Clicked()) {
				m.sub, on = it.Label, true
			}
			if on {
				open, at = it, row.Bounds()
			}
		case RowItem:
			row := p.popRow(c, nil, it.Label, it.hint, it.off, it.on)
			hold(row, it.off)
			if !list && m.sub != "" && row.Hovered() {
				// The pointer went on to another row of the menu: the list
				// beside it goes.
				m.sub = ""
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
	*m = MenuState{}
	if picked.do != nil {
		picked.do()
	}
	return nil, ui.Rect{}
}

// menuBeside draws the list the row at opens, beside it: to its left when
// flip says there is no room on the right.
func (m *MenuState) menuBeside(c *ui.Context, p *Palette, row *MenuItem, at ui.Rect, flip bool) {
	// Its first row beside the row that opened it.
	x := at.X + at.W + 4
	if flip {
		x = at.X - 4 - menuSide
	}
	list := p.popPanel(c.Key("菜单旁-" + m.Owner)).Absolute().Left(x).Width(menuSide)
	y := at.Y - 4
	if now := list.Bounds(); at.W == 0 || now.H == 0 {
		list.Opacity(0) // not laid out yet: placed in the next frame
		c.Invalidate()
	} else {
		// Level with its row, unless that takes it out of the window.
		_, h := c.Size()
		y = max(MenuEdge, min(y, float32(h)-now.H-MenuEdge))
	}
	list.Top(y)
	list.Children(func() { m.menuRows(c, p, row.build, true) })
}

// contextMenu gives el a menu that build fills: where el is clicked with
// the secondary button, or under it for the menu key while it has the
// focus. key tells el's menu from the others.
func (m *MenuState) ContextMenu(c *ui.Context, p *Palette, el ui.Element, key string, build func(m *Menu)) {
	if el.RightClicked() {
		at := el.Bounds()
		x, y, _ := el.PointerPosition()
		*m = MenuState{Owner: key, X: at.X + x, Y: at.Y + y}
	} else if el.Shortcut(0, ui.KeyContextMenu) || el.Shortcut(ui.Shift, ui.KeyF10) {
		at := el.Bounds()
		*m = MenuState{Owner: key, X: at.X, Y: at.Y + at.H}
	}
	m.MenuAt(c, p, key, build)
}

// menuAt draws the menu key where it was asked for, while it is open. A
// press anywhere else puts it away and does nothing more, as Escape does.
func (m *MenuState) MenuAt(c *ui.Context, p *Palette, key string, build func(m *Menu)) {
	if m.Owner != key || m.Drop {
		return
	}
	m.Seen = true
	ui.Overlay(c, func() {
		// Each part has a name: the window's overlays are told apart by
		// their order otherwise, and one coming or going, as the list
		// beside the menu, would hand a part the box of another.
		veil := ui.Box(c.Key("菜单底-" + key)).Absolute().Left(0).Top(0).Right(0).Bottom(0)
		shut := veil.Clicked() || veil.RightClicked()
		w, h := c.Size()
		panel := p.popPanel(c.Key("菜单-" + key)).Absolute().Width(MenuWide)
		x, y := m.X, m.Y
		if now := panel.Bounds(); now.H == 0 {
			panel.Opacity(0) // not laid out yet: placed in the next frame
			c.Invalidate()
		} else {
			// The menu stays in the window whole.
			x = max(MenuEdge, min(x, float32(w)-now.W-MenuEdge))
			y = max(MenuEdge, min(y, float32(h)-now.H-MenuEdge))
		}
		panel.Left(x).Top(y)
		var open *MenuItem
		var at ui.Rect
		panel.Children(func() { open, at = m.menuRows(c, p, build, false) })
		if open != nil {
			m.menuBeside(c, p, open, at, x+MenuWide+menuSide+MenuEdge > float32(w))
		}
		// Tab stays in the menu, as in the system's: the arrows go
		// through its rows.
		panel.OverlayShortcut(0, ui.KeyTab)
		panel.OverlayShortcut(ui.Shift, ui.KeyTab)
		if shut || panel.OverlayShortcut(0, ui.KeyEscape) {
			*m = MenuState{}
		}
	})
}

// dropMenu hangs a menu from b, which opens it and puts it away: place
// sets where the panel goes and how wide it is, build fills it. key tells
// b's menu from the others.
func (m *MenuState) DropMenu(c *ui.Context, p *Palette, b ui.Element, key string, place func(panel ui.Element), build func(m *Menu)) {
	if b.Clicked() {
		if m.Owner == key {
			*m = MenuState{}
		} else {
			*m = MenuState{Owner: key, Drop: true}
		}
	}
	if m.Owner != key || !m.Drop {
		return
	}
	m.Seen = true
	open := true
	// Named, as the parts of the other menus are: a dialog a row opens is
	// not taken for the menu going on, and takes the focus as its own.
	ui.PopoverBase(c.Key("菜单-"+key), b, &open, func(panel ui.Element) {
		p.popLook(panel)
		place(panel)
		m.menuRows(c, p, build, false)
	})
	if !open && m.Owner == key {
		*m = MenuState{}
	}
}

// menuOpen reports whether the menu key is open: what opens it shows it.
func (m *MenuState) MenuOpen(key string) bool { return m.Owner == key }

// popPanel is the panel of a menu.
func (p *Palette) popPanel(c *ui.Context) ui.Element {
	return p.popLook(ui.Column(c))
}

// popLook gives panel the look of a menu.
func (p *Palette) popLook(panel ui.Element) ui.Element {
	panel.Padding(4).Radius(RCard).Background(p.Raised).Border(1, p.Theme.Border).Role(ui.RoleMenu).FocusGroup(ui.Vertical)
	p.Pop.On(panel)
	return panel
}

// popRow is a row of a menu: a picture if any, what it does, and at its
// right its shortcut, a count, or why it cannot be done now. on is the
// row in use, or the one whose list is open.
func (p *Palette) popRow(c *ui.Context, pic *ui.SVG, label, hint string, off, on bool) ui.Element {
	t := p.Theme
	b := ui.ButtonBase(c).Row().FillWidth().Height(HRow).Padding(0, 10).Gap(8).AlignItems(ui.Center).Radius(RControl).Shrink(0).
		Role(ui.RoleMenuItem).Label("菜单-" + strings.TrimSuffix(label, "…")).Disabled(off)
	switch {
	case off:
		Pale(b)
	case on || b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		if pic != nil && pic != IcChevron {
			Glyph(c, pic, IconMd, t.TextMuted)
		}
		// Its own ink: a disabled button would pale its words a second time.
		ui.Text(c, label).FontSize(FsControl).TextColor(t.Text).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
		if hint != "" {
			ui.Text(c, hint).FontSize(FsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
		}
		if pic == IcChevron {
			Glyph(c, pic, IconSm, p.Faint)
		}
	})
	return b
}

// popHead names the rows under it.
func (p *Palette) PopHead(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(FsSmall).TextColor(p.Faint).Padding(6, 10, 2, 10).Shrink(0)
}

// popLine parts groups of rows.
func (p *Palette) PopLine(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Margin(4, 0).Background(p.Line).Shrink(0)
}
