package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A chapter's check sits beside its prose, knows the prose it read, and
// goes where the chapter goes.
func TestCheckFollowsItsChapter(t *testing.T) {
	s := testStore(t)
	vol := mustChapter(t, s, "", "第一卷", true)
	one := mustChapter(t, s, vol, "第一章", false)
	if _, err := s.WriteChapter(one, "她一个人走进了没有灯的巷子。", nil); err != nil {
		t.Fatal(err)
	}
	if k, err := s.ReadCheck(one); err != nil || k.Of != "" {
		t.Fatal(k, err)
	}
	ch, _ := s.ReadChapter(one)
	want := Check{Of: ProseHash(ch.Text), Words: Count(ch.Text), By: "deepseek/deepseek-flash", At: time.Now().Truncate(time.Second),
		Findings: []Finding{{ID: NewID(), Kind: FindConflict, Title: "林晚怕黑", Quote: "一个人走进了没有灯的巷子", Entry: NewID(), Says: "她很怕黑", Meant: true}}}
	if err := s.WriteCheck(one, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadCheck(one)
	if err != nil || got.Of != want.Of || len(got.Findings) != 1 || got.Findings[0] != want.Findings[0] || got.Stale(ch.Text) || !got.Stale("她走了。") {
		t.Fatalf("%+v %v", got, err)
	}
	if err := s.WriteCheck(vol, want); err == nil {
		t.Fatal("a folder took a check")
	}
	for _, bad := range []Finding{{Kind: FindNew, Quote: "x"}, {ID: "a", Kind: "odd", Quote: "x"}, {ID: "a", Kind: FindName, Quote: " "}} {
		if err := s.WriteCheck(one, Check{Of: want.Of, Findings: []Finding{bad}}); err == nil {
			t.Fatalf("kept %+v", bad)
		}
	}

	// It moves, goes to the trash and comes back with the chapter.
	if err := s.MoveChapter(one, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameChapter(one, "序章"); err != nil {
		t.Fatal(err)
	}
	if !exists(t, s.Root(), "chapters", "序章.check.json") {
		t.Fatal("the check did not follow")
	}
	if err := s.TrashChapter(one); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreChapter(one); err != nil {
		t.Fatal(err)
	}
	all, err := s.Checks()
	if err != nil || len(all) != 1 || len(all[one].Findings) != 1 {
		t.Fatal(all, err)
	}

	// A check edited by hand into something else is an error, not lost.
	p := filepath.Join(s.Root(), "chapters", "序章.check.json")
	if err := os.WriteFile(p, []byte(`{"of":"y","extra":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadCheck(one); err == nil {
		t.Fatal("unknown field accepted")
	}
	if err := s.WriteCheck(one, Check{}); err != nil || exists(t, s.Root(), "chapters", "序章.check.json") {
		t.Fatal("an empty check stayed", err)
	}
}
