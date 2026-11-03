package ext_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"writer/ext"
)

type localStore struct {
	off              []string
	calls            []string
	loadErr, saveErr error
}

func (s *localStore) Load() ([]string, error) {
	s.calls = append(s.calls, "load")
	return slices.Clone(s.off), s.loadErr
}
func (s *localStore) Save(off []string) error {
	s.calls = append(s.calls, "save")
	if s.saveErr != nil {
		return s.saveErr
	}
	s.off = slices.Clone(off)
	return nil
}

type bookStore struct {
	values           map[ext.Book]map[string]bool
	calls            []string
	loadErr, saveErr error
}

func (s *bookStore) Load(b ext.Book) (map[string]bool, error) {
	s.calls = append(s.calls, "load:"+b.Dir)
	return s.values[b], s.loadErr
}
func (s *bookStore) Save(b ext.Book, v map[string]bool) error {
	s.calls = append(s.calls, "save:"+b.Dir)
	if s.saveErr != nil {
		return s.saveErr
	}
	if s.values == nil {
		s.values = map[ext.Book]map[string]bool{}
	}
	s.values[b] = v
	return nil
}
func chain() []ext.Module {
	return []ext.Module{{ID: "c", Kind: ext.Library}, {ID: "b", Requires: []string{"c"}}, {ID: "a", Requires: []string{"b"}}}
}

