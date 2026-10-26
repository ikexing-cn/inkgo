package desk

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/internal/testshot"
)

// openOutline opens the page of a folder from its menu in the tree.
func openOutline(tt *ui.Tester, title string) error {
	if err := tt.RightClick("章节-" + title); err != nil {
		return err
	}
	tt.Frame()
	if err := tt.Click("菜单-编辑卷纲"); err != nil {
		return err
	}
	tt.Frame()
	tt.Frame()
	return nil
}

// tabsFixture is a work of three chapters and a folder, the first chapter
// open. Words typed stay unsaved until a test saves them.
func tabsFixture(t *testing.T) (a *App, tt *ui.Tester, ids map[string]string) {
	t.Helper()
	a, tt, one := writingFixture(t, "整条街的灯是同时灭的。")
	book.SaveAfter = time.Hour // quick put it back as the test ends
	ids = map[string]string{"第一章": one}
	for _, title := range []string{"第二章", "第三章"} {
		id, err := a.Store.NewChapter("", title, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = a.Store.WriteChapter(id, title+"的正文。", nil); err != nil {
			t.Fatal(err)
		}
		ids[title] = id
	}
	vol, err := a.Store.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	ids["第一卷"] = vol
	markUnchecked(a)
	tt.Frame()
	tt.Frame()
	return a, tt, ids
}

// tabs is the row of tabs as the page shows it: their names, the one shown
// starred.
func tabs(a *App, tt *ui.Tester) string {
	var out []string
	for _, s := range tt.Texts() {
		name, ok := strings.CutPrefix(s, "标签页-")
		if !ok {
			continue
		}
		if title, _, _ := a.tabTitle(shownChapter(a)); title == name {
			name += "*"
		}
		out = append(out, name)
	}
	return strings.Join(out, " ")
}

// A chapter clicked in the tree opens in a tab of its own beside the one
// shown, or its tab is shown when it has one; a folder clicked only folds,
// and has a tab once its outline is opened from its menu.
func TestChaptersOpenInTabs(t *testing.T) {
	a, tt, ids := tabsFixture(t)
	if got := tabs(a, tt); got != "第一章*" {
		t.Fatalf("the work opens with %q", got)
	}
	r, _ := tt.Find("标签页-第一章")
	top, _ := tt.Find("查找替换")
	if r.Y < top.Y+top.H || r.H != kit.HBar {
		t.Fatalf("the tabs are not a row under the title bar: %+v under %+v", r, top)
	}
	click(t, tt, "章节-第一卷")
	if got := tabs(a, tt); got != "第一章*" || !a.write.folded[ids["第一卷"]] {
		t.Fatalf("a folder clicked: tabs %q, folded %v", got, a.write.folded)
	}
	click(t, tt, "章节-第一卷", "章节-第三章", "章节-第二章")
	if got := tabs(a, tt); got != "第一章 第三章 第二章*" {
		t.Fatalf("two more opened: %q", got)
	}
	click(t, tt, "标签页-第一章")
	if got := tabs(a, tt); got != "第一章* 第三章 第二章" || shownChapter(a) != ids["第一章"] {
		t.Fatalf("a tab clicked: %q", got)
	}
	// A new one stands beside the one shown, not at the end.
	click(t, tt, "关闭标签页-第三章")
	if got := tabs(a, tt); got != "第一章* 第二章" {
		t.Fatalf("a tab not shown closed: %q", got)
	}
	click(t, tt, "章节-第三章")
	if got := tabs(a, tt); got != "第一章 第三章* 第二章" {
		t.Fatalf("a new tab goes beside the one shown: %q", got)
	}
	if err := openOutline(tt, "第一卷"); err != nil {
		t.Fatal(err)
	}
	if got := tabs(a, tt); got != "第一章 第三章 第一卷* 第二章" || !tt.HasText("卷纲") {
		t.Fatalf("the outline of a folder: %q", got)
	}
	// Kept with the work: the tabs are there the next time it opens.
	a.loadPrefs()
	if !slices.Equal(openTabs(a), []string{ids["第一章"], ids["第三章"], ids["第一卷"], ids["第二章"]}) || shownChapter(a) != ids["第一卷"] {
		t.Fatalf("kept %v, shown %q", openTabs(a), shownChapter(a))
	}
	// The toolbar does not say whether the chapter is saved: its tab does.
	if tt.HasText("已保存") || tt.HasText("未保存") {
		t.Fatal("the toolbar still says whether the chapter is saved")
	}
}

// Closing the tab shown shows the one at its right, or else at its left;
// the keys and the tab's menu close too. With none left the page is blank,
// stays so, and going back opens the chapter just closed.
func TestClosingTabs(t *testing.T) {
	a, tt, ids := tabsFixture(t)
	click(t, tt, "章节-第二章", "章节-第三章", "标签页-第二章")
	if got := tabs(a, tt); got != "第一章 第二章* 第三章" {
		t.Fatal(got)
	}
	click(t, tt, "关闭标签页-第二章")
	if got := tabs(a, tt); got != "第一章 第三章*" {
		t.Fatalf("the one shown closed: %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyW)
	tt.Frame()
	tt.Frame()
	if got := tabs(a, tt); got != "第一章*" {
		t.Fatalf("Ctrl+W, the last of the row: %q", got)
	}
	click(t, tt, "章节-第二章", "章节-第三章")
	shownEditor(a).Focus() // the keys work with the caret in the prose
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeyTab)
	tt.Frame()
	tt.Frame()
	if got := tabs(a, tt); got != "第一章* 第二章 第三章" {
		t.Fatalf("Ctrl+Tab from the last goes round to the first: %q", got)
	}
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyTab)
	tt.Frame()
	tt.Frame()
	if got := tabs(a, tt); got != "第一章 第二章 第三章*" {
		t.Fatalf("Ctrl+Shift+Tab goes back: %q", got)
	}
	shownEditor(a).Focus()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyW)
	tt.Frame()
	tt.Frame()
	if got := tabs(a, tt); got != "第一章 第二章*" {
		t.Fatalf("Ctrl+W with the caret in the prose: %q", got)
	}
	click(t, tt, "章节-第三章")
	if err := tt.RightClick("标签页-第二章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := rows(tt); !slices.Equal(got, []string{"关闭", "关闭其他", "关闭右侧"}) {
		t.Fatalf("the menu of a tab: %q", got)
	}
	click(t, tt, "菜单-关闭右侧")
	if got := tabs(a, tt); got != "第一章 第二章*" {
		t.Fatalf("closing those at the right, the one shown among them: %q", got)
	}
	if err := tt.RightClick("标签页-第二章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	click(t, tt, "菜单-关闭其他")
	if got := tabs(a, tt); got != "第二章*" {
		t.Fatalf("closing the others: %q", got)
	}
	click(t, tt, "关闭标签页-第二章")
	if got := tabs(a, tt); got != "" || shownChapter(a) != "" || !tt.HasText("从左侧选择一章开始写作") {
		t.Fatalf("the last one closed: %q, shown %q", got, shownChapter(a))
	}
	click(t, tt, "讨论", "正文")
	if shownChapter(a) != "" {
		t.Fatal("coming back to the page opened a chapter by itself")
	}
	click(t, tt, "上一个打开的章节")
	if got := tabs(a, tt); got != "第二章*" || shownChapter(a) != ids["第二章"] {
		t.Fatalf("going back after the last was closed: %q", got)
	}
}

// A tab keeps its editor while another is shown: coming back, what was
// typed can still be undone and the caret is where it was. What changed on
// the disk meanwhile is read anew.
func TestATabKeepsItsEditor(t *testing.T) {
	a, tt, ids := tabsFixture(t)
	ed := shownEditor(a)
	ed.MoveTo(3, false)
	ed.Focus()
	tt.Frame()
	tt.Type("甲")
	tt.Frame()
	click(t, tt, "章节-第二章")
	if got := disk(t, a, "第一章.txt"); !strings.Contains(got, "整条街甲的灯") {
		t.Fatalf("leaving a tab did not save it: %q", got)
	}
	click(t, tt, "标签页-第一章")
	if shownEditor(a) != ed || ed.Caret() != 4 || !ed.CanUndo() {
		t.Fatalf("the tab did not keep its editor: same %v, caret %d, undo %v", shownEditor(a) == ed, ed.Caret(), ed.CanUndo())
	}
	ed.Undo()
	tt.Frame()
	if got := ed.Doc.String(); got != "整条街的灯是同时灭的。" {
		t.Fatalf("undone after coming back: %q", got)
	}
	// Changed on the disk while another tab is shown: the tab reads it.
	click(t, tt, "标签页-第二章")
	if _, err := a.Store.WriteChapter(ids["第一章"], "磁盘上改过的一句。", nil); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "标签页-第一章")
	if shownEditor(a) == ed || shownEditor(a).Doc.String() != "磁盘上改过的一句。" {
		t.Fatalf("the tab kept words the disk no longer has: %q", shownEditor(a).Doc.String())
	}
}

