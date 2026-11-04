package workspace

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestCheckQueueOrder(t *testing.T) {
	q := CheckQueue{Language: Chinese, Failed: map[string]error{"a": errors.New("old failure")}}
	q.Enqueue("a")
	q.Enqueue("a")
	q.Enqueue("b")
	if !slices.Equal(q.Pending, []string{"a", "b"}) || q.Failed["a"] != nil {
		t.Fatal(q)
	}
	id, ok := q.Next()
	if !ok || id != "a" || q.Running != "a" {
		t.Fatal(q)
	}
	q.Enqueue("a")
	if _, ok := q.Next(); ok || len(q.Pending) != 1 {
		t.Fatal(q)
	}
	q.Finished()
	if id, ok := q.Next(); !ok || id != "b" {
		t.Fatal(q)
	}
	q.Finished()
	if _, ok := q.Next(); ok {
		t.Fatal(q)
	}
	q.Enqueue("c")
	q.Running = "running"
	q.Stop()
	if q.Running != "running" || q.Pending != nil {
		t.Fatal(q)
	}
}

func TestCheckQueuePrunable(t *testing.T) {
	all := map[string]Check{
		"empty": {}, "meant": {Findings: []Finding{{Meant: true}}},
		"open": {Findings: []Finding{{Quote: "x"}}}, "mixed": {Findings: []Finding{{Meant: true}, {Quote: "y"}}},
	}
	got := PrunableChapters(all)
	slices.Sort(got)
	if !slices.Equal(got, []string{"mixed", "open"}) {
		t.Fatal(got)
	}
	if got := PrunableChapters(nil); got != nil {
		t.Fatal(got)
	}
}

func TestCheckQueueDueAndLeave(t *testing.T) {
	tree := Tree{Root: &Node{Children: []*Node{{ID: "folder", Folder: true, Children: []*Node{{ID: "new"}, {ID: "same"}, {ID: "changed"}, {ID: "short"}, {ID: "missing"}}}}}}
	all := map[string]Check{"same": {Of: ProseHash("一样。")}, "changed": {Of: ProseHash("旧稿。")}}
	q := CheckQueue{Language: Chinese}
	texts := map[string]string{"folder": "不是章。", "new": "新稿。", "same": "一样。", "changed": "新稿。", "short": "短"}
	if got := DueChapters(all, tree, texts, 3, Chinese); !slices.Equal(got, []string{"new", "changed"}) {
		t.Fatal(got)
	}
	for _, id := range []string{"new", "same", "changed", "short"} {
		q.EnqueueStale(all, id, texts[id], 3)
	}
	if !slices.Equal(q.Pending, []string{"new", "changed"}) {
		t.Fatal(q)
	}
	if got := DueChapters(all, Tree{}, texts, 3, Chinese); got != nil {
		t.Fatal(got)
	}
}

func TestCheckQueueAfterPause(t *testing.T) {
	now := time.Date(2041, 1, 2, 0, 0, 0, 0, time.UTC)
	for _, ago := range []time.Duration{-time.Second, 8*time.Second - time.Nanosecond, 8 * time.Second, 9 * time.Second} {
		q := CheckQueue{Language: Chinese}
		q.AfterPause("a", "原句。", Check{}, 3, now.Add(-ago), now)
		want := ago >= 8*time.Second
		if (len(q.Pending) == 1) != want || (q.Tried["a"] != "") != want {
			t.Fatal(ago, q)
		}
	}
	q := CheckQueue{Language: Chinese}
	q.AfterPause("a", "短", Check{}, 3, now.Add(-time.Hour), now)
	if len(q.Tried) != 0 {
		t.Fatal(q)
	}
	q.AfterPause("a", "原句。", Check{}, 3, now.Add(-time.Hour), now)
	q.Pending = nil
	q.Fail("a", errors.New("failed"))
	q.AfterPause("a", "原句。", Check{}, 3, now.Add(-time.Hour), now.Add(time.Hour))
	if len(q.Pending) != 0 || failed(q, "a") != "failed" {
		t.Fatal("failed prose was resubmitted", q)
	}
	q.AfterPause("a", "新原句。", Check{}, 3, now.Add(-time.Hour), now)
	if !slices.Equal(q.Pending, []string{"a"}) || q.Failed["a"] != nil {
		t.Fatal(q)
	}
	q = CheckQueue{Language: Chinese}
	q.AfterPause("a", "原句。", Check{Of: ProseHash("原句。")}, 3, now.Add(-time.Hour), now)
	if len(q.Pending) != 0 || len(q.Tried) != 0 {
		t.Fatal(q)
	}
}

func TestCheckQueueKeep(t *testing.T) {
	old := Check{Of: "old", Findings: []Finding{{Kind: FindName, Quote: "名字", Name: "正确", Meant: true}}}
	was := Check{Of: "old", Findings: []Finding{{Kind: FindName, Quote: "名字", Name: "正确", Meant: true}}} // old as it was
	q := CheckQueue{Language: Chinese, Failed: map[string]error{"a": errors.New("earlier")}}
	for _, err := range []error{context.Canceled, errors.Join(errors.New("stopped"), context.Canceled)} {
		if _, keep := q.Keep("a", old, Check{}, err); keep || failed(q, "a") != "earlier" || !reflect.DeepEqual(old, was) {
			t.Fatal(q, keep)
		}
	}
	if _, keep := q.Keep("a", old, Check{}, errors.New("timeout")); keep || failed(q, "a") != "timeout" || !reflect.DeepEqual(old, was) {
		t.Fatal(q, keep)
	}
	fresh := Check{Of: "new", Findings: []Finding{{Kind: FindName, Quote: "名字", Name: "正确"}}}
	kept, keep := q.Keep("a", old, fresh, nil)
	if !keep || q.Failed["a"] != nil || !kept.Findings[0].Meant || kept.Of != "new" || !reflect.DeepEqual(old, was) {
		t.Fatal(kept, q)
	}
}

func TestCheckQueueLifecycle(t *testing.T) {
	var c CheckQueue
	c.Fail("old", errors.New("old failure"))
	c.AfterPause("old", "原句。", Check{}, 3, time.Time{}, time.Time{}.Add(time.Hour))
	c.Next()
	c.Enqueue("waiting")
	c.Fail("old", errors.New("old failure"))
	c.WorkChanged()
	if len(c.Failed) != 0 || len(c.Tried) != 0 || c.Running != "old" || !slices.Equal(c.Pending, []string{"waiting"}) {
		t.Fatal("a work change lost a running check", c)
	}
	c.Finished()
	if c.Running != "" || !slices.Equal(c.Pending, []string{"waiting"}) {
		t.Fatal("a finished result changed pending checks", c)
	}
	c.Next()
	c.Enqueue("later")
	c.CannotStart("waiting", errors.New("no pen"))
	if c.Running != "" || len(c.Pending) != 0 || failed(c, "waiting") != "no pen" {
		t.Fatal("a setup failure did not abandon the queue", c)
	}
}

// failed is the text of the chapter's failure, "" when none.
func failed(q CheckQueue, chapter string) string {
	if err := q.Failed[chapter]; err != nil {
		return err.Error()
	}
	return ""
}
