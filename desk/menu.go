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

// menu is a menu being drawn: build adds its rows in order, and a row's
// Chosen says it was picked, which closes the menu.
type menu struct {
	a *App
	// The Context of the frame the menu is built in: a menu is made, built
	// and dropped in the one call, so it is never kept past its frame. mygo
	// vet reports any Context in a struct; scripts/verify-ui.ts lets this
	// one through, and no other.
	c    *ui.Context
	wait *menuItem // the row asked for and not yet drawn
	side func()    // the list beside the menu, drawn after it
	flip bool      // that list goes to the left: no room on the right
	held bool      // a row of it took the focus as it opened
	list bool      // it is the list beside a menu, not the menu
}

// hold gives the first row that can be picked the focus as the menu opens:
// the arrows then go through the rows and Enter picks one, and the focus
// goes back where it was as the menu closes.
func (m *menu) hold(row ui.Element, off bool) {
	if !off && !m.held {
		m.held = true
		row.AutoFocus()
	}
}

// menuItem is a row of a menu. Its methods set it up and return it, for
// chaining; it is drawn when Chosen asks, or when the next row comes.
type menuItem struct {
	m           *menu
	label, hint string
	off, on     bool
}

// Item adds a row saying label.
func (m *menu) Item(label string) *menuItem {
	m.flush()
	m.wait = &menuItem{m: m, label: label}
	return m.wait
}

// Disabled pales the row when d: it cannot be picked.
func (it *menuItem) Disabled(d bool) *menuItem { it.off = d; return it }

// Hint puts s at the row's right: its shortcut, or how many it leads to.
func (it *menuItem) Hint(s string) *menuItem { it.hint = s; return it }

// On marks the row as the one in use.
func (it *menuItem) On(on bool) *menuItem { it.on = on; return it }

// Chosen reports that the row was picked.
func (it *menuItem) Chosen() bool {
	m := it.m
	if m.wait != it {
		return false
	}
	m.wait = nil
	row := m.a.pal.popRow(m.c, nil, it.label, it.hint, it.off, it.on)
	m.hold(row, it.off)
	if !m.list && m.a.menu.sub != "" && row.Hovered() {
		// The pointer went on to another row of the menu: the list beside
		// it goes.
		m.a.menu.sub = ""
		m.c.Invalidate()
	}
	if !row.Clicked() {
		return false
	}
	m.a.menu = menuState{}
	return true
}

// flush draws the row no one asked about.
func (m *menu) flush() {
	if m.wait != nil {
		m.wait.Chosen()
	}
}

// Separator draws a line between groups of rows.
func (m *menu) Separator() {
	m.flush()
	m.a.pal.popLine(m.c)
}

// Head names the rows under it.
func (m *menu) Head(label string) {
	m.flush()
	m.a.pal.popHead(m.c, label)
}

// Submenu adds a row that opens a list beside the menu, which build fills:
// as the pointer comes over the row, or as it is picked.
func (m *menu) Submenu(label string, build func(m *menu)) {
	m.flush()
	a, c := m.a, m.c
	open := a.menu.sub == label
	row := a.pal.popRow(c, icChevron, label, "", false, open)
	m.hold(row, false)
	if !open && (row.Hovered() || row.Clicked()) {
		a.menu.sub, open = label, true
	}
	if !open {
		return
	}
	at := row.Bounds()
	flip := m.flip
	m.side = func() {
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
		list.Children(func() {
			sub := &menu{a: a, c: c, list: true}
			build(sub)
			sub.flush()
		})
	}
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
		m := &menu{a: a, c: c, flip: x+menuWide+menuSide+menuEdge > float32(w)}
		panel.Children(func() {
			build(m)
			m.flush()
		})
		if m.side != nil {
			m.side()
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
		m := &menu{a: a, c: c}
		build(m)
		m.flush()
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