// Closing a tab with words not saved asks first, and saves nothing while
// it asks: saving keeps them, not saving puts the chapter back to what the
// disk has, and cancelling leaves the tab open with its words. The tab
// says there are words not saved with a dot where its cross is.
func TestClosingAsksAboutUnsavedWords(t *testing.T) {
	a, tt, _ := tabsFixture(t)
	asked := "保存对「第一章」的修改？"
	typeIn := func(s string) {
		t.Helper()
		shownEditor(a).MoveTo(0, false)
		shownEditor(a).Focus()
		tt.Frame()
		tt.Type(s)
		tt.Frame()
	}
	typeIn("甲")
	if !a.unsaved() {
		t.Fatal("typing did not leave words unsaved")
	}
	// The dot stands where the cross does, till the pointer is on the tab.
	tt.Move(700, 500)
	tt.Frame()
	dot, ok := tt.Find("未保存-第一章")
	x, _ := tt.Find("关闭标签页-第一章")
	if !ok || dot.X < x.X || dot.X+dot.W > x.X+x.W || dot.W != 6 {
		t.Fatalf("the dot of words not saved: %+v in %+v", dot, x)
	}
	tt.Move(x.X-20, x.Y+x.H/2)
	tt.Frame()
	tt.Frame()
	if _, ok = tt.Find("未保存-第一章"); ok {
		t.Fatal("under the pointer the tab still shows the dot, not the cross")
	}
	click(t, tt, "关闭标签页-第一章")
	if !tt.HasText(asked) || !tt.HasText("不保存将丢失上次保存后的修改。") {
		t.Fatal("closing did not ask", tt.Texts())
	}
	// The three answers, in the order the system asks in, at the right.
	save, _ := tt.Find("保存修改")
	drop, _ := tt.Find("不保存")
	stay, _ := tt.Find("取消")
	if !(save.X < drop.X && drop.X < stay.X) {
		t.Fatalf("the answers are not in the system's order: %v %v %v", save.X, drop.X, stay.X)
	}
	// Asked, the chapter does not save itself.
	book.SaveAfter = 0
	for range 3 {
		tt.Frame()
	}
	if got := disk(t, a, "第一章.txt"); strings.Contains(got, "甲") {
		t.Fatalf("saved while the question was open: %q", got)
	}
	book.SaveAfter = time.Hour
	click(t, tt, "取消")
	if tt.HasText(asked) || tabs(a, tt) != "第一章*" || !a.unsaved() || !strings.HasPrefix(shownEditor(a).Doc.String(), "甲") {
		t.Fatal("cancelling did not leave the tab as it was", tabs(a, tt))
	}
	// Not saving: the tab closes, and the chapter is what the disk has.
	click(t, tt, "关闭标签页-第一章", "不保存")
	if tt.HasText(asked) || tabs(a, tt) != "" {
		t.Fatalf("not saving left %q", tabs(a, tt))
	}
	if got := disk(t, a, "第一章.txt"); strings.Contains(got, "甲") {
		t.Fatalf("not saving saved: %q", got)
	}
	click(t, tt, "章节-第一章")
	if got := shownEditor(a).Doc.String(); got != "整条街的灯是同时灭的。" {
		t.Fatalf("after not saving the chapter reads %q", got)
	}
	// Saving: the words are on the disk, and the tab closes.
	typeIn("乙")
	click(t, tt, "关闭标签页-第一章", "保存修改")
	if tt.HasText(asked) || tabs(a, tt) != "" {
		t.Fatalf("saving left %q", tabs(a, tt))
	}
	if got := disk(t, a, "第一章.txt"); !strings.HasPrefix(got, "乙整条街") {
		t.Fatalf("saving did not save: %q", got)
	}
	// Going to another tab does not ask: the words are saved on the way.
	click(t, tt, "章节-第一章")
	typeIn("丙")
	click(t, tt, "章节-第二章")
	if tt.HasText(asked) || !strings.HasPrefix(disk(t, a, "第一章.txt"), "丙乙") {
		t.Fatal("going to another tab asked, or did not save")
	}
}

