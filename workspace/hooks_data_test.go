package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForeshadowingDataLivesOnlyUnderHooksAndRetainsMixedMarkOrder(t *testing.T) {
	s := testStore(t)
	id := mustChapter(t, s, "", "chapter", false)
	_, err := s.WriteChapter(id, "abcdef", []ChapterMark{{Kind: MarkAI, Start: 0, End: 1}, {Kind: MarkForeshadow, Start: 1, End: 2}, {Kind: MarkAI, Start: 2, End: 3}, {Kind: MarkPayoff, Start: 3, End: 4}})
	if err != nil {
		t.Fatal(err)
	}
	hooks, err := os.ReadFile(filepath.Join(s.Root(), "ext", "hooks", id+".json"))
	if err != nil || !strings.Contains(string(hooks), "foreshadow") || strings.Contains(string(hooks), `"kind": "ai"`) {
		t.Fatal(string(hooks), err)
	}
	core, err := os.ReadFile(filepath.Join(s.Root(), "chapters", "chapter.marks.json"))
	if err != nil || strings.Contains(string(core), "foreshadow") || strings.Contains(string(core), "payoff") {
		t.Fatal(string(core), err)
	}
	ch, err := s.ReadChapter(id)
	if err != nil || len(ch.Marks) != 4 || ch.Marks[0].Kind != MarkAI || ch.Marks[1].Kind != MarkForeshadow || ch.Marks[3].Kind != MarkPayoff {
		t.Fatal(ch, err)
	}
	if err = s.TrashChapter(id); err != nil {
		t.Fatal(err)
	}
	if err = s.RestoreChapter(id); err != nil {
		t.Fatal(err)
	}
	ch, err = s.ReadChapter(id)
	if err != nil || len(ch.Marks) != 4 {
		t.Fatal("trash restore", ch, err)
	}
}
