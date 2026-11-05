package ext_test

import (
	"reflect"
	"testing"
	"writer/ext"
)

func TestReadPointAssemblyValidation(t *testing.T) {
	const point ext.PointKey[int] = "base.values"
	for _, tc := range []struct {
		name        string
		key         string
		wrong, fail bool
	}{
		{name: "empty", key: string(point)}, {name: "typo", key: "base.value", fail: true}, {name: "type", key: string(point), wrong: true, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, err := ext.New(ext.Options{}, ext.Module{ID: "base", Setup: func(r *ext.Registrar) { ext.Declare(r, point) }}, ext.Module{ID: "reader", Requires: []string{"base"}, Setup: func(r *ext.Registrar) {
				if tc.wrong {
					ext.ReadPoint(r, ext.PointKey[string](tc.key))
				} else {
					ext.ReadPoint(r, ext.PointKey[int](tc.key))
				}
			}})
			if (err != nil) != tc.fail {
				t.Fatalf("assembly: %v", err)
			}
			if h != nil {
				defer h.Close()
				if !reflect.DeepEqual(h.Ledger("reader").Reads, []string{tc.key}) {
					t.Fatal(h.Ledger("reader"))
				}
			}
		})
	}
}

func TestSingleCapabilityHasNoFallbackOnConflict(t *testing.T) {
	const point ext.PointKey[int] = "base.capability"
	for count := 0; count <= 2; count++ {
		mods := []ext.Module{{ID: "base", Setup: func(r *ext.Registrar) { ext.DeclareSingle(r, point) }}}
		for _, id := range []string{"one", "two"}[:count] {
			mods = append(mods, ext.Module{ID: id, Requires: []string{"base"}, Setup: func(r *ext.Registrar) { ext.Contribute(r, point, "candidate", 1) }})
		}
		h, err := ext.New(ext.Options{}, mods...)
		if err != nil {
			t.Fatal(err)
		}
		items := ext.Items(h, point)
		if (len(items) == 1) != (count == 1) {
			t.Fatal(count, items)
		}
		ledger := h.Ledger("base").Capabilities
		if len(ledger) != 1 || len(ledger[0].Candidates) != count || ledger[0].Conflict != (count == 2) {
			t.Fatal(ledger)
		}
		if count == 2 {
			if err := h.Set("two", false); err != nil {
				t.Fatal(err)
			}
			if len(ext.Items(h, point)) != 1 {
				t.Fatal("remaining candidate unavailable")
			}
		}
		h.Close()
	}
}

type operations struct {
	first  func(int) int
	second func(string) string
}

func TestNamedPointOperationsHaveIndependentInjectionChains(t *testing.T) {
	const point ext.PointKey[operations] = "base.operations"
	var reader *ext.PointReader[operations]
	var trace []string
	h, err := ext.New(ext.Options{}, ext.Module{ID: "base", Setup: func(r *ext.Registrar) {
		ext.Declare(r, point)
		ext.DeclareOperation(r, point, "first", func(o operations) func(int) int { return o.first })
		ext.DeclareOperation(r, point, "second", func(o operations) func(string) string { return o.second })
		ext.Contribute(r, point, "value", operations{first: func(n int) int { trace = append(trace, "original"); return n * 2 }, second: func(s string) string { return s + "!" }})
	}}, ext.Module{ID: "caller", Requires: []string{"base"}, Setup: func(r *ext.Registrar) { reader = ext.ReadPoint(r, point) }}, ext.Module{ID: "injector", Requires: []string{"base"}, Setup: func(r *ext.Registrar) {
		key := ext.HookOf(ext.CallPoint[operations, int, int]{Point: point, Operation: "first"}, "base.value")
		ext.Before(r, key, "before", func(n *int) { trace = append(trace, "before"); *n++ })
		ext.Around(r, key, "around", func(n int, next func(int) int) int { trace = append(trace, "around"); return next(n) + 1 })
		ext.After(r, key, "after", func(_ int, n *int) { trace = append(trace, "after"); *n += 10 })
		key2 := ext.HookOf(ext.CallPoint[operations, string, string]{Point: point, Operation: "second"}, "base.value")
		ext.Overwrite(r, key2, "overwrite", func(s string) string { return s + " replaced" })
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if len(h.Ledger("base").Calls) != 2 {
		t.Fatal(h.Ledger("base").Calls)
	}
	item := reader.Items()[0]
	n, err := ext.CallReadOperation(reader, "first", item.ID, 2, item.Value.first)
	if err != nil || n != 17 {
		t.Fatal(n, err)
	}
	if !reflect.DeepEqual(trace, []string{"before", "around", "original", "after"}) {
		t.Fatal(trace)
	}
	s, err := ext.CallReadOperation(reader, "second", item.ID, "value", item.Value.second)
	if err != nil || s != "value replaced" {
		t.Fatal(s, err)
	}
}