// A chapter moved to the trash takes its tab with it; when it was the one
// shown the tab beside it is shown, or with none left the chapter standing
// where it stood.
func TestTheTrashTakesTheTab(t *testing.T) {
	a, tt, ids := tabsFixture(t)
	click(t, tt, "章节-第二章", "章节-第三章")
	n, _ := chapters(a).Node(ids["第二章"])
	a.trashNode(n)
	tt.Frame()
	tt.Frame()
	if got := tabs(a, tt); got != "第一章 第三章*" {
		t.Fatalf("a tab not shown, trashed: %q", got)
	}
	n, _ = chapters(a).Node(ids["第三章"])
	a.trashNode(n)
	tt.Frame()
	tt.Frame()
	if got := tabs(a, tt); got != "第一章*" || shownEditor(a) == nil {
		t.Fatalf("the one shown, trashed: %q", got)
	}
	if len(a.write.back) > 0 && a.write.back[len(a.write.back)-1] == ids["第三章"] {
		t.Fatal("going back would look for the chapter in the trash")
	}
}

// In focus the row of tabs is away with the rest.
func TestFocusPutsTheTabsAway(t *testing.T) {
	a, tt, _ := tabsFixture(t)
	tt.Key(0, ui.KeyF11)
	tt.Frame()
	tt.Frame()
	if got := tabs(a, tt); got != "" {
		t.Fatalf("focused, the tabs are still there: %q", got)
	}
}

