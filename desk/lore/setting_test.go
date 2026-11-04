package lore

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
	r := Route{}
	r.ShowWaiting()
	r.OpenEntry("a")
	r.EditChange("a1")
	r.EditEntry("a")
	r.ShowHistory()

	check := func(want Screen, entry, change string) {
		t.Helper()
		s, e, c := r.Screen()
		if s != want || e != entry || c != change {
			t.Fatalf("screen %d about %q/%q, want %d about %q/%q", s, e, c, want, entry, change)
		}
	}
	check(screenHistory, "", "")
	r.CloseHistory()
	check(screenEntryForm, "a", "")
	r.CloseForm()
	check(screenChange, "a", "a1")
	r.CloseChange()
	check(screenEntry, "a", "")
	r.CloseEntry()
	check(screenWaiting, "", "")
	r.ShowDropped()
	check(screenDropped, "", "")
	r.CloseDropped()
	check(screenList, "", "")

	// A new entry's form comes above an entry's page and a change's.
	r.OpenEntry("a")
	r.EditChange("a1")
	r.Answer("q")
	check(screenNewEntry, "", "")
	r.ShowHistory()
	check(screenHistory, "", "")
}

// Closing a layer shows the one below it.
func TestSettingRouteClosingShowsTheLayerBelow(t *testing.T) {
	r := Route{}
	r.JumpToEntry("a")
	r.EditChange("a1")
	r.EditEntry("a")
	r.ShowHistory()
	r.CloseHistory()
	if s, e, _ := r.Screen(); s != screenEntryForm || e != "a" {
		t.Fatalf("under the revisions: %d %q", s, e)
	}
	r.CloseForm()
	if s, e, c := r.Screen(); s != screenChange || e != "a" || c != "a1" {
		t.Fatalf("under the form: %d %q %q", s, e, c)
	}
	r.CloseChange()
	if s, e, _ := r.Screen(); s != screenEntry || e != "a" {
		t.Fatalf("under the change: %d %q", s, e)
	}
	r.CloseEntry()
	if s, _, _ := r.Screen(); s != screenList {
		t.Fatalf("under the entry: %d", s)
	}
	r.ShowWaiting()
	r.CloseWaiting()
	if s, _, _ := r.Screen(); s != screenList {
		t.Fatalf("under what waits: %d", s)
	}
	r.ShowDropped()
	r.CloseDropped()
	if s, _, _ := r.Screen(); s != screenList {
		t.Fatalf("under the deleted: %d", s)
	}
}

// The base is one screen: showing one of what waits and the deleted
// entries puts the other away, and closing the one that shows leaves the
// list.
func TestSettingRouteOneBase(t *testing.T) {
	r := Route{}
	r.ShowDropped()
	r.CloseWaiting()
	if s, _, _ := r.Screen(); s != screenDropped {
		t.Fatalf("closing what waits closed the deleted: %d", s)
	}
	r.ShowWaiting()
	r.CloseDropped()
	if s, _, _ := r.Screen(); s != screenWaiting {
		t.Fatalf("closing the deleted closed what waits: %d", s)
	}
	r.ShowDropped()
	r.ShowWaiting()
	if s, _, _ := r.Screen(); s != screenWaiting {
		t.Fatalf("what waits did not take the base: %d", s)
	}
	r.ShowDropped()
	if s, _, _ := r.Screen(); s != screenDropped {
		t.Fatalf("the deleted did not take the base: %d", s)
	}
}

// Each frame drops what points at nothing: the form of an entry gone, a
// new entry whose question is gone or no longer waits, the page of an
// entry gone with its change, and a change gone or no longer in the body.
func TestSettingRouteDropsWhatPointsAtNothing(t *testing.T) {
	set := routeSetting()
	screen := func(r *Route) (Screen, string, string) {
		t.Helper()
		r.Settle(set)
		return r.Screen()
	}

	r := Route{}
	r.OpenEntry("a")
	r.EditEntry("missing")
	if s, e, _ := screen(&r); s != screenEntry || e != "a" {
		t.Fatalf("form of an entry gone: %d %q", s, e)
	}

	r = Route{}
	r.Answer("missing")
	if s, _, _ := screen(&r); s != screenList {
		t.Fatalf("new form answering no question: %d", s)
	}

	r = Route{}
	r.Answer("b") // b is settled, not open
	if s, _, _ := screen(&r); s != screenList {
		t.Fatalf("new form answering a settled entry: %d", s)
	}

	r = Route{}
	r.ShowWaiting()
	r.OpenEntry("missing")
	r.EditChange("a1")
	if s, _, _ := screen(&r); s != screenWaiting {
		t.Fatalf("page of an entry gone: %d", s)
	}
	if c := r.ChangeID(); c != "" {
		t.Fatalf("its change stayed: %q", c)
	}

	r = Route{}
	r.OpenEntry("a")
	r.EditChange("missing")
	if s, e, _ := screen(&r); s != screenEntry || e != "a" {
		t.Fatalf("change gone: %d %q", s, e)
	}

	r = Route{}
	r.OpenEntry("a")
	r.EditChange("a2") // dropped, so not in the body
	if s, _, _ := screen(&r); s != screenEntry {
		t.Fatalf("change no longer in the body: %d", s)
	}

	r = Route{}
	r.OpenEntry("b")
	r.EditChange("a1") // the change of another entry
	if s, e, _ := screen(&r); s != screenEntry || e != "b" {
		t.Fatalf("change of another entry: %d %q", s, e)
	}
}

// A jump closes every layer; opening an entry's page keeps the layers
// under it.
func TestSettingRouteJumps(t *testing.T) {
	r := Route{}
	r.ShowWaiting()
	r.OpenEntry("a")
	r.EditEntry("a")
	r.EditChange("a1")
	r.ShowHistory()
	r.JumpToEntry("b")
	if s, e, _ := r.Screen(); s != screenEntry || e != "b" {
		t.Fatalf("jump to an entry: %d %q", s, e)
	}
	if r.ShowingHistory() || r.ShowingWaiting() || r.EditID() != "" || r.ChangeID() != "" {
		t.Fatalf("jump kept a layer: %+v", r)
	}

	r = Route{}
	r.ShowDropped()
	r.OpenEntry("a")
	r.EditEntry("a")
	r.ShowHistory()
	r.JumpToWaiting()
	if s, _, _ := r.Screen(); s != screenWaiting {
		t.Fatalf("jump to what waits: %d", s)
	}
	if r.ShowingHistory() || r.Entry() != "" || r.EditID() != "" || r.ChangeID() != "" {
		t.Fatalf("jump to what waits kept a layer: %+v", r)
	}

	r = Route{}
	r.ShowWaiting()
	r.OpenEntry("a")
	r.EditEntry("a")
	r.EditChange("a1")
	r.ShowHistory()
	r.OpenEntry("b")
	if s, _, _ := r.Screen(); s != screenHistory {
		t.Fatalf("opening an entry closed the revisions: %d", s)
	}
	if r.Entry() != "b" || r.EditID() != "a" || r.ChangeID() != "a1" || !r.ShowingWaiting() {
		t.Fatalf("opening an entry closed what is under it: %+v", r)
	}
}
