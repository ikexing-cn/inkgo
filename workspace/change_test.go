package workspace

import (
	"testing"
)

// An entry reads, at a chapter, as its last change from that chapter or
// one before it in the tree, a folder's from its first chapter on; a
// change whose chapter is in the trash does not hold till it is back.
func TestAnEntryChangesFromAChapter(t *testing.T) {
	s := testStore(t)
	node := func(parent, title string, folder bool) string {
		id, err := s.NewChapter(parent, title, folder)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	one := node("", "第一卷", true)
	ch1, ch2 := node(one, "第1章", false), node(one, "第2章", false)
	two := node("", "第二卷", true)
	ch3, ch4 := node(two, "第3章", false), node(two, "第4章", false)
	r := mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: "人物", Text: "林晚用左手拧灯座。", Names: []string{"林晚"}}}})
	lin := r.Ops[0].Entry
	r = mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{
		{Op: OpChange, Entry: lin, From: ch2, Place: PlaceBody, Text: "林晚丢了左手。"},
		{Op: OpChange, Entry: lin, From: two, Place: PlaceBody, Text: "林晚装了假手。"},
	}})
	lost, fake := r.Ops[0].Change, r.Ops[1].Change
	if lost == "" || fake == "" || lost == fake {
		t.Fatalf("the changes have IDs %q %q", lost, fake)
	}
	at := func(want map[string]string, ahead map[string]int) {
		t.Helper()
		set, err := s.Setting()
		if err != nil {
			t.Fatal(err)
		}
		tree, err := s.Chapters()
		if err != nil {
			t.Fatal(err)
		}
		for n, text := range want {
			now, k := set.At(tree.Order(), n)
			if got := now.Entries[0].Text; got != text || len(now.Entries[0].Changes) > 0 || k != ahead[n] {
				t.Errorf("at %s: %q, %d ahead; want %q, %d", n, got, k, text, ahead[n])
			}
		}
	}
	at(map[string]string{ch1: "林晚用左手拧灯座。", ch2: "林晚丢了左手。", ch3: "林晚装了假手。", ch4: "林晚装了假手。", one: "林晚用左手拧灯座。"},
		map[string]int{ch1: 2, ch2: 1, ch3: 0, ch4: 0, one: 2})
	if e := entry(t, s, lin); len(e.Changes) != 2 || !e.Changes[0].Author || e.Changes[0].From != ch2 {
		t.Fatalf("%+v", e.Changes)
	}

	// The tree says the order: the fourth chapter put first comes before
	// both changes.
	if err := s.MoveChapter(ch4, "", 0); err != nil {
		t.Fatal(err)
	}
	at(map[string]string{ch4: "林晚用左手拧灯座。", ch3: "林晚装了假手。"}, map[string]int{ch4: 2})
	if err := s.MoveChapter(ch4, two, 1); err != nil {
		t.Fatal(err)
	}

	// In the trash a chapter's change holds no more, and back it does.
	if err := s.TrashChapter(ch2); err != nil {
		t.Fatal(err)
	}
	at(map[string]string{ch1: "林晚用左手拧灯座。", ch3: "林晚装了假手。"}, map[string]int{ch1: 1})
	if _, err := s.Revise(Revision{By: ByAuthor, Ops: []Op{{Op: OpEditChange, Entry: lin, Change: fake, From: ch2}}}); err == nil {
		t.Error("a change was moved to a chapter in the trash")
	}
	if err := s.RestoreChapter(ch2); err != nil {
		t.Fatal(err)
	}
	at(map[string]string{ch2: "林晚丢了左手。"}, map[string]int{ch2: 1})

	// One the AI suggests waits, and holds once the author takes it; the
	// change it would take the place of is dropped with it.
	r = mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpChange, Entry: lin, From: two, Place: PlaceOpen, Text: "林晚换了一只铁手。"}}})
	idea := r.Ops[0].Change
	at(map[string]string{ch3: "林晚装了假手。"}, nil)
	if _, err := s.Revise(Revision{By: ByAuthor, Ops: []Op{{Op: OpEditChange, Entry: lin, Change: idea, Place: PlaceBody}}}); err == nil {
		t.Error("two changes hold from one volume")
	}
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpDropChange, Entry: lin, Change: fake}, {Op: OpEditChange, Entry: lin, Change: idea, Place: PlaceBody}}})
	at(map[string]string{ch3: "林晚换了一只铁手。"}, nil)

	// The author edits a change's chapter and text, and deletes it.
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpEditChange, Entry: lin, Change: lost, From: ch3, Text: "林晚的左手好了。"}}})
	at(map[string]string{ch2: "林晚用左手拧灯座。", ch3: "林晚的左手好了。"}, map[string]int{ch2: 2})
	mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpDropChange, Entry: lin, Change: lost}}})
	at(map[string]string{ch3: "林晚换了一只铁手。"}, nil)

	r = mustRevise(t, s, Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceOpen, Category: "人物", Text: "老周是林晚的邻居。"}}})
	for _, op := range []Op{
		{Op: OpChange, Entry: r.Ops[0].Entry, From: ch1, Place: PlaceBody, Text: "x"}, // of an open entry
		{Op: OpChange, Entry: lin, From: ch1, Place: PlaceBody},                       // no text
		{Op: OpChange, Entry: lin, From: ch1, Place: PlaceDropped, Text: "x"},
		{Op: OpChange, Entry: lin, From: "nope", Place: PlaceBody, Text: "x"},
		{Op: OpChange, Entry: lin, From: ch1, Place: PlaceBody, Text: "x", Category: "人物"},
		{Op: OpEditChange, Entry: lin, Change: lost, Text: "x"}, // dropped
		{Op: OpEditChange, Entry: lin, Change: idea},
		{Op: OpEditChange, Entry: lin, Change: idea, Place: PlaceOpen},
		{Op: OpDropChange, Entry: lin, Change: "nope"},
		{Op: OpDropChange, Entry: lin, Change: idea, Text: "x"},
	} {
		if _, err := s.Revise(Revision{By: ByAuthor, Ops: []Op{op}}); err == nil {
			t.Errorf("took %+v", op)
		}
	}
}
