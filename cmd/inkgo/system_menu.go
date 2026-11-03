package main

import (
	"runtime"
	"sync"

	"github.com/egoist/mygo"
	"writer/desk"
)

type menuItemState interface {
	SetLabel(string)
	SetEnabled(bool)
	SetChecked(bool)
	SetVisible(bool)
}

type menuSync struct {
	previous []desk.SystemMenuItem
	items    map[string]menuItemState
	install  func([]desk.SystemMenuItem) map[string]menuItemState
}

func sameMenuStructure(a, b []desk.SystemMenuItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		y := b[i]
		if x.ID != y.ID || x.Role != y.Role || x.Accelerator != y.Accelerator || x.Separator != y.Separator || x.Radio != y.Radio || !sameMenuStructure(x.Children, y.Children) {
			return false
		}
	}
	return true
}

func (s *menuSync) update(next []desk.SystemMenuItem) {
	if s.items == nil || !sameMenuStructure(s.previous, next) {
		s.items = s.install(next)
	} else {
		var push func([]desk.SystemMenuItem, []desk.SystemMenuItem)
		push = func(old, now []desk.SystemMenuItem) {
			for i, n := range now {
				o := old[i]
				item := s.items[n.ID]
				if n.Label != o.Label {
					item.SetLabel(n.Label)
				}
				if n.Enabled != o.Enabled {
					item.SetEnabled(n.Enabled)
				}
				if n.Checked != o.Checked {
					item.SetChecked(n.Checked)
				}
				if n.Visible != o.Visible {
					item.SetVisible(n.Visible)
				}
				push(o.Children, n.Children)
			}
		}
		push(s.previous, next)
	}
	s.previous = next
}

func nativeMenu(d *desk.App, invalidate func()) *menuSync {
	return &menuSync{install: func(model []desk.SystemMenuItem) map[string]menuItemState {
		items := map[string]menuItemState{}
		var build func([]desk.SystemMenuItem) []*mygo.MenuItem
		build = func(rows []desk.SystemMenuItem) []*mygo.MenuItem {
			result := make([]*mygo.MenuItem, 0, len(rows))
			for _, row := range rows {
				item := &mygo.MenuItem{ID: row.ID, Label: row.Label, Role: mygo.MenuRole(row.Role), Accelerator: row.Accelerator, Disabled: !row.Enabled, Hidden: !row.Visible, Checked: row.Checked}
				if row.Separator {
					item.Type = mygo.MenuItemSeparator
				}
				if row.Radio {
					item.Type = mygo.MenuItemRadio
				}
				if row.Children != nil {
					item.Submenu = build(row.Children)
				} else if !row.Separator && row.Role == "" {
					id := row.ID
					item.Click = func(_ *mygo.MenuItem, _ *mygo.Window) { d.QueueSystemMenu(id); invalidate() }
				}
				items[row.ID] = item
				result = append(result, item)
			}
			return result
		}
		mygo.App.SetMenu(mygo.NewMenu(build(model)))
		return items
	}}
}

// Called before NewWindow can start drawing; only the ready main thread
// installs the initial menu. Later snapshots come from the view thread.
func installSystemMenu(d *desk.App, invalidate func()) {
	if runtime.GOOS != "darwin" {
		return
	}
	s := nativeMenu(d, invalidate)
	s.update(d.SystemMenu())
	var mu sync.Mutex
	d.MenuChanged = func(next []desk.SystemMenuItem) { mu.Lock(); defer mu.Unlock(); s.update(next) }
}
