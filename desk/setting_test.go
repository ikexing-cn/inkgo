package desk

import (
	"testing"

	"writer/workspace"
)

// routeSetting is what the route is settled against: an entry with a
// change that holds and one dropped, another entry, and a question.
func routeSetting() workspace.Setting {
	return workspace.Setting{Entries: []workspace.Entry{
		{ID: "a", Place: workspace.PlaceBody, Text: "甲", Changes: []workspace.Change{
			{ID: "a1", Place: workspace.PlaceBody, From: "ch1", Text: "甲的旧样子"},
			{ID: "a2", Place: workspace.PlaceDropped, From: "ch1", Text: "甲的另一版"},
		}},
		{ID: "b", Place: workspace.PlaceBody, Text: "乙"},
		{ID: "q", Place: workspace.PlaceOpen, Text: "丙", Asked: "丙是谁"},
	}}
}

// The screens come in layers: the revisions over everything, then the
// new entry's form, an entry's form, a change's form, the entry's page,
// what waits, the deleted, and the list.
func TestSettingRouteScreenOrder(t *testing.T) {
	r := settingRoute{}
	r.showWaiting()
	r.openEntry("a")
	r.editChange("a1")
	r.editEntry("a")
	r.showHistory()

	check := func(want settingScreen, entry, change string) {
		t.Helper()
		s, e, c := r.screen()
		if s != want || e != entry || c != change {
			t.Fatalf("screen %d about %q/%q, want %d about %q/%q", s, e, c, want, entry, change)
		}
	}
	check(screenHistory, "", "")
	r.closeHistory()
	check(screenEntryForm, "a", "")
	r.closeForm()
	check(screenChange, "a", "a1")
	r.closeChange()
	check(screenEntry, "a", "")
	r.closeEntry()
	check(screenWaiting, "", "")
	r.showDropped()
	check(screenDropped, "", "")
	r.closeDropped()
	check(screenList, "", "")

	// A new entry's form comes above an entry's page and a change's.
	r.openEntry("a")
	r.editChange("a1")
	r.answer("q")
	check(screenNewEntry, "", "")
	r.showHistory()
	check(screenHistory, "", "")
}

// Closing a layer shows the one below it.
func TestSettingRouteClosingShowsTheLayerBelow(t *testing.T) {
	r := settingRoute{}
	r.jumpToEntry("a")
	r.editChange("a1")
	r.editEntry("a")
	r.showHistory()
	r.closeHistory()
	if s, e, _ := r.screen(); s != screenEntryForm || e != "a" {
		t.Fatalf("under the revisions: %d %q", s, e)
	}
	r.closeForm()
	if s, e, c := r.screen(); s != screenChange || e != "a" || c != "a1" {
		t.Fatalf("under the form: %d %q %q", s, e, c)
	}
	r.closeChange()
	if s, e, _ := r.screen(); s != screenEntry || e != "a" {
		t.Fatalf("under the change: %d %q", s, e)
	}
	r.closeEntry()
	if s, _, _ := r.screen(); s != screenList {
		t.Fatalf("under the entry: %d", s)
	}
	r.showWaiting()
	r.closeWaiting()
	if s, _, _ := r.screen(); s != screenList {
		t.Fatalf("under what waits: %d", s)
	}
	r.showDropped()
	r.closeDropped()
	if s, _, _ := r.screen(); s != screenList {
		t.Fatalf("under the deleted: %d", s)
	}
}

// The base is one screen: showing one of what waits and the deleted
// entries puts the other away, and closing the one that shows leaves the
// list.
func TestSettingRouteOneBase(t *testing.T) {
	r := settingRoute{}
	r.showDropped()
	r.closeWaiting()
	if s, _, _ := r.screen(); s != screenDropped {
		t.Fatalf("closing what waits closed the deleted: %d", s)
	}
	r.showWaiting()
	r.closeDropped()
	if s, _, _ := r.screen(); s != screenWaiting {
		t.Fatalf("closing the deleted closed what waits: %d", s)
	}
	r.showDropped()
	r.showWaiting()
	if s, _, _ := r.screen(); s != screenWaiting {
		t.Fatalf("what waits did not take the base: %d", s)
	}
	r.showDropped()
	if s, _, _ := r.screen(); s != screenDropped {
		t.Fatalf("the deleted did not take the base: %d", s)
	}
}

