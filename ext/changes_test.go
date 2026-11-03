package ext_test

import (
	"testing"

	"writer/ext"
)

func TestChangesTracksMutationsButNotReads(t *testing.T) {
	key := ext.HookKey[int, int]("base.call")
	point := ext.PointKey[int]("host.items")
	h := host(t, ext.Options{}, ext.Module{ID: "base", Setup: func(r *ext.Registrar) { ext.DeclareHook(r, key); ext.Contribute(r, point, "item", 1) }}, ext.Module{ID: "injector", Requires: []string{"base"}, Setup: func(r *ext.Registrar) {
		ext.Before(r, key, "first", func(*int) {})
		ext.Before(r, key, "second", func(*int) {})
		ext.Overwrite(r, key, "one", func(v int) int { return v })
		ext.Overwrite(r, key, "two", func(v int) int { return v })
	}})
	check := func(name string, f func()) {
		t.Helper()
		before := h.Changes()
		f()
		if h.Changes() <= before {
			t.Fatal(name, "did not change the revision")
		}
	}
	check("off", func() { must(t, h.Set("injector", false)) })
	check("on", func() { must(t, h.Set("injector", true)) })
	check("book", func() { must(t, h.Open(ext.Book{Dir: "one"})) })
	check("switch book", func() { must(t, h.Open(ext.Book{Dir: "two"})) })
	check("order", func() { must(t, h.SetOrder(string(key), []string{"injector.second", "injector.first"})) })
	check("choice", func() { must(t, h.Choose(string(key), "injector.two")) })
	before := h.Changes()
	h.Modules()
	h.Ledger("base")
	h.Chain(string(key))
	h.Book()
	h.Own("base")
	ext.Items(h, point)
	_, err := ext.Call(h, key, 1, func(v int) int { return v })
	must(t, err)
	if h.Changes() != before {
		t.Fatal("reads changed the revision")
	}
	if allocs := testing.AllocsPerRun(100, func() { h.Changes() }); allocs != 0 {
		t.Fatal("Changes allocated", allocs)
	}
	must(t, h.Set("injector", false))
	check("call failure", func() {
		_, err := ext.Call(h, key, 1, func(int) int { panic("broken") })
		if err == nil {
			t.Fatal("panic not isolated")
		}
	})
	check("close", h.Close)
}
