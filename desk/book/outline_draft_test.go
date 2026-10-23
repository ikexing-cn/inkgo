package book

import (
	"testing"
	"time"
)

func TestOutlineDraftWaitsAfterItsLastChange(t *testing.T) {
	now := time.Unix(100, 0)
	d := OutlineDraft{Node: "chapter", Text: "旧章纲", Saved: "旧章纲", Seen: "旧章纲"}
	if d.Change(now) {
		t.Fatal("unchanged text counted as typing")
	}
	if wait, due := d.Waits(time.Second, now); wait != 0 || due {
		t.Fatal(wait, due)
	}
	d.Text = "新章纲"
	if !d.Change(now) {
		t.Fatal("typing was not recorded")
	}
	if wait, due := d.Waits(time.Second, now.Add(time.Second/2)); wait != time.Second/2 || !due {
		t.Fatal(wait, due)
	}
	if d.Change(now.Add(time.Second / 2)) {
		t.Fatal("drawing restarted the pause")
	}
	if wait, due := d.Waits(time.Second, now.Add(time.Second)); wait != 0 || !due {
		t.Fatal(wait, due)
	}
	d.Node = ""
	if wait, due := d.Waits(time.Second, now); wait != 0 || due {
		t.Fatal(wait, due)
	}
}

func TestOutlineDraftSavesThroughBook(t *testing.T) {
	store := makeWork(t)
	id, err := store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	b := Open(store)
	d := OutlineDraft{Node: id, Text: "新的章纲"}
	if err := d.Save(b); err != nil {
		t.Fatal(err)
	}
	ch, err := store.ReadChapter(id)
	if err != nil || ch.Outline != "新的章纲" || b.Outlines()[id] != d.Text || d.Saved != d.Text {
		t.Fatal(ch, d, err)
	}
	version := b.Version()
	if err := d.Save(b); err != nil || b.Version() != version {
		t.Fatal("unchanged draft was written", err)
	}
	d.Node, d.Text = "missing", "不能保存的章纲"
	if err := d.Save(b); err == nil || d.Saved != "新的章纲" {
		t.Fatal("failed save marked the draft saved", d, err)
	}
	d.Node = ""
	if err := d.Save(nil); err != nil {
		t.Fatal(err)
	}
}
