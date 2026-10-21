package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Marks are found again by what they quote after the text changed outside;
// one whose words are gone is orphaned.
func TestMarksFollowTheirQuote(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "一", false)
	text := "井边有一把钥匙。灯灭了。"
	if _, err := s.WriteChapter(id, text, []ChapterMark{
		{Kind: MarkForeshadow, Start: 5, End: 7, Note: "钥匙"},
		{Kind: markAIDraft, Start: 8, End: 11},
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
	if !m[1].Orphaned || m[1].Start != m[1].End || m[1].Kind != MarkAI {
		t.Fatalf("a mark whose words are gone, of what an AI wrote as it was marked before: %+v", m[1])
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
