package desk

import (
	"slices"
	"testing"
)

func TestPagesEnterAndLeaveOnce(t *testing.T) {
	a, _, _ := writingFixture(t, "雨停了。")
	var calls []string
	a.pages[pageWrite].leave = func() { calls = append(calls, "leave 正文") }
	a.pages[pagePlan].enter = func() { calls = append(calls, "enter 大纲") }
	showPage(a, pagePlan)
	a.turnPage()
	a.turnPage()
	if want := []string{"leave 正文", "enter 大纲"}; !slices.Equal(calls, want) {
		t.Fatalf("page actions = %v, want %v", calls, want)
	}
}
