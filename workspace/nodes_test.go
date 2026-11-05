package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func mustChapter(t *testing.T, s *Store, parent, title string, folder bool) string {
	t.Helper()
	id, err := s.NewChapter(parent, title, folder)
	if err != nil {
		t.Fatal(err)
	}
	return id
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
	for _, f := range []string{"第二章 铜钥匙.txt"} {
		if !exists(t, ch, "第一卷", f) {
			t.Fatalf("%s did not follow the rename", f)
		}
	}
	if !exists(t, s.Root(), "ext", "hooks", two+".json") {
		t.Fatal("hooks lost after rename")
	}
	if !exists(t, s.Root(), "ext", "outline", two+".txt") {
		t.Fatal("outline lost after rename")
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

// A summary an older version wrote beside a chapter stays where it is:
// renaming the chapter, trashing it and restoring it neither move nor
// change it, and the work opens with it as before.
func TestAnOldSummaryStaysWhereItWas(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "第一章", false)
	if _, err := s.WriteChapter(id, "正文。", nil); err != nil {
		t.Fatal(err)
	}
	const sum = `{"text":"林晚回到旧城。"}`
	old := filepath.Join(s.Root(), "chapters", "第一章.summary.json")
	if err := os.WriteFile(old, []byte(sum), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameChapter(id, "第二章"); err != nil {
		t.Fatal(err)
	}
	if err := s.TrashChapter(id); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreChapter(id); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(old); err != nil || string(b) != sum {
		t.Fatalf("the old summary is %q, %v", b, err)
	}
	if exists(t, s.Root(), "chapters", "第二章.summary.json") {
		t.Fatal("the old summary moved with the chapter")
	}
	s.Close()
	again, err := Open(s.Root())
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	tree, err := again.Chapters()
	if err != nil || len(tree.Root.Children) != 1 || tree.Root.Children[0].Title != "第二章" {
		t.Fatalf("the tree is %+v, %v", tree.Root, err)
	}
	if ch, err := again.ReadChapter(id); err != nil || ch.Text != "正文。" {
		t.Fatalf("read %+v, %v", ch, err)
	}
}
