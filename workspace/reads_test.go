package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutlinesReadWithoutProse(t *testing.T) {
	s := testStore(t)
	v := mustChapter(t, s, "", "卷", true)
	id := mustChapter(t, s, v, "章", false)
	empty := mustChapter(t, s, "", "无纲", false)
	planOK(t, s.WriteOutline(v, "卷纲"))
	planOK(t, s.WriteOutline(id, "章纲\r\n第二行"))
	tree, err := s.Chapters()
	planOK(t, err)
	n, _ := tree.Node(id)
	marks, err := s.nodePath(n, marksExt)
	planOK(t, err)
	planOK(t, os.WriteFile(marks, []byte("{"), 0600))
	if _, err := s.ReadChapter(id); err == nil {
		t.Fatal("fixture should have unreadable marks")
	}
	out, err := s.Outlines()
	planOK(t, err)
	if len(out) != 3 || out[v] != "卷纲" || out[id] != "章纲\n第二行" || out[empty] != "" {
		t.Fatal(out)
	}
	p, err := s.outlinePath(n.ID)
	planOK(t, err)
	planOK(t, os.Remove(p))
	planOK(t, os.Mkdir(p, 0755))
	if _, err := s.Outlines(); err == nil || !strings.Contains(err.Error(), p) {
		t.Fatalf("outline error must carry its path: %v", err)
	}
}

func TestPiecesAndCount(t *testing.T) {
	s := testStore(t)
	vol := mustChapter(t, s, "", "第一卷", true)
	one := mustChapter(t, s, vol, "第一章", false)
	if _, err := s.WriteChapter(one, "雨下着。\n她 走了。", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteOutline(one, "  她走了\n"); err != nil {
		t.Fatal(err)
	}
	mustChapter(t, s, "", "番外", false)
	pieces, err := s.Pieces()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range pieces {
		got = append(got, fmt.Sprintf("%s/%d/%v", p.Title, p.Depth, p.Folder))
	}
	if want := "第一卷/0/true 第一章/1/false 番外/0/false"; strings.Join(got, " ") != want {
		t.Fatalf("pieces %q, want %q", got, want)
	}
	if p := pieces[1]; p.ID != one || p.Text != "雨下着。\n她 走了。" || p.Outline != "她走了" {
		t.Fatalf("chapter %+v", p)
	}
	counts, err := s.Counts()
	if err != nil || counts[one] != 8 {
		t.Fatalf("counts %v %v", counts, err)
	}
}

// Texts gives every document's text as reading each gives it.
func TestTextsAreWhatReadingEachGives(t *testing.T) {
	s := testStore(t)
	vol := mustChapter(t, s, "", "卷", true)
	one := mustChapter(t, s, vol, "一", false)
	two := mustChapter(t, s, "", "二", false)
	if err := os.WriteFile(filepath.Join(s.Root(), "chapters", "卷", "一.txt"), []byte("　　雨。\r\n　　灯。\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	texts, err := s.Texts()
	if err != nil || len(texts) != 2 {
		t.Fatalf("%v, %v", texts, err)
	}
	for _, id := range []string{one, two} {
		ch, err := s.ReadChapter(id)
		if err != nil || texts[id] != ch.Text {
			t.Fatalf("%s: %q, reading it gives %q, %v", id, texts[id], ch.Text, err)
		}
	}
	if texts[one] == "" {
		t.Fatal("no text")
	}
}
