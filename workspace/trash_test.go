package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// failRename makes moving the files with suffix fail, as a file another
// program holds open does on Windows.
func failRename(t *testing.T, suffix string) {
	t.Helper()
	was := rename
	rename = func(from, to string) error {
		if strings.HasSuffix(from, suffix) {
			return &os.LinkError{Op: "rename", Old: from, New: to, Err: os.ErrPermission}
		}
		return was(from, to)
	}
	t.Cleanup(func() { rename = was })
}

// A node that cannot be moved to the trash stays whole where it was: the
// trash lists nothing of it, and it keeps its ID and its snapshots.
func TestTrashThatCannotMoveLeavesTheNodeWhole(t *testing.T) {
	for _, held := range []string{"第一章.txt", "第一章.marks.json"} {
		t.Run(held, func(t *testing.T) { trashHeld(t, held) })
	}
}

func trashHeld(t *testing.T, held string) {
	{
		s := testStore(t)
		id := mustChapter(t, s, "", "第一章", false)
		if _, err := s.WriteChapter(id, "正文。", []ChapterMark{{Kind: MarkForeshadow, Start: 0, End: 1}}); err != nil {
			t.Fatal(err)
		}
		if err := s.WriteOutline(id, "章纲"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.TakeSnapshot(id, SnapOpen, "正文。", nil); err != nil {
			t.Fatal(err)
		}
		failRename(t, held)
		if err := s.TrashChapter(id); err == nil {
			t.Fatalf("%s held: trashed", held)
		}
		if items, err := s.Trash(); err != nil || len(items) != 0 {
			t.Fatalf("%s held: the trash lists %+v, %v", held, items, err)
		}
		if exists(t, s.Root(), "trash", id) {
			t.Fatalf("%s held: the trash kept a folder for it", held)
		}
		tree, err := s.Chapters()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := tree.Node(id); !ok {
			t.Fatalf("%s held: the chapter lost its ID", held)
		}
		ch, err := s.ReadChapter(id)
		if err != nil || ch.Text != "正文。" || ch.Outline != "章纲" {
			t.Fatalf("%s held: read %+v, %v", held, ch, err)
		}
		if snaps, err := s.Snapshots(id); err != nil || len(snaps) != 1 {
			t.Fatalf("%s held: %d snapshots, %v", held, len(snaps), err)
		}
	}
}

// A note in the trash of a node that never got there is not an item of
// the trash: the node keeps its ID, and its snapshots cannot be purged.
func TestTrashNoteWithoutItsFilesIsNotListed(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "第一章", false)
	if _, err := s.TakeSnapshot(id, SnapOpen, "正文。", nil); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Root(), "trash", id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	note := `{"id":"` + id + `","title":"第一章","file":"第一章","at":"2026-10-14T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, ".trashed.json"), []byte(note), 0644); err != nil {
		t.Fatal(err)
	}
	if items, err := s.Trash(); err != nil || len(items) != 0 {
		t.Fatalf("the trash lists %+v, %v", items, err)
	}
	tree, err := s.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tree.Node(id); !ok {
		t.Fatal("the chapter lost its ID")
	}
	if err = s.PurgeChapter(id); err == nil {
		t.Fatal("purged a chapter that is not in the trash")
	}
	if snaps, _ := s.Snapshots(id); len(snaps) != 1 {
		t.Fatalf("%d snapshots", len(snaps))
	}
	// Trashed in earnest, it is there.
	if err = s.TrashChapter(id); err != nil {
		t.Fatal(err)
	}
	if items, _ := s.Trash(); len(items) != 1 || items[0].ID != id {
		t.Fatalf("the trash lists %+v", items)
	}
}

// A node that cannot be moved out of the trash stays whole in it.
func TestRestoreThatCannotMoveLeavesTheNodeInTheTrash(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "第一章", false)
	if _, err := s.WriteChapter(id, "正文。", []ChapterMark{{Kind: MarkForeshadow, Start: 0, End: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteOutline(id, "章纲"); err != nil {
		t.Fatal(err)
	}
	if err := s.TrashChapter(id); err != nil {
		t.Fatal(err)
	}
	was := rename
	failRename(t, "第一章.marks.json")
	if err := s.RestoreChapter(id); err == nil {
		t.Fatal("restored")
	}
	if items, _ := s.Trash(); len(items) != 1 || items[0].ID != id {
		t.Fatalf("the trash lists %+v", items)
	}
	if got := shape(t, s); got != "" {
		t.Fatalf("the tree has\n%s", got)
	}
	if !exists(t, s.Root(), "trash", id, "第一章.txt") || !exists(t, s.Root(), "ext", "outline", id+".txt") {
		t.Fatal("the trash lost a file of it")
	}
	rename = was
	if err := s.RestoreChapter(id); err != nil {
		t.Fatal(err)
	}
	ch, err := s.ReadChapter(id)
	if err != nil || ch.Text != "正文。" || ch.Outline != "章纲" {
		t.Fatalf("read %+v, %v", ch, err)
	}
}
