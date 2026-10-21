package desk

import (
	"testing"
	"time"

	"writer/workspace"
)

func TestAFolderCountsItsChapters(t *testing.T) {
	w := &writeView{counts: map[string]int{"a": 3, "b": 4, "c": 5}}
	vol := &workspace.Node{ID: "v", Folder: true, Children: []*workspace.Node{
		{ID: "a"},
		{ID: "w", Folder: true, Children: []*workspace.Node{{ID: "b"}}},
	}}
	if k := w.wordsIn(vol); k != 7 {
		t.Fatalf("folder counts %d, want 7", k)
	}
	if k := w.wordsIn(&workspace.Node{ID: "c"}); k != 5 {
		t.Fatalf("chapter counts %d, want 5", k)
	}
}

// The day's goal is set from the counts in the bar and kept with the work;
// what is not a count leaves it be, and is said in red under the field.
func TestSetTheDaysGoal(t *testing.T) {
	a, tt, _ := writingFixture(t, "一二三")
	if err := tt.Click("字数"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.write.goalOpen || !tt.HasText("每日目标") {
		t.Fatal("no goal to set", tt.Texts())
	}
	// The days above it are said as every day is: 今天, 昨天, 10-14.
	now := time.Now()
	for _, day := range []string{"今天 写了 0 字", "昨天 写了 0 字", dayOf(now.AddDate(0, 0, -6), now) + " 写了 0 字"} {
		if _, ok := tt.Find(day); !ok {
			t.Fatal("the last week does not say", day, tt.Texts())
		}
	}
	a.setGoal("2000")
	if a.write.counter.Goal != 2000 || savedPrefs(t, a).Goal != 2000 {
		t.Fatal(a.write.counter.Goal)
	}
	for _, typed := range []string{"两千", "0"} {
		a.write.goalText = typed
		a.setGoal(typed)
		tt.Frame()
		if a.write.counter.Goal != 2000 || !tt.HasText("每日目标须为正整数，留空则不设。") {
			t.Fatal(typed, "changed the goal, or was not said to be wrong:", a.write.counter.Goal)
		}
	}
	a.write.goalText = "2000"
	tt.Frame()
	if tt.HasText("每日目标须为正整数，留空则不设。") || !tt.HasText("字数旁显示今日字数和目标。") || tt.HasText("设置后，字数旁") {
		t.Fatal("a goal set right is said to be wrong", tt.Texts())
	}
	a.write.focus = true // room for today's count in the bar
	tt.Frame()
	tt.Frame()
	if !tt.HasText("/ 2,000") {
		t.Fatal("the goal is not by the counts", tt.Texts())
	}
	a.setGoal("")
	if a.write.counter.Goal != 0 || savedPrefs(t, a).Goal != 0 {
		t.Fatal("the goal stayed", a.write.counter.Goal)
	}
}

// With text selected, the header counts the selection in place of the
// chapter.
func TestCountTheSelection(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚推门。 雨停了。")
	a.write.focus = true
	tt.Frame()
	if !tt.HasText("本章 9 字 · 全书 9 字") {
		t.Fatal("before", tt.Texts())
	}
	a.write.ed.MoveTo(0, false)
	a.write.ed.MoveTo(5, true)
	tt.Frame()
	if !tt.HasText("选中 5 字 · 全书 9 字") {
		t.Fatal("selected", tt.Texts())
	}
}

// The counts move as the author types, before the text is saved.
func TestCountsMoveAsTyped(t *testing.T) {
	a, tt, _ := writingFixture(t, "一")
	saveAfter = time.Hour
	a.write.focus = true
	a.write.ed.Focus()
	a.write.ed.MoveTo(1, false)
	tt.Frame()
	tt.Type("二三")
	tt.Frame()
	if !tt.HasText("本章 3 字 · 全书 3 字 · 今天 +2") {
		t.Fatal(tt.Texts())
	}
}
