package ext_test

import (
	"reflect"
	"testing"

	"writer/ext"
)

func TestDeclaredReadsInactiveContributorsAndPointOwnersWithoutChangingRevision(t *testing.T) {
	point := ext.PointKey[string]("base.items")
	h := host(t, ext.Options{LocalSwitches: &localStore{off: []string{"base", "extra"}}},
		ext.Module{ID: "base", Setup: func(r *ext.Registrar) { ext.Declare(r, point) }},
		ext.Module{ID: "extra", Optional: []string{"base"}, Setup: func(r *ext.Registrar) {
			ext.Contribute(r, point, "one", "first")
			ext.Contribute(r, point, "two", "second")
		}},
		ext.Module{ID: "last", Optional: []string{"base"}, Setup: func(r *ext.Registrar) {
			ext.Contribute(r, point, "three", "third")
		}})
	for _, switches := range [][2]bool{{false, false}, {false, true}, {true, false}, {true, true}} {
		must(t, h.Set("base", switches[0]))
		must(t, h.Set("extra", switches[1]))
		before := h.Changes()
		declared := ext.Declared(h, point)
		active := ext.Items(h, point)
		if len(declared) != 3 || declared[0].Value != "first" || declared[1].Value != "second" || declared[2].ID != "last.three" {
			t.Fatal(declared)
		}
		want := 0
		if switches[0] {
			want = 1
			if switches[1] {
				want = 3
			}
		}
		if len(active) != want || h.Changes() != before {
			t.Fatal(active, before, h.Changes())
		}
		if want == 3 && !reflect.DeepEqual(declared, active) {
			t.Fatal(declared, active)
		}
	}
}

func TestDeclaredUsesTheSameReplacementAsItems(t *testing.T) {
	var calls ext.CallPoint[command, int, int]
	h := host(t, commandOptions(&calls), exporter(), replacer("replacement", 10))
	before := h.Changes()
	declared := ext.Declared(h, commandPoint)
	active := ext.Items(h, commandPoint)[0]
	if declared[0].ID != active.ID || declared[0].Owner != active.Owner || declared[0].ReplacedBy != active.ReplacedBy || declared[0].Value.Title != active.Value.Title || declared[0].ReplacedBy != "replacement.replace" || declared[0].Value.Title != "replacement" {
		t.Fatal(declared, active)
	}
	if h.Changes() != before {
		t.Fatal("read changed revision")
	}
	must(t, h.Set("replacement", false))
	if got := ext.Declared(h, commandPoint)[0]; got.ReplacedBy != "" || got.Value.Title != "Original" {
		t.Fatal(got)
	}
}
