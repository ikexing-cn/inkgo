package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteCheckOverGuardsOnlyItsChapter(t *testing.T) {
	s := testStore(t)
	one := mustChapter(t, s, "", "第一章", false)
	two := mustChapter(t, s, "", "第二章", false)
	before := Check{Of: "prose", By: "test", At: time.Now(), Findings: []Finding{{ID: "f", Kind: FindNew, Quote: "雨"}}}
	if err := s.WriteCheck(one, before); err != nil {
		t.Fatal(err)
	}
	after := before
	after.Findings = []Finding{{ID: "f", Kind: FindNew, Quote: "雨", Meant: true}}
	// An unrelated unreadable result does not participate in this save.
	if err := os.WriteFile(filepath.Join(s.Root(), "ext", "check", two+".json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteCheckOver(one, after, before); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteCheckOver(one, before, after); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadCheck(one)
	if err != nil || !got.Equal(before) {
		t.Fatal(got, err)
	}
	later := before
	later.At = later.At.Add(time.Second)
	if err := s.WriteCheck(one, later); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Root(), "ext", "check", one+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteCheckOver(one, before, after); err != ErrCheckChanged {
		t.Fatal(err)
	}
	kept, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, kept) {
		t.Fatal("a later result was overwritten", err)
	}
	if err := s.WriteCheckOver(two, before, Check{}); err == nil {
		t.Fatal("a damaged result was overwritten")
	}
}