func TestRequiredSwitchesCascadeAndPreserveOwnChoices(t *testing.T) {
	h := host(t, ext.Options{}, chain()...)
	must(t, h.Set("a", false))
	must(t, h.Set("c", false))
	state(t, h, "b", ext.Off)
	must(t, h.Set("c", true))
	state(t, h, "a", ext.Off)
	state(t, h, "b", ext.On)
	must(t, h.Set("b", false))
	must(t, h.Set("c", false))
	must(t, h.Set("a", true))
	for _, id := range []string{"c", "b", "a"} {
		state(t, h, id, ext.On)
	}
}
func TestStartsAndStopsMatchPrototypeTraversal(t *testing.T) {
	h := host(t, ext.Options{}, chain()...)
	if got := h.Stops("c"); !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Fatal(got)
	}
	must(t, h.Set("b", false))
	must(t, h.Set("c", false))
	if got := h.Starts("a"); !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Fatal(got)
	}
	if len(h.Stops("c")) != 0 || len(h.Starts("unknown")) != 0 {
		t.Fatal("inactive consumers or unknown ID included")
	}
}
func TestBookSwitchesAreIsolatedAndClearFollowsLocal(t *testing.T) {
	h := host(t, ext.Options{}, chain()...)
	one, two := ext.Book{Dir: "one"}, ext.Book{Dir: "two"}
	must(t, h.Open(one))
	must(t, h.SetBook("a", false))
	state(t, h, "a", ext.Off)
	must(t, h.Open(two))
	state(t, h, "a", ext.On)
	must(t, h.Open(one))
	state(t, h, "a", ext.Off)
	must(t, h.ClearBook("a"))
	state(t, h, "a", ext.On)
}
func TestBookEnableCascadesFeaturesInBookAndLibrariesLocally(t *testing.T) {
	ls := &localStore{off: []string{"a", "b", "c"}}
	bs := &bookStore{}
	h := host(t, ext.Options{LocalSwitches: ls, BookSwitches: bs}, chain()...)
	one := ext.Book{Dir: "one"}
	must(t, h.Open(one))
	must(t, h.SetBook("a", true))
	if !reflect.DeepEqual(ls.off, []string{"a", "b"}) || !reflect.DeepEqual(bs.values[one], map[string]bool{"a": true, "b": true}) {
		t.Fatalf("local %v book %v", ls.off, bs.values)
	}
	for _, id := range []string{"a", "b", "c"} {
		state(t, h, id, ext.On)
	}
	must(t, h.Open(ext.Book{Dir: "two"}))
	state(t, h, "a", ext.Off)
	state(t, h, "b", ext.Off)
	state(t, h, "c", ext.On)
}
func TestLibraryRejectsBookOverride(t *testing.T) {
	h := host(t, ext.Options{}, chain()...)
	must(t, h.Open(ext.Book{Dir: "one"}))
	if h.SetBook("c", false) == nil || h.ClearBook("c") == nil {
		t.Fatal("library accepted book switch")
	}
}
func TestBookSwitchRequiresOpenBook(t *testing.T) {
	h := host(t, ext.Options{}, chain()...)
	if h.SetBook("a", true) == nil || h.ClearBook("a") == nil {
		t.Fatal("shelf accepted book switch")
	}
}
func TestLocalSetKeepsExistingBookOverride(t *testing.T) {
	h := host(t, ext.Options{}, chain()...)
	must(t, h.Open(ext.Book{Dir: "one"}))
	must(t, h.SetBook("a", false))
	must(t, h.Set("a", true))
	state(t, h, "a", ext.Off)
	must(t, h.ClearBook("a"))
	state(t, h, "a", ext.On)
}
func TestSwitchStoresAreReadAndWrittenAtTheirScopes(t *testing.T) {
	one := ext.Book{Dir: "one"}
	ls := &localStore{off: []string{"a"}}
	bs := &bookStore{values: map[ext.Book]map[string]bool{one: {"a": true}}}
	h := host(t, ext.Options{LocalSwitches: ls, BookSwitches: bs}, ext.Module{ID: "a"})
	state(t, h, "a", ext.Off)
	must(t, h.Open(one))
	state(t, h, "a", ext.On)
	must(t, h.SetBook("a", false))
	must(t, h.ClearBook("a"))
	state(t, h, "a", ext.Off)
	must(t, h.Set("a", true))
	state(t, h, "a", ext.On)
	if !reflect.DeepEqual(ls.calls, []string{"load", "save"}) || !reflect.DeepEqual(bs.calls, []string{"load:one", "save:one", "save:one"}) {
		t.Fatalf("%v %v", ls.calls, bs.calls)
	}
}
func TestSwitchStorageErrorsDoNotSilentlyChangeLiveState(t *testing.T) {
	boom := errors.New("storage failed")
	if _, err := ext.New(ext.Options{LocalSwitches: &localStore{loadErr: boom}}, ext.Module{ID: "a"}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	ls := &localStore{}
	bs := &bookStore{}
	h := host(t, ext.Options{LocalSwitches: ls, BookSwitches: bs}, ext.Module{ID: "a"})
	ls.saveErr = boom
	if !errors.Is(h.Set("a", false), boom) {
		t.Fatal("lost local error")
	}
	state(t, h, "a", ext.On)
	bs.loadErr = boom
	if !errors.Is(h.Open(ext.Book{Dir: "one"}), boom) {
		t.Fatal("lost load error")
	}
	state(t, h, "a", ext.On)
	bs.loadErr = nil
	must(t, h.Open(ext.Book{Dir: "one"}))
	bs.saveErr = boom
	if !errors.Is(h.SetBook("a", false), boom) {
		t.Fatal("lost book error")
	}
	state(t, h, "a", ext.On)
}

func TestBookSaveFailureKeepsSuccessfulLocalLibrarySave(t *testing.T) {
	boom := errors.New("book write failed")
	ls := &localStore{off: []string{"a", "b", "c"}}
	bs := &bookStore{saveErr: boom}
	h := host(t, ext.Options{LocalSwitches: ls, BookSwitches: bs}, chain()...)
	must(t, h.Open(ext.Book{Dir: "one"}))
	if !errors.Is(h.SetBook("a", true), boom) {
		t.Fatal("lost save error")
	}
	state(t, h, "a", ext.Off)
	state(t, h, "b", ext.Off)
	state(t, h, "c", ext.On)
	if !reflect.DeepEqual(ls.off, []string{"a", "b"}) {
		t.Fatal(ls.off)
	}
}
