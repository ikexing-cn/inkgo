package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A chapter's summary sits beside its prose, knows the prose it was made
// from, and goes where the chapter goes.
func TestSummaryFollowsItsChapter(t *testing.T) {
	s := testStore(t)
	vol := mustChapter(t, s, "", "第一卷", true)
	one := mustChapter(t, s, vol, "第一章", false)
	if _, err := s.WriteChapter(one, "她推开门。\n雨停了。", nil); err != nil {
		t.Fatal(err)
	}
	if sum, err := s.ReadSummary(one); err != nil || sum.Text != "" {
		t.Fatal(sum, err)
	}
	ch, _ := s.ReadChapter(one)
	at := time.Now().Truncate(time.Second)
	want := Summary{Text: "林晚推门，雨停。", Of: ProseHash(ch.Text), Words: Count(ch.Text), By: "deepseek/deepseek-flash", At: at}
	if err := s.WriteSummary(one, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadSummary(one)
	if err != nil || got.Text != want.Text || got.Of != want.Of || got.By != want.By || !got.At.Equal(at) || got.Stale(ch.Text) {
		t.Fatalf("%+v %v", got, err)
	}
	if !got.Stale("她推开门。") || got.Stale("她推开门。\r\n雨停了。\n") {
		t.Fatal("stale is not about the prose as stored")
	}
	if err := s.WriteSummary(vol, want); err == nil {
		t.Fatal("a folder took a summary")
	}
	if err := s.WriteSummary(one, Summary{Text: "x"}); err == nil {
		t.Fatal("a summary without its prose")
	}

	// It moves, goes to the trash and comes back with the chapter.
	if err := s.MoveChapter(one, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameChapter(one, "序章"); err != nil {
		t.Fatal(err)
	}
	if !exists(t, s.Root(), "chapters", "序章.summary.json") {
		t.Fatal("the summary did not follow")
	}
	if err := s.TrashChapter(one); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreChapter(one); err != nil {
		t.Fatal(err)
	}
	all, err := s.Summaries()
	if err != nil || all[one].Text != want.Text || len(all) != 1 {
		t.Fatal(all, err)
	}
	if err := s.WriteSummary(one, Summary{}); err != nil || exists(t, s.Root(), "chapters", "序章.summary.json") {
		t.Fatal("an empty summary stayed", err)
	}

	// A summary edited by hand into something else is an error, not lost.
	p := filepath.Join(s.Root(), "chapters", "序章.summary.json")
	if err := os.WriteFile(p, []byte(`{"text":"x","of":"y","extra":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadSummary(one); err == nil {
		t.Fatal("unknown field accepted")
	}
}
