package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desk.json")
	if p := Load(path); !p.Writing {
		t.Fatal("new work should open on prose")
	}
	for _, tc := range []struct {
		data, tab string
		writing   bool
	}{
		{`{}`, "", false},
		{`{`, "", true},
		{`{"talk_tab":true}`, "talk", false},
		{`{"talk_tab":true,"fore_tab":true}`, "fore", false},
		{`{"pane_tab":"","fore_tab":true}`, "", false},
		{`{"pane_tab":"phone","talk_tab":true,"writing":true}`, "phone", true},
	} {
		if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
			t.Fatal(err)
		}
		if p := Load(path); p.PaneTab != tc.tab || p.Writing != tc.writing {
			t.Fatalf("%s: %+v", tc.data, p)
		}
	}
}

func TestPreferencesKeepJSONValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desk.json")
	p := Prefs{SettingHidden: true, Chapter: "chapter", Tabs: []string{"chapter"}, Writing: true, PaneTab: "phone", Carets: map[string]int{"chapter": 3}}
	want := `{"setting_hidden":true,"chapter":"chapter","tabs":["chapter"],"writing":true,"pane_tab":"phone","carets":{"chapter":3}}`
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", got, err)
	}
	var gotJSON, wantJSON any
	if err := json.Unmarshal(got, &gotJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &wantJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Fatalf("got %s, want %s", got, want)
	}
	if err := json.Unmarshal(got, &p); err != nil {
		t.Fatal(err)
	}
	if q := Load(path); q.Carets["chapter"] != 3 {
		t.Fatal(q)
	}
	if err := (Prefs{}).Save(filepath.Join(path, "desk.json")); err == nil {
		t.Fatal("write failure lost")
	}
}

// What the desk remembers of a work is written whole: the file is put in
// place of the last, with nothing left beside it.
func TestPrefsAreWrittenWhole(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".writer", "desk.json")
	p := Prefs{Chapter: "chapter"}
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), p.Chapter) {
		t.Fatalf("%s, %v", b, err)
	}
	if _, err = os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("a file left beside it: %v", err)
	}
}
