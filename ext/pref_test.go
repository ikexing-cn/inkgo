package ext_test

import (
	"reflect"
	"testing"

	"writer/ext"
)

type settingsStore struct {
	values map[ext.Book]map[string]any
	calls  []string
}

func (s *settingsStore) Load(b ext.Book, id string) (any, bool) {
	s.calls = append(s.calls, "load:"+b.Dir+":"+id)
	v, ok := s.values[b][id]
	return v, ok
}
func (s *settingsStore) Save(b ext.Book, id string, v any) {
	s.calls = append(s.calls, "save:"+b.Dir+":"+id)
	if s.values == nil {
		s.values = map[ext.Book]map[string]any{}
	}
	if s.values[b] == nil {
		s.values[b] = map[string]any{}
	}
	s.values[b][id] = v
}

func TestSettingDefaultsSetGetAndOnChange(t *testing.T) {
	var p *ext.Pref[int]
	var changes []int
	store := &settingsStore{}
	h := host(t, ext.Options{Settings: store}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		p = ext.Setting(r, "count", ext.Schema[int]{Default: 7})
		p.OnChange(func(v int) {
			if p.Get() != v {
				t.Error("notification before persistence")
			}
			changes = append(changes, v)
		})
	}})
	if p.Get() != 7 {
		t.Fatal(p.Get())
	}
	p.Set(9)
	if p.Get() != 9 || !reflect.DeepEqual(changes, []int{9}) || store.values[ext.Book{}]["a.count"] != 9 {
		t.Fatal(changes, store.values)
	}
	p.Set(9)
	must(t, h.Open(ext.Book{Dir: "one"}))
	if p.Get() != 9 || len(changes) != 1 {
		t.Fatal("local preference changed with book")
	}
}
func TestBookSettingFollowsCurrentBookAndDefaultsOnShelf(t *testing.T) {
	var p *ext.Pref[string]
	var changes []string
	store := &settingsStore{values: map[ext.Book]map[string]any{{Dir: "two"}: {"a.mode": "second"}}}
	h := host(t, ext.Options{Settings: store}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		p = ext.Setting(r, "mode", ext.Schema[string]{Default: "default", Scope: ext.BookScope})
		p.OnChange(func(v string) { changes = append(changes, v) })
	}})
	if p.Get() != "default" {
		t.Fatal(p.Get())
	}
	must(t, h.Open(ext.Book{Dir: "one"}))
	p.Set("first")
	must(t, h.Open(ext.Book{Dir: "two"}))
	if p.Get() != "second" {
		t.Fatal(p.Get())
	}
	must(t, h.Open(ext.Book{Dir: "one"}))
	if p.Get() != "first" {
		t.Fatal(p.Get())
	}
	must(t, h.Open(ext.Book{}))
	if p.Get() != "default" || !reflect.DeepEqual(changes, []string{"first", "second", "first", "default"}) {
		t.Fatal(p.Get(), changes)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("book Set accepted shelf")
			}
		}()
		p.Set("invalid")
	}()
}
func TestBothSettingUsesBookOverrideThenLocalThenDefault(t *testing.T) {
	var p *ext.Pref[int]
	h := host(t, ext.Options{}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) { p = ext.Setting(r, "mode", ext.Schema[int]{Default: 1, Scope: ext.Both}) }})
	if p.Get() != 1 {
		t.Fatal(p.Get())
	}
	p.Set(2)
	must(t, h.Open(ext.Book{Dir: "one"}))
	if p.Get() != 2 {
		t.Fatal(p.Get())
	}
	p.Set(3)
	must(t, h.Open(ext.Book{Dir: "two"}))
	if p.Get() != 2 {
		t.Fatal(p.Get())
	}
	p.Set(4)
	must(t, h.Open(ext.Book{Dir: "one"}))
	if p.Get() != 3 {
		t.Fatal(p.Get())
	}
	must(t, h.Open(ext.Book{}))
	if p.Get() != 2 {
		t.Fatal(p.Get())
	}
}
func TestSettingStoreRehydratesAndIgnoresWrongStoredType(t *testing.T) {
	store := &settingsStore{values: map[ext.Book]map[string]any{{}: {"a.count": "wrong type"}}}
	var p *ext.Pref[int]
	mod := ext.Module{ID: "a", Setup: func(r *ext.Registrar) { p = ext.Setting(r, "count", ext.Schema[int]{Default: 7}) }}
	h := host(t, ext.Options{Settings: store}, mod)
	if p.Get() != 7 {
		t.Fatal(p.Get())
	}
	p.Set(8)
	h.Close()
	host(t, ext.Options{Settings: store}, mod)
	if p.Get() != 8 {
		t.Fatal(p.Get())
	}
}
