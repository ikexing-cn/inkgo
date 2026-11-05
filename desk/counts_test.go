package desk

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/goal"
	"writer/internal/testshot"
	"writer/workspace"
)

func TestAFolderCountsItsChapters(t *testing.T) {
	counts := map[string]int{"a": 3, "b": 4, "c": 5}
	vol := &workspace.Node{ID: "v", Folder: true, Children: []*workspace.Node{
		{ID: "a"},
		{ID: "w", Folder: true, Children: []*workspace.Node{{ID: "b"}}},
	}}
	words := nodeWords(&workspace.Node{Children: []*workspace.Node{vol, {ID: "c"}}}, counts)
	if k := words["v"]; k != 7 {
		t.Fatalf("folder counts %d, want 7", k)
	}
	if k := words["c"]; k != 5 {
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
	if !goalOf(a).Open || !tt.HasText("每日目标") {
		t.Fatal("no goal to set", tt.Texts())
	}
	// The days above it are said as every day is: 今天, 昨天, 10-14.
	now := time.Now()
	for _, day := range []string{"今天 写了 0 字", "昨天 写了 0 字", kit.DayOf(now.AddDate(0, 0, -6), now) + " 写了 0 字"} {
		if _, ok := tt.Find(day); !ok {
			t.Fatal("the last week does not say", day, tt.Texts())
		}
	}
	goalOf(a).SetGoal("2000")
	if goalOf(a).Counter.Goal != 2000 || savedGoal(t, a).Goal != 2000 {
		t.Fatal(goalOf(a).Counter.Goal)
	}
	for _, typed := range []string{"两千", "0"} {
		goalOf(a).Text = typed
		goalOf(a).SetGoal(typed)
		tt.Frame()
		if goalOf(a).Counter.Goal != 2000 || !tt.HasText("每日目标须为正整数，留空则不设。") {
			t.Fatal(typed, "changed the goal, or was not said to be wrong:", goalOf(a).Counter.Goal)
		}
	}
	goalOf(a).Text = "2000"
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
	goalOf(a).SetGoal("")
	if goalOf(a).Counter.Goal != 0 || savedGoal(t, a).Goal != 0 {
		t.Fatal("the goal stayed", goalOf(a).Counter.Goal)
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
	shownEditor(a).MoveTo(0, false)
	shownEditor(a).MoveTo(5, true)
	tt.Frame()
	if !tt.HasText("选中 5 字 · 全书 9 字") {
		t.Fatal("selected", tt.Texts())
	}
}

// The counts move as the author types, before the text is saved.
func TestCountsMoveAsTyped(t *testing.T) {
	a, tt, _ := writingFixture(t, "一")
	a.book.Timing.SaveAfter = time.Hour
	a.write.focus = true
	shownEditor(a).Focus()
	shownEditor(a).MoveTo(1, false)
	tt.Frame()
	tt.Type("二三")
	tt.Frame()
	if !slices.Contains(tt.Texts(), "本章 3 字 · 全书 3 字") {
		t.Fatal(tt.Texts())
	}
	testshot.Save(t, "count-no-goal.png", tt.Image())
	goalOf(a).SetGoal("2000")
	tt.Frame()
	testshot.Save(t, "count-with-goal.png", tt.Image())
	if !tt.HasText("本章 3 字 · 全书 3 字 · 今天 2 / 2,000") {
		t.Fatal(tt.Texts())
	}
}

func TestCountsWaitForTheBook(t *testing.T) {
	a, _, _ := writingFixture(t, "一二三")
	goalOf(a).Counter = goal.Counter{Day: "2026-10-09", DayStart: 3, Days: map[string]int{"2026-10-08": 2}}
	setChapterCounts(a, nil)
	before := goalOf(a).Counter
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.Local)
	if goalOf(a).RollDay(now) || goalOf(a).Today(now) != 0 || goalOf(a).Today(now.AddDate(0, 0, -1)) != 0 {
		t.Fatal("unread book started a day")
	}
	if got := goalOf(a).LastDays(now, 3); !slices.Equal(got, []int{2, 0, 0}) {
		t.Fatal(got)
	}
	if got := goalOf(a).LastDays(now.AddDate(0, 0, -1), 2); !slices.Equal(got, []int{2, 0}) {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(goalOf(a).Counter, before) {
		t.Fatal("unread book changed the counter", goalOf(a).Counter)
	}
	setChapterCounts(a, map[string]int{"chapter": 10})
	if !goalOf(a).RollDay(now) || goalOf(a).Counter.Days["2026-10-09"] != 7 {
		t.Fatal("known count did not start a day", goalOf(a).Counter)
	}
	if goalOf(a).Today(now) != 0 {
		t.Fatal("new day did not start at zero")
	}
}

func TestShotCountWithGoalOffIsPlainText(t *testing.T) {
	a, tt, _ := writingFixture(t, "一二三")
	a.setExtension("goal", false, false)
	a.write.focus = true
	tt.Move(0, 0)
	tt.Frame()
	tt.Frame()
	r := find(t, tt, "字数")
	before := tt.Image()
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	tt.Frame()
	if slices.Contains(tt.Texts(), "全书 3 字") {
		t.Fatal("plain count has tooltip")
	}
	if pixelsChanged(before, tt.Image(), r) {
		t.Fatal("plain count responds to hover")
	}
	tt.Click("字数")
	tt.Frame()
	if tt.HasText("每日目标") {
		t.Fatal("disabled goal opened")
	}
	for range 80 {
		tt.Key(0, ui.KeyTab)
		if tt.Focused("字数") {
			t.Fatal("plain count is focusable")
		}
	}
	testshot.Save(t, "count-goal-off.png", tt.Image())
}
