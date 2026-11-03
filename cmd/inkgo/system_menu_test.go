package main

import (
	"testing"
	"writer/desk"
)

type fakeMenuItem struct{ calls []string }

func (f *fakeMenuItem) SetLabel(string) { f.calls = append(f.calls, "label") }
func (f *fakeMenuItem) SetEnabled(bool) { f.calls = append(f.calls, "enabled") }
func (f *fakeMenuItem) SetChecked(bool) { f.calls = append(f.calls, "checked") }
func (f *fakeMenuItem) SetVisible(bool) { f.calls = append(f.calls, "visible") }

func TestSystemMenuPushesOnlyChanges(t *testing.T) {
	installs := 0
	items := map[string]*fakeMenuItem{}
	s := menuSync{install: func(rows []desk.SystemMenuItem) map[string]menuItemState {
		installs++
		out := map[string]menuItemState{}
		for _, r := range rows {
			f := &fakeMenuItem{}
			items[r.ID] = f
			out[r.ID] = f
		}
		return out
	}}
	state := func() []desk.SystemMenuItem {
		return []desk.SystemMenuItem{{ID: "one", Label: "One", Enabled: true, Visible: true}, {ID: "two", Enabled: true, Visible: true}}
	}
	s.update(state())
	s.update(state())
	if installs != 1 || len(items["one"].calls)+len(items["two"].calls) != 0 {
		t.Fatal("unchanged frame pushed")
	}
	next := state()
	next[1].Enabled = false
	s.update(next)
	if installs != 1 || len(items["one"].calls) != 0 || len(items["two"].calls) != 1 || items["two"].calls[0] != "enabled" {
		t.Fatal("not a selective update")
	}
	next = state()
	next[0].Label = "New"
	next[0].Checked = true
	next[0].Visible = false
	next[1].Enabled = false
	s.update(next)
	if len(items["one"].calls) != 3 || installs != 1 {
		t.Fatal("state rebuilt structure")
	}
	next = append(next, desk.SystemMenuItem{ID: "three"})
	s.update(next)
	if installs != 2 {
		t.Fatal("structure did not rebuild")
	}
}
