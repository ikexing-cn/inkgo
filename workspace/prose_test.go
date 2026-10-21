package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// Snapshots skip a text the latest holds, and after a week keep the last
// of each day.
func TestSnapshotsAndTheirKeeping(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "一", false)
	now := time.Now()
	old := now.Add(-10 * 24 * time.Hour)
	day := time.Date(old.Year(), old.Month(), old.Day(), 9, 0, 0, 0, time.Local)
	for i, text := range []string{"早上", "中午", "晚上"} {
		if _, err := s.takeSnapshot(id, SnapInterval, text, nil, nil, day.Add(time.Duration(i)*4*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	took, err := s.TakeSnapshot(id, SnapOpen, "今天", []ChapterMark{{Kind: MarkForeshadow, Start: 0, End: 1}})
	if err != nil || !took {
		t.Fatal(took, err)
	}
	if took, _ = s.TakeSnapshot(id, SnapOpen, "今天", nil); took {
		t.Fatal("took a snapshot of the text the latest holds")
	}
	snaps, err := s.Snapshots(id)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, sn := range snaps {
		text, err := s.SnapshotText(id, sn.Name)
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, text)
	}
	if !slices.Equal(texts, []string{"今天", "晚上"}) {
		t.Fatalf("snapshots kept: %q", texts)
	}
	if snaps[0].Reason != SnapOpen || snaps[0].Title != "一" || len(snaps[0].Marks) != 1 || snaps[0].Marks[0].Quote != "今" {
		t.Fatalf("record: %+v", snaps[0])
	}
}

// An entry taken from the prose quotes the chapter as it is; editing the
// chapter later does not stop the work from opening.
func TestSourceInAChapter(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "一", false)
	if _, err := s.WriteChapter(id, "她怕黑。", nil); err != nil {
		t.Fatal(err)
	}
	add := func(q string) error {
		_, err := s.Revise(Revision{By: ByAuthor, Ops: []Op{{Op: OpAdd, Place: PlaceBody, Category: Other, Text: q,
			Sources: []Source{{Chapter: id, Quote: q}}}}})
		return err
	}
	if err := add("她怕光"); err == nil {
		t.Fatal("a quote not in the chapter was taken")
	}
	if err := add("怕黑"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteChapter(id, "全改了。", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(s.Root()); err != nil {
		t.Fatal(err)
	}
}

// Writing over a version writes only when the disk still has it.
func TestWriteChapterOverTheVersionLastSeen(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "第一章", false)
	seen, err := s.WriteChapter(id, "原来的。", nil)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.Root(), "chapters", "第一章.txt")
	if err = os.WriteFile(p, []byte("别处改的，很重要。"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = s.WriteChapterOver(id, "新字原来的。", nil, seen); err != ErrChangedOutside {
		t.Fatalf("wrote over a change made outside: %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "别处改的，很重要。" {
		t.Fatalf("the disk has %q", b)
	}
	// Having read what is there, the caller may write over it.
	ch, err := s.ReadChapter(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.WriteChapterOver(id, "新字原来的。", nil, ch.Stamp); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "新字原来的。" {
		t.Fatalf("the disk has %q", b)
	}
}