// Each frame drops what points at nothing: the form of an entry gone, a
// new entry whose question is gone or no longer waits, the page of an
// entry gone with its change, and a change gone or no longer in the body.
func TestSettingRouteDropsWhatPointsAtNothing(t *testing.T) {
	set := routeSetting()
	screen := func(r *settingRoute) (settingScreen, string, string) {
		t.Helper()
		r.settle(set)
		return r.screen()
	}

	r := settingRoute{}
	r.openEntry("a")
	r.editEntry("missing")
	if s, e, _ := screen(&r); s != screenEntry || e != "a" {
		t.Fatalf("form of an entry gone: %d %q", s, e)
	}

	r = settingRoute{}
	r.answer("missing")
	if s, _, _ := screen(&r); s != screenList {
		t.Fatalf("new form answering no question: %d", s)
	}

	r = settingRoute{}
	r.answer("b") // b is settled, not open
	if s, _, _ := screen(&r); s != screenList {
		t.Fatalf("new form answering a settled entry: %d", s)
	}

	r = settingRoute{}
	r.showWaiting()
	r.openEntry("missing")
	r.editChange("a1")
	if s, _, _ := screen(&r); s != screenWaiting {
		t.Fatalf("page of an entry gone: %d", s)
	}
	if c := r.changeID(); c != "" {
		t.Fatalf("its change stayed: %q", c)
	}

	r = settingRoute{}
	r.openEntry("a")
	r.editChange("missing")
	if s, e, _ := screen(&r); s != screenEntry || e != "a" {
		t.Fatalf("change gone: %d %q", s, e)
	}

	r = settingRoute{}
	r.openEntry("a")
	r.editChange("a2") // dropped, so not in the body
	if s, _, _ := screen(&r); s != screenEntry {
		t.Fatalf("change no longer in the body: %d", s)
	}

	r = settingRoute{}
	r.openEntry("b")
	r.editChange("a1") // the change of another entry
	if s, e, _ := screen(&r); s != screenEntry || e != "b" {
		t.Fatalf("change of another entry: %d %q", s, e)
	}
}

// A jump closes every layer; opening an entry's page keeps the layers
// under it.
func TestSettingRouteJumps(t *testing.T) {
	r := settingRoute{}
	r.showWaiting()
	r.openEntry("a")
	r.editEntry("a")
	r.editChange("a1")
	r.showHistory()
	r.jumpToEntry("b")
	if s, e, _ := r.screen(); s != screenEntry || e != "b" {
		t.Fatalf("jump to an entry: %d %q", s, e)
	}
	if r.showingHistory() || r.showingWaiting() || r.editID() != "" || r.changeID() != "" {
		t.Fatalf("jump kept a layer: %+v", r)
	}

	r = settingRoute{}
	r.showDropped()
	r.openEntry("a")
	r.editEntry("a")
	r.showHistory()
	r.jumpToWaiting()
	if s, _, _ := r.screen(); s != screenWaiting {
		t.Fatalf("jump to what waits: %d", s)
	}
	if r.showingHistory() || r.entry() != "" || r.editID() != "" || r.changeID() != "" {
		t.Fatalf("jump to what waits kept a layer: %+v", r)
	}

	r = settingRoute{}
	r.showWaiting()
	r.openEntry("a")
	r.editEntry("a")
	r.editChange("a1")
	r.showHistory()
	r.openEntry("b")
	if s, _, _ := r.screen(); s != screenHistory {
		t.Fatalf("opening an entry closed the revisions: %d", s)
	}
	if r.entry() != "b" || r.editID() != "a" || r.changeID() != "a1" || !r.showingWaiting() {
		t.Fatalf("opening an entry closed what is under it: %+v", r)
	}
}