// SHOT=dir go test ./desk -run TestShotTabs draws the row of tabs: one
// shown, words not saved, the pointer on another, more than fit, the menu
// of a tab, the question about saving, and the dark of it.
func TestShotTabs(t *testing.T) {
	testshot.Dir(t)
	a, tt, _ := tabsFixture(t)
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})

	click(t, tt, "章节-第二章", "章节-第三章")
	if err := openOutline(tt, "第一卷"); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "标签页-第二章")
	tt.Move(700, 500)
	shot(t, tt, "tabs.png")
	shownEditor(a).MoveTo(0, false)
	shownEditor(a).Focus()
	tt.Frame()
	tt.Type("雨还在下。")
	shot(t, tt, "tabs-unsaved.png")
	r, _ := tt.Find("标签页-第三章")
	tt.Move(r.X+30, r.Y+r.H/2)
	shot(t, tt, "tabs-hover.png")
	if err := tt.RightClick("标签页-第三章"); err != nil {
		t.Fatal(err)
	}
	shot(t, tt, "tabs-menu.png")
	a.menu = kit.MenuState{}
	tt.Move(700, 500)
	tt.Frame()
	click(t, tt, "关闭标签页-第二章")
	shot(t, tt, "tabs-ask.png")
	tt.SetDark(true)
	shot(t, tt, "tabs-ask-dark.png")
	click(t, tt, "取消")
	shot(t, tt, "tabs-dark.png")
	tt.SetDark(false)
	click(t, tt, "关闭标签页-第二章", "不保存")
	for i := range 12 {
		id, err := a.Store.NewChapter("", fmt.Sprintf("第%d章：很长的一个章节名字", i+4), false)
		if err != nil {
			t.Fatal(err)
		}
		markUnchecked(a)
		tt.Frame()
		a.openNode(id)
	}
	shot(t, tt, "tabs-many.png")
	click(t, tt, "关闭标签页-第15章：很长的一个章节名字")
	tt.Key(ui.Cmd, ui.KeyW)
	a.shutTabs(slices.Clone(openTabs(a)))
	shot(t, tt, "tabs-none.png")
}

