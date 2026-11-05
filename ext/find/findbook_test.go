package find

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"writer/internal/testshot"
)

// Finding in the whole book lists each chapter with the query and opens
// the one chosen with its first match selected.
func TestFindInTheWholeBook(t *testing.T) {
	a, tt, first := writingFixture(t, "林晚走进雨里。")
	second, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(second, "门开了，林晚又看见了林晚的影子。", nil); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	tt.Frame()
	if err = tt.Click("查找替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("林晚")
	tt.Frame()
	if err = tt.Click("全书"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("全书 2 章、共 3 处") || !tt.HasText("门开了，林晚又看见了林晚的影子。") {
		t.Fatal(tt.Texts())
	}
	testshot.Save(t, "find-each-hit.png", tt.Image())
	if !tt.HasText("第二章 · 2 处") {
		t.Fatal(tt.Texts())
	}
	if _, ok := tt.Find("全书结果-第二章-1"); !ok {
		t.Fatal("missing first hit")
	}
	if err = tt.Click("全书结果-第二章-2"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if shownChapter(a) != second || !findOf(a).Finding || findOf(a).Wide || findOf(a).Current != 1 {
		t.Fatal("the chapter chosen did not open with the search", shownChapter(a), first)
	}
	if s, e := shownEditor(a).Selection(); s != 10 || e != 12 {
		t.Fatal("the chosen match is not selected", s, e)
	}
	if _, ok := tt.Find("全书结果-第二章-1"); ok {
		t.Fatal("book list remained")
	}
	if err = tt.Click("全书"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// What is typed into the chapter open counts before it is saved.
	shownEditor(a).Replace(0, 0, "林晚")
	tt.Frame()
	if !tt.HasText("全书 2 章、共 4 处") {
		t.Fatal(tt.Texts())
	}
}

// Replacing through the book asks first, changes every chapter with a
// snapshot before each, and the chapter open can be undone.
func TestReplaceThroughTheBook(t *testing.T) {
	a, tt, first := writingFixture(t, "林晚走进雨里。")
	second, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(second, "门开了，林晚又看见了林晚的影子。", nil); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	tt.Frame()
	if err = tt.Click("查找替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("林晚")
	findOf(a).With = "苏晴"
	tt.Frame()
	if err = tt.Click("全书"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("全书替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("将全书 2 章中的 3 处“林晚”替换为“苏晴”？仅修改正文，设定和章纲不变；各章替换前均保存历史版本。") {
		t.Fatal("no asking", tt.Texts())
	}
	if ch, _ := a.Store.ReadChapter(second); ch.Text != "门开了，林晚又看见了林晚的影子。" {
		t.Fatal("replaced before asked", ch.Text)
	}
	testshot.Save(t, "find-replace-dialog.png", tt.Image())
	if err = tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if findOf(a).Sure || shownEditor(a).Doc.String() != "林晚走进雨里。" {
		t.Fatal("cancel changed prose")
	}
	if ch, _ := a.Store.ReadChapter(second); ch.Text != "门开了，林晚又看见了林晚的影子。" {
		t.Fatal("cancel changed disk")
	}
	if err = tt.Click("全书替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err = tt.Click("确定替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if ch, _ := a.Store.ReadChapter(second); ch.Text != "门开了，苏晴又看见了苏晴的影子。" {
		t.Fatal(ch.Text)
	}
	if ch, _ := a.Store.ReadChapter(first); ch.Text != "苏晴走进雨里。" {
		t.Fatal(ch.Text)
	}
	// The chapter open had its text kept as it opened; the other gets one.
	if snaps, err := a.Store.Snapshots(second); err != nil || len(snaps) == 0 || snaps[0].Reason != "replace" {
		t.Fatal("no snapshot before replacing", snaps, err)
	}
	if !tt.HasText("全书未找到") {
		t.Fatal(tt.Texts())
	}
	shownEditor(a).Undo()
	if shownEditor(a).Doc.String() != "林晚走进雨里。" {
		t.Fatal("the chapter open cannot be undone", shownEditor(a).Doc.String())
	}
}

func TestBookFindLongListScrollAndFrameCost(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚走进雨里。")
	second, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(second, strings.Repeat("林晚走了。\n", 10000), nil); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	v := findOf(a)
	v.Finding = true
	v.Query = "林晚"
	for range 5 {
		tt.Frame()
	}
	measure := func() time.Duration {
		start := time.Now()
		for range 30 {
			tt.Frame()
		}
		return time.Since(start) / 30
	}
	base := measure()
	v.Wide = true
	for range 5 {
		tt.Frame()
	}
	if v.hitTotal != 10001 || len(v.hitRows) != 10003 {
		t.Fatal(v.hitTotal, len(v.hitRows))
	}
	first := &v.hitRows[0]
	cost := measure()
	if first != &v.hitRows[0] || len(tt.Texts()) > 200 {
		t.Fatal("steady frame rebuilt results or built hidden rows")
	}
	at, ok := tt.Find("全书结果-第二章-1")
	if !ok {
		t.Fatal(tt.Texts())
	}
	tt.Scroll(at.X+10, at.Y+10, 0, 1200)
	tt.Frame()
	tt.Frame()
	if start, _ := v.hitList.Visible(); start == 0 {
		t.Fatal("results did not scroll")
	}
	testshot.Save(t, "find-long-scrolled.png", tt.Image())
	start, _ := v.hitList.Visible()
	index := start + 2
	if index >= len(v.hitRows) {
		t.Fatal(index)
	}
	row := v.hitRows[index]
	label := fmt.Sprintf("全书结果-第二章-%d", row.Match+1)
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if shownChapter(a) != second || v.Wide || v.Current != row.Match {
		t.Fatal("scrolled result did not open", v.Current, row.Match)
	}
	want := v.Hits[row.Chapter].Matches[row.Match]
	if s, e := shownEditor(a).Selection(); s != want.Start || e != want.End {
		t.Fatal(s, e, want)
	}
	t.Logf("steady frame: closed=%s; 10,001 hits=%s", base, cost)
	if cost > max(base*3, 20*time.Millisecond) {
		t.Fatal("long results slowed steady frames", base, cost)
	}
}
