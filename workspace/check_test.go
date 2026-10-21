package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestCheckRejectsTrailingContent(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "章节", false)
	path := filepath.Join(s.Root(), "chapters", "章节.check.json")
	for _, tail := range []string{"{}", "null", "trailing", "{"} {
		if err := os.WriteFile(path, []byte(`{"of":"hash"}`+tail), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ReadCheck(id); err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("ReadCheck accepted tail %q: %v", tail, err)
		}
		if _, err := s.Checks(); err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("Checks accepted tail %q: %v", tail, err)
		}
	}
}

func TestCheckPrune(t *testing.T) {
	original := Check{Of: "hash", Words: 12, Read: 8, By: "model", At: time.Date(2041, 1, 2, 3, 4, 5, 0, time.UTC), Sent: &Sent{Setting: 7}, Findings: []Finding{
		{ID: "one", Quote: "原句"}, {ID: "gone", Quote: "删掉了"}, {ID: "meant", Quote: "故意", Meant: true},
	}}
	for _, tc := range []struct {
		text    string
		ids     []string
		changed bool
	}{
		{"原句、删掉了、故意", []string{"one", "gone", "meant"}, false},
		{"原句。故意。", []string{"one", "meant"}, true},
		{"原句。", []string{"one"}, true}, {"", nil, true},
	} {
		got, changed := original.Prune(tc.text)
		var ids []string
		for _, f := range got.Findings {
			ids = append(ids, f.ID)
		}
		if !reflect.DeepEqual(ids, tc.ids) || changed != tc.changed {
			t.Fatal(tc, got, changed)
		}
		want := original
		want.Findings = got.Findings
		if !reflect.DeepEqual(got, want) || len(original.Findings) != 3 || original.Findings[1].ID != "gone" {
			t.Fatal("pruning changed metadata or source", got, original)
		}
	}
	if got, changed := (Check{}).Prune(""); changed || !reflect.DeepEqual(got, Check{}) {
		t.Fatal(got, changed)
	}
}

func TestCheckKeepsMeant(t *testing.T) {
	old := Finding{ID: "old", Kind: FindConflict, Quote: "原句", Entry: "entry", Name: "name", Meant: true}
	previous := Check{Findings: []Finding{old, {ID: "not-meant", Kind: FindNew, Quote: "新名字"}}}
	fresh := old
	fresh.ID = "fresh"
	fresh.Meant = false
	fresh.Title = "新标题"
	fresh.Says = "新解释"
	incoming := Check{Of: "new-hash", Words: 20, By: "new-model", At: time.Date(2041, 1, 2, 0, 0, 0, 0, time.UTC), Sent: &Sent{Setting: 8}, Findings: []Finding{fresh, fresh, {ID: "not-meant", Kind: FindNew, Quote: "新名字"}}}
	for _, kind := range []string{"kind", "quote", "entry", "name"} {
		different := fresh
		different.ID = kind
		switch kind {
		case "kind":
			different.Kind = FindName
		case "quote":
			different.Quote = "别的原句"
		case "entry":
			different.Entry = "other"
		case "name":
			different.Name = "other"
		}
		incoming.Findings = append(incoming.Findings, different)
	}
	got := incoming.KeepMeant(previous)
	for i, f := range got.Findings {
		if f.Meant != (i < 2) {
			t.Fatal(i, f)
		}
	}
	want := incoming
	want.Findings = got.Findings
	if !reflect.DeepEqual(got, want) || incoming.Findings[0].Meant || !previous.Findings[0].Meant {
		t.Fatal("merge mutated its inputs or metadata", got, incoming, previous)
	}
}

func TestCheckStaleKeepsReadProse(t *testing.T) {
	c := Check{Of: ProseHash("原句。"), At: time.Date(2041, 1, 2, 0, 0, 0, 0, time.UTC), Sent: &Sent{Setting: 3}}
	if c.Stale(" 原句。\n") || !c.Stale("原句。新句。") {
		t.Fatal(c)
	}
	c.Sent.Setting = 4
	if c.Stale("原句。") {
		t.Fatal("setting version changed the prose staleness rule")
	}
}