// The tabs open, and which is shown, are kept with the work: it opens again
// with them. So is the page left blank by closing them all.
func TestTabsComeBackWithTheWork(t *testing.T) {
	a, tt, ids := tabsFixture(t)
	click(t, tt, "章节-第三章", "章节-第二章", "标签页-第三章")
	again := func() {
		t.Helper()
		path := a.path
		a.closeWork()
		if err := a.Open(path); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if shownPage(a) != pageWrite {
			click(t, tt, "正文")
		}
		tt.Frame()
	}
	again()
	if got := tabs(a, tt); got != "第一章 第三章* 第二章" || shownChapter(a) != ids["第三章"] || shownEditor(a) == nil {
		t.Fatalf("the work opened again with %q", got)
	}
	a.shutTabs(slices.Clone(openTabs(a)))
	tt.Frame()
	again()
	if got := tabs(a, tt); got != "" || shownChapter(a) != "" || !tt.HasText("从左侧选择一章开始写作") {
		t.Fatalf("closed, every tab; opened again with %q", got)
	}
}

// Going to another chapter by a key while the question is open takes the
// question away: the words are saved on the way, as going always does.
func TestTheQuestionGoesWithItsChapter(t *testing.T) {
	a, tt, _ := tabsFixture(t)
	click(t, tt, "章节-第二章", "标签页-第一章")
	shownEditor(a).Focus()
	tt.Frame()
	tt.Type("甲")
	tt.Frame()
	a.shutTabs([]string{shownChapter(a)})
	tt.Frame()
	if !tt.HasText("保存对「第一章」的修改？") {
		t.Fatal("not asked", tt.Texts())
	}
	a.openNode(openTabs(a)[1])
	tt.Frame()
	tt.Frame()
	if tt.HasText("保存对「第一章」的修改？") || len(a.write.closing) > 0 || tabs(a, tt) != "第一章 第二章*" {
		t.Fatal("the question stayed for a chapter no longer shown", tabs(a, tt))
	}
	if got := disk(t, a, "第一章.txt"); !strings.Contains(got, "甲") {
		t.Fatalf("going on did not save: %q", got)
	}
}

