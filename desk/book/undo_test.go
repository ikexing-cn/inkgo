package book

import (
	"testing"
	"time"

	"writer/workspace"
)

func TestUnreviseUpdatesTheCopy(t *testing.T) {
	s := makeWork(t)
	b := Open(s)
	r, err := b.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "怕黑"}}})
	if err != nil {
		t.Fatal(err)
	}
	version := b.Version()
	if err := b.Unrevise(r.ID); err != nil {
		t.Fatal(err)
	}
	if b.Version() <= version || len(b.Setting().Entries) != 0 || len(b.Setting().Revisions) != 0 {
		t.Fatal("book kept the removed revision")
	}
	version = b.Version()
	if err := b.Unrevise(r.ID); err == nil || b.Version() != version {
		t.Fatal("refused undo changed the copy", err)
	}
}

func TestSaveCheckOverUpdatesOnlyOnSuccess(t *testing.T) {
	s := makeWork(t)
	one, err := s.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	b := Open(s)
	before := workspace.Check{Of: "prose", At: time.Now()}
	if err := b.SaveCheck(one, before); err != nil {
		t.Fatal(err)
	}
	after := before
	after.Words = 20
	if err := b.SaveCheckOver(one, after, before); err != nil {
		t.Fatal(err)
	}
	if !b.Checks()[one].Equal(after) {
		t.Fatal("copy missed successful save")
	}
	if err := b.CheckUnchanged(one, after); err != nil {
		t.Fatal(err)
	}
	if err := b.CheckUnchanged(one, before); err == nil {
		t.Fatal("stale result passed the guard")
	}
	version := b.Version()
	if err := b.SaveCheckOver(one, before, before); err == nil {
		t.Fatal("stale result replaced the copy")
	}
	if b.Version() != version || !b.Checks()[one].Equal(after) {
		t.Fatal("refused save changed the copy")
	}
	if err := b.SaveCheckOver(one, before, after); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadCheck(one)
	if err != nil || !got.Equal(before) || !b.Checks()[one].Equal(before) {
		t.Fatal(got, err)
	}
}
