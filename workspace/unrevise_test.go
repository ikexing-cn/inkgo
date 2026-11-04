package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestUnreviseRestoresTheSettingAndItsBytes(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n", ""} {
		t.Run(strings.ReplaceAll(ending, "\n", "LF"), func(t *testing.T) {
			s := testStore(t)
			r := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "怕黑"}}})
			path := filepath.Join(s.Root(), "setting.jsonl")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			before := append(bytes.TrimRight(data, "\r\n"), []byte(ending)...)
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			want, err := s.Setting()
			if err != nil {
				t.Fatal(err)
			}
			edit := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpEdit, Entry: r.Ops[0].Entry, Text: "不再怕黑"}}})
			if err := s.Unrevise(edit.ID); err != nil {
				t.Fatal(err)
			}
			got, err := s.Setting()
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatal("setting did not return", got, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("prefix bytes changed", string(before), string(after), err)
			}
			// A new revision still appends after undo.
			mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpEdit, Entry: r.Ops[0].Entry, Text: "怕雨"}}})
		})
	}
}

func TestUnreviseRefusesALaterRevision(t *testing.T) {
	s := testStore(t)
	first := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "怕黑"}}})
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpEdit, Entry: first.Ops[0].Entry, Text: "怕雨"}}})
	path := filepath.Join(s.Root(), "setting.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Unrevise(first.ID); err != ErrSettingChanged {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refused undo wrote the file", err)
	}
}

func TestUnreviseTheOnlyRevision(t *testing.T) {
	s := testStore(t)
	r := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "怕黑"}}})
	if err := s.Unrevise(r.ID); err != nil {
		t.Fatal(err)
	}
	set, err := s.Setting()
	if err != nil || len(set.Entries) != 0 || len(set.Revisions) != 0 {
		t.Fatal(set, err)
	}
	data, err := os.ReadFile(filepath.Join(s.Root(), "setting.jsonl"))
	if err != nil || len(data) != 0 {
		t.Fatal("empty setting changed", string(data), err)
	}
	if err := s.Unrevise(r.ID); err == nil {
		t.Fatal("removed a revision twice")
	}
}
