package archive

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"writer/shelf"
	"writer/workspace"
)

func TestBackupRoundTripKeepsLoreCardsChapterAndVolumeOutlinesAndBookSettings(t *testing.T) {
	root := t.TempDir()
	s, err := workspace.Create(root, "book", "网文")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	vol, err := s.NewChapter("", "volume", true)
	check(err)
	id, err := s.NewChapter(vol, "chapter", false)
	check(err)
	_, err = s.WriteChapter(id, "prose", nil)
	check(err)
	check(s.WriteOutline(vol, "volume outline"))
	check(s.WriteOutline(id, "chapter outline"))
	_, err = s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: s.Work().Categories[0].Name, Text: "lore"}}})
	check(err)
	line, err := s.SaveLine(workspace.PlotLine{Name: "storyline"})
	check(err)
	_, err = s.SaveCard(workspace.Card{Title: "card", Chapter: id, Line: line.ID}, "")
	check(err)
	settings := []byte(`{"switches":{"lore":false,"outline":true},"settings":{"outline.example":{"value":42}}}`)
	check(os.WriteFile(workspace.ExtSettingsPath(root), settings, 0600))
	files, err := Read(root)
	check(err)
	meta := files.Meta(s.Work().ID)
	if meta.Chapters != 1 || meta.Words != 5 {
		t.Fatal(meta)
	}
	backup, made, err := Make(files, t.TempDir(), "book", meta, 2, time.Now())
	check(err)
	if !made {
		t.Fatal("backup not created")
	}
	into := t.TempDir()
	check(Unpack(backup.Path, into))
	verifyExtensionBook(t, s, into, vol, id, settings)
}

func TestImportingCopiedBookKeepsExtensionDataAndChapterIDs(t *testing.T) {
	s, err := workspace.Create(t.TempDir(), "book", "网文")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	vol, err := s.NewChapter("", "volume", true)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.NewChapter(vol, "chapter", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteOutline(id, "chapter outline"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteOutline(vol, "volume outline"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: s.Work().Categories[0].Name, Text: "lore"}}}); err != nil {
		t.Fatal(err)
	}
	line, err := s.SaveLine(workspace.PlotLine{Name: "line"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveCard(workspace.Card{Title: "card", Chapter: id, Line: line.ID}, ""); err != nil {
		t.Fatal(err)
	}
	settings := []byte(`{"switches":{"lore":false},"settings":{"outline.example":42}}`)
	if err := os.WriteFile(workspace.ExtSettingsPath(s.Root()), settings, 0600); err != nil {
		t.Fatal(err)
	}
	into := t.TempDir()
	if err := os.CopyFS(into, os.DirFS(s.Root())); err != nil {
		t.Fatal(err)
	}
	var library shelf.Shelf
	if _, added, err := library.Add(into); err != nil || !added {
		t.Fatal("import copied work", added, err)
	}
	verifyExtensionBook(t, s, into, vol, id, settings)
}

func verifyExtensionBook(t *testing.T, source *workspace.Store, root, vol, id string, settings []byte) {
	t.Helper()
	s, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	wantSet, err := source.Setting()
	if err != nil {
		t.Fatal(err)
	}
	set, err := s.Setting()
	if err != nil || !reflect.DeepEqual(set, wantSet) {
		t.Fatal("lore", set, err)
	}
	wantPlan, err := source.Plan()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.Plan()
	if err != nil || !reflect.DeepEqual(plan, wantPlan) {
		t.Fatal("cards/storylines", plan, err)
	}
	for _, n := range []string{vol, id} {
		if n == "" {
			continue
		}
		want, err := source.ReadChapter(n)
		if err != nil {
			t.Fatal(err)
		}
		ch, err := s.ReadChapter(n)
		if err != nil || ch.Outline != want.Outline || ch.Text != want.Text {
			t.Fatal("chapter/volume", ch, err)
		}
	}
	got, err := os.ReadFile(filepath.Join(root, "ext.json"))
	if err != nil || !bytes.Equal(got, settings) {
		t.Fatalf("switches/settings = %s, %v", got, err)
	}
}