// A chapter gone from the disk keeps its tab while its words are still
// here, also while another tab is shown; closing the tab asks, for they are
// nowhere else, and saving puts them in a new chapter in that tab.
func TestALostChapterKeepsItsTab(t *testing.T) {
	a, tt, ids := tabsFixture(t)
	click(t, tt, "章节-第二章", "标签页-第一章")
	if err := os.Remove(filepath.Join(a.Store.Root(), "chapters", "第一章.txt")); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	tt.Frame()
	tt.Frame()
	if !chapterLost(a) || tabs(a, tt) != "第一章* 第二章" {
		t.Fatalf("gone from the disk: lost %v, tabs %q", chapterLost(a), tabs(a, tt))
	}
	click(t, tt, "标签页-第二章", "标签页-第一章")
	if !chapterLost(a) || shownEditor(a) == nil || shownEditor(a).Doc.String() != "整条街的灯是同时灭的。" {
		t.Fatal("coming back to the tab, the words are not there")
	}
	click(t, tt, "关闭标签页-第一章")
	if !tt.HasText("保存对「第一章」的修改？") {
		t.Fatal("closing a chapter whose words are only here did not ask", tt.Texts())
	}
	click(t, tt, "保存修改")
	if chapterLost(a) || shownChapter(a) == ids["第一章"] || tabs(a, tt) != "第一章* 第二章" {
		t.Fatalf("saved as a new chapter: lost %v, tabs %q", chapterLost(a), tabs(a, tt))
	}
	if got := disk(t, a, "第一章.txt"); got != "整条街的灯是同时灭的。" {
		t.Fatalf("the new chapter reads %q", got)
	}
}

// Changed outside and not yet chosen between, the chapter's words here are
// not the disk's: closing its tab asks, and saving keeps the ones here.
func TestClosingAsksBetweenTwoTexts(t *testing.T) {
	a, tt, _ := tabsFixture(t)
	if err := os.WriteFile(filepath.Join(a.Store.Root(), "chapters", "第一章.txt"), []byte("别处改的一句。"), 0644); err != nil {
		t.Fatal(err)
	}
	markUnchecked(a)
	tt.Frame()
	tt.Frame()
	if !diskChanged(a) {
		t.Fatal("the change outside was not noticed")
	}
	click(t, tt, "关闭标签页-第一章")
	if !tt.HasText("保存对「第一章」的修改？") {
		t.Fatal("closing did not ask", tt.Texts())
	}
	click(t, tt, "保存修改")
	if got := disk(t, a, "第一章.txt"); got != "整条街的灯是同时灭的。" || tabs(a, tt) != "" {
		t.Fatalf("saved %q, tabs %q", got, tabs(a, tt))
	}
}

// The row of tabs scrolls to the tab shown as it becomes the one shown,
// and then stays where the author scrolls it.
func TestTheRowOfTabsScrollsFreely(t *testing.T) {
	a, tt, _ := writingFixture(t, "原来的。")
	var ids []string
	for i := 2; i <= 14; i++ {
		id, err := a.Store.NewChapter("", fmt.Sprintf("第%d章：很长很长的一个章节名字", i), false)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	a.warn(a.loadTree())
	for _, id := range ids {
		a.openNode(id)
		tt.Frame()
	}
	tt.Frame()
	last := "标签页-第14章：很长很长的一个章节名字"
	inView := func(r ui.Rect) bool {
		return r.W > 0 && r.X >= a.LastWrite.X && r.X+r.W <= a.LastWrite.X+a.LastWrite.W+1
	}
	shown, ok := tt.Find(last)
	if !ok || !inView(shown) {
		t.Fatalf("the tab opened is out of view: %+v in %+v", shown, a.LastWrite)
	}
	// Toward the start of the row: the tab shown goes off to the right.
	for range 6 {
		tt.Scroll(shown.X+10, shown.Y+10, -400, 0)
		tt.Frame()
	}
	away, _ := tt.Find(last)
	if inView(away) {
		t.Fatalf("the row did not scroll away from the tab shown: %+v, before %+v", away, shown)
	}
	for range 3 {
		tt.Frame()
	}
	if still, _ := tt.Find(last); still != away {
		t.Fatalf("the row went back by itself: %+v, then %+v", away, still)
	}
	// Shown again, it comes back into view.
	a.openNode(ids[0])
	tt.Frame()
	tt.Frame()
	a.openNode(ids[len(ids)-1])
	tt.Frame()
	tt.Frame()
	if shown, _ = tt.Find(last); !inView(shown) {
		t.Fatalf("the tab shown again is out of view: %+v in %+v", shown, a.LastWrite)
	}
}
