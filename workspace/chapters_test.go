package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func mustChapter(t *testing.T, s *Store, parent, title string, folder bool) string {
	t.Helper()
	id, err := s.NewChapter(parent, title, folder)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// shape draws the tree as titles indented by depth.
func shape(t *testing.T, s *Store) string {
	t.Helper()
	tree, err := s.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	tree.Walk(func(n *Node, depth int) {
		b.WriteString(strings.Repeat("  ", depth) + n.Title)
		if n.Folder {
			b.WriteString("/")
		}
		b.WriteString("\n")
	})
	return b.String()
}

func exists(t *testing.T, parts ...string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

// The disk mirrors the tree: what the author makes, renames, moves, trashes
// and restores is a folder or a .txt where the tree shows it.
func TestChapterTreeOnDisk(t *testing.T) {
	s := testStore(t)
	ch := filepath.Join(s.Root(), "chapters")
	vol := mustChapter(t, s, "", "第一卷", true)
	one := mustChapter(t, s, vol, "第一章 雨夜", false)
	two := mustChapter(t, s, vol, "第二章：钥匙?", false)
	mustChapter(t, s, "", "后记", false)
	if got := shape(t, s); got != "第一卷/\n  第一章 雨夜\n  第二章：钥匙?\n后记\n" {
		t.Fatalf("tree:\n%s", got)
	}
	if !exists(t, ch, "第一卷", "第一章 雨夜.txt") || !exists(t, ch, "第一卷", "第二章：钥匙？.txt") {
		t.Fatal("files do not mirror the tree")
	}
	if _, err := s.WriteChapter(two, "她握着钥匙。", []ChapterMark{{Kind: MarkForeshadow, Start: 3, End: 5}}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteOutline(two, "她发现钥匙"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameChapter(two, "第二章 铜钥匙"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"第二章 铜钥匙.txt", "第二章 铜钥匙.outline.txt", "第二章 铜钥匙.marks.json"} {
		if !exists(t, ch, "第一卷", f) {
			t.Fatalf("%s did not follow the rename", f)
		}
	}
	if err := s.MoveChapter(two, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveChapter(vol, vol, 0); err == nil {
		t.Fatal("a folder moved into itself")
	}
	if got := shape(t, s); got != "第二章 铜钥匙\n第一卷/\n  第一章 雨夜\n后记\n" {
		t.Fatalf("after moving:\n%s", got)
	}
	c, err := s.ReadChapter(two)
	if err != nil {
		t.Fatal(err)
	}
	if c.Text != "她握着钥匙。" || c.Outline != "她发现钥匙" || len(c.Marks) != 1 || c.Marks[0].Quote != "钥匙" {
		t.Fatalf("chapter after moving: %+v", c)
	}
	if err = s.TrashChapter(vol); err != nil {
		t.Fatal(err)
	}
	if got := shape(t, s); got != "第二章 铜钥匙\n后记\n" || exists(t, ch, "第一卷") {
		t.Fatalf("after trashing:\n%s", got)
	}
	trash, err := s.Trash()
	if err != nil || len(trash) != 1 || trash[0].Title != "第一卷" {
		t.Fatalf("trash %+v %v", trash, err)
	}
	if err = s.RestoreChapter(vol); err != nil {
		t.Fatal(err)
	}
	if got := shape(t, s); got != "第二章 铜钥匙\n后记\n第一卷/\n  第一章 雨夜\n" {
		t.Fatalf("after restoring:\n%s", got)
	}
	tree, _ := s.Chapters()
	if n, ok := tree.Node(one); !ok || n.Parent.ID != vol {
		t.Fatal("what the folder held lost its ID")
	}
	if trash, _ = s.Trash(); len(trash) != 0 {
		t.Fatalf("trash after restoring: %+v", trash)
	}
}

// Titles that clash on disk, letter case aside, or that file systems
// refuse, still save and read back as the author typed them.
func TestChapterTitlesFileSystemsRefuse(t *testing.T) {
	s := testStore(t)
	a := mustChapter(t, s, "", "Chapter", false)
	b := mustChapter(t, s, "", "chapter", false)
	c := mustChapter(t, s, "", "CON", false)
	d := mustChapter(t, s, "", ".order", false)
	e := mustChapter(t, s, "", "a/b\\c*?", false)
	f := mustChapter(t, s, "", "注.outline", false)
	ch := filepath.Join(s.Root(), "chapters")
	for _, name := range []string{"Chapter.txt", "chapter (2).txt", "CON_.txt", "．order.txt", "a／b＼c＊？.txt", "注.outline_.txt"} {
		if !exists(t, ch, name) {
			t.Errorf("no %s", name)
		}
	}
	tree, err := s.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{a: "Chapter", b: "chapter", c: "CON", d: ".order", e: "a/b\\c*?", f: "注.outline"} {
		if n, ok := tree.Node(id); !ok || n.Title != want {
			t.Errorf("node %s: %+v", want, n)
		}
	}
	if got := shape(t, s); strings.Count(got, "\n") != 6 {
		t.Fatalf("an outline-like name was taken for a companion file:\n%s", got)
	}
}

// Files the author adds or renames outside become nodes of their own, and
// entries whose files are gone drop out.
func TestChaptersChangedOutside(t *testing.T) {
	s := testStore(t)
	ch := filepath.Join(s.Root(), "chapters")
	one := mustChapter(t, s, "", "一", false)
	mustChapter(t, s, "", "二", false)
	if err := os.WriteFile(filepath.Join(ch, "三.txt"), []byte("　　外面写的。\r\n第二段"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(ch, "二.txt"), filepath.Join(ch, "贰.txt")); err != nil {
		t.Fatal(err)
	}
	if got := shape(t, s); got != "一\n三\n贰\n" {
		t.Fatalf("tree:\n%s", got)
	}
	tree, _ := s.Chapters()
	var three *Node
	tree.Walk(func(n *Node, _ int) {
		if n.Title == "三" {
			three = n
		}
	})
	c, err := s.ReadChapter(three.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Text != "外面写的。\n第二段" {
		t.Fatalf("indent and CRLF kept: %q", c.Text)
	}
	again, _ := s.Chapters()
	if n, ok := again.Node(three.ID); !ok || n.Title != "三" {
		t.Fatal("a file added outside got a new ID each read")
	}
	if _, ok := again.Node(one); !ok {
		t.Fatal("the first node lost its ID")
	}
}

// Marks are found again by what they quote after the text changed outside;
// one whose words are gone is orphaned.
func TestMarksFollowTheirQuote(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "一", false)
	text := "井边有一把钥匙。灯灭了。"
	if _, err := s.WriteChapter(id, text, []ChapterMark{
		{Kind: MarkForeshadow, Start: 5, End: 7, Note: "钥匙"},
		{Kind: MarkAIAccepted, Start: 8, End: 11},
	}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.Root(), "chapters", "一.txt")
	if err := os.WriteFile(p, []byte("雨夜。井边有一把钥匙。"), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := s.ReadChapter(id)
	if err != nil {
		t.Fatal(err)
	}
	m := c.Marks
	if len(m) != 2 || m[0].Start != 8 || m[0].End != 10 || m[0].Orphaned || m[0].Note != "钥匙" {
		t.Fatalf("foreshadowing: %+v", m)
	}
	if !m[1].Orphaned || m[1].Start != m[1].End {
		t.Fatalf("a mark whose words are gone: %+v", m[1])
	}
	// Saved again, the orphaned one still tells what it was.
	if _, err = s.WriteChapter(id, c.Text, m); err != nil {
		t.Fatal(err)
	}
	if c, err = s.ReadChapter(id); err != nil || c.Marks[1].Quote != "灯灭了" {
		t.Fatalf("an orphaned mark saved again: %+v, %v", c.Marks, err)
	}
	m = c.Marks
	mustChapter(t, s, "", "二", false) // no marks
	all, err := s.Marks()
	if err != nil || len(all) != 1 || !slices.Equal(all[id], m) {
		t.Fatalf("all the marks: %+v, %v", all, err)
	}
}

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

// Purging a trashed folder deletes it and the snapshots of what it held.
func TestPurgeDeletesHistory(t *testing.T) {
	s := testStore(t)
	vol := mustChapter(t, s, "", "卷", true)
	one := mustChapter(t, s, vol, "一", false)
	if _, err := s.TakeSnapshot(one, SnapOpen, "字", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.TrashChapter(vol); err != nil {
		t.Fatal(err)
	}
	if err := s.PurgeChapter(vol); err != nil {
		t.Fatal(err)
	}
	if exists(t, s.Root(), "history", one) || exists(t, s.Root(), "trash", vol) {
		t.Fatal("purge left files behind")
	}
}

func TestExportAndCount(t *testing.T) {
	s := testStore(t)
	vol := mustChapter(t, s, "", "第一卷", true)
	one := mustChapter(t, s, vol, "第一章", false)
	if _, err := s.WriteChapter(one, "雨下着。\n她 走了。", nil); err != nil {
		t.Fatal(err)
	}
	two := mustChapter(t, s, "", "第二章", false)
	if _, err := s.WriteChapter(two, "天晴了。", nil); err != nil {
		t.Fatal(err)
	}
	out, err := s.Export("", true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "测试作品\n\n第一卷\n\n第一章\n\n　　雨下着。\n　　她 走了。\n\n第二章\n\n　　天晴了。\n"; out != want {
		t.Fatalf("export:\n%q\nwant\n%q", out, want)
	}
	// A volume alone, and a chapter alone, without the work's name.
	if out, err = s.Export(vol, false); err != nil || out != "第一卷\n\n第一章\n\n雨下着。\n她 走了。\n" {
		t.Fatalf("volume %q %v", out, err)
	}
	if out, err = s.Export(two, false); err != nil || out != "第二章\n\n天晴了。\n" {
		t.Fatalf("chapter %q %v", out, err)
	}
	counts, err := s.Counts()
	if err != nil || counts[one] != 8 {
		t.Fatalf("counts %v %v", counts, err)
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
