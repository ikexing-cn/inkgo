package ext_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"writer/ext"
)

func TestOwnReportsOwnSwitchDespiteInactiveDependency(t *testing.T) {
	h := host(t, ext.Options{}, chain()...)
	must(t, h.Set("c", false))
	if !h.Own("a") || !h.Own("b") || h.Own("c") || h.Own("missing") {
		t.Fatal("Own returned activity instead of own switch")
	}
	state(t, h, "a", ext.Off)
	must(t, h.Open(ext.Book{Dir: "one"}))
	must(t, h.SetBook("a", false))
	if h.Own("a") {
		t.Fatal("Own ignored book override")
	}
	must(t, h.ClearBook("a"))
	if !h.Own("a") {
		t.Fatal("Own failed to follow local switch")
	}
}

func TestBookOverrideDistinguishesAbsentFalseAndTrue(t *testing.T) {
	h := host(t, ext.Options{}, chain()...)
	if _, ok := h.BookOverride("a"); ok {
		t.Fatal("override on shelf")
	}
	must(t, h.Open(ext.Book{Dir: "one"}))
	if _, ok := h.BookOverride("a"); ok {
		t.Fatal("synthetic override")
	}
	must(t, h.SetBook("a", false))
	if on, ok := h.BookOverride("a"); on || !ok {
		t.Fatal(on, ok)
	}
	must(t, h.SetBook("a", true))
	if on, ok := h.BookOverride("a"); !on || !ok {
		t.Fatal(on, ok)
	}
	for _, id := range []string{"c", "missing"} {
		if _, ok := h.BookOverride(id); ok {
			t.Fatal(id)
		}
	}
	must(t, h.ClearBook("a"))
	if _, ok := h.BookOverride("a"); ok {
		t.Fatal("cleared override remains")
	}
}

func TestBookReturnsCurrentBookAndShelf(t *testing.T) {
	h := host(t, ext.Options{})
	for _, b := range []ext.Book{{}, {Dir: "one"}, {Dir: "two"}, {}} {
		must(t, h.Open(b))
		if h.Book() != b {
			t.Fatal(h.Book(), b)
		}
	}
}

func TestSettingPersistsJSONAndNewHostRestoresConcreteType(t *testing.T) {
	type value struct {
		Count int
		Names []string
	}
	want := value{17, []string{"one", "two"}}
	store := &settingsStore{}
	var p *ext.Pref[value]
	m := ext.Module{ID: "a", Setup: func(r *ext.Registrar) { p = ext.Setting(r, "data", ext.Schema[value]{}) }}
	h := host(t, ext.Options{Settings: store}, m)
	p.Set(want)
	raw := store.values[ext.Book{}]["a.data"]
	if !json.Valid(raw) {
		t.Fatalf("not JSON: %s", raw)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["Count"] != float64(17) {
		t.Fatal(decoded)
	}
	h.Close()
	host(t, ext.Options{Settings: store}, m)
	if got := p.Get(); !reflect.DeepEqual(got, want) {
		t.Fatalf("%T %+v", got, got)
	}
}

func TestBadJSONSettingsUseDefault(t *testing.T) {
	for _, raw := range []string{"{broken", "\"wrong type\"", "1 trailing"} {
		t.Run(raw, func(t *testing.T) {
			store := &settingsStore{values: map[ext.Book]map[string][]byte{{}: {"a.count": []byte(raw)}}}
			var p *ext.Pref[int]
			host(t, ext.Options{Settings: store}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) { p = ext.Setting(r, "count", ext.Schema[int]{Default: 7}) }})
			if p.Get() != 7 {
				t.Fatal(p.Get())
			}
		})
	}
}
