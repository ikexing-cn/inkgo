package check

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/internal/testshot"

	"writer/ext/check/checktext"
	"writer/model"
	"writer/workspace"
)

const checkProse = "整条街的灯是同时灭的。\n林婉站在巷口，手电在工具箱最底下。她没有去摸，一个人走进了没有灯的巷子。\n老周在巷子那头等她，手里拎着一只旧灯泡。\n"

const checkAnswer = `{"findings":[
	{"kind":"conflict","title":"林晚怕黑，这里却一个人进了黑巷子","quote":"一个人走进了没有灯的巷子","entry":1,"says":"但她很怕黑"},
	{"kind":"name","title":"「林婉」是不是「林晚」写错了？","quote":"林婉","name":"林晚"},
	{"kind":"new","title":"老周","quote":"老周在巷子那头等她","name":"老周","category":"人物","text":"在巷子那头等林晚的人"}]}`

// checkFixture is a chapter at odds with the setting in three ways, and a
// model that says so.
func checkFixture(t *testing.T) (*testApp, *ui.Tester, string, *stream) {
	t.Helper()
	a, tt, id := writingFixture(t, checkProse+strings.Repeat("她蹲下去拧开灯座。", 30))
	a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角靠维修路灯谋生，但她很怕黑。她宁可绕路，沿着亮着的灯走回家。", Names: []string{"林晚"}})
	f := &stream{pieces: []string{checkAnswer}}
	penWith(a, f)
	tt.Frame()
	return a, tt, id, f
}

// A chapter is checked when asked; what is found is counted at its foot,
// in the sidebar and in the pending list, and looked at one by one on the
// page 检查, where the author says what to do about each.
func TestCheckAChapterAndSettleWhatItFinds(t *testing.T) {
	a, tt, id, f := checkFixture(t)
	if !tt.HasText("本章尚未检查") || !tt.HasText("写完一章后检查 ▴") || tt.HasText("已保存") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("立即检查"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the check", func() bool { return tt.HasText(checktext.StatusIssues(3)) })
	if len(f.asked) != 1 || !tt.HasText("待处理 3") || !existsCheck(a, id) {
		t.Fatal(len(f.asked), tt.Texts())
	}
	if err := tt.Click("待处理"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("检查") || !tt.HasText("「老周」尚未加入设定") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("待处理-「林婉」是不是「林晚」写错了？"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if shownPage(a) != pageCheck || !tt.HasText("全部 3") || !tt.HasText("名字有误 1") || !tt.HasText("2 / 3") || !tt.HasText("设定 · 人物 · 林晚") {
		t.Fatal(shownPage(a), tt.Texts())
	}

	// 稍后再说 goes on; 加入设定 writes the entry and the finding is gone.
	if err := tt.Click("稍后再说"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("3 / 3") || !tt.HasText("可加入设定 · 人物") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("加入设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	named := false
	for _, e := range setting(a).Entries {
		if len(e.Names) == 1 && e.Names[0] == "老周" && e.Text == "在巷子那头等林晚的人" && e.Place == workspace.PlaceBody && e.Sources[0].Chapter == id {
			named = true
		}
	}
	if !named || !tt.HasText("全部 2") {
		t.Fatal(named, tt.Texts())
	}

	// 无需修改 is remembered on disk and not counted again.
	if err := tt.Click("检查-林晚怕黑，这里却一个人进了黑巷子"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("无需修改"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	k, _ := a.Store.ReadCheck(id)
	if !tt.HasText("全部 1") || !k.Findings[0].Meant || k.Findings[1].Meant {
		t.Fatal(tt.Texts(), k)
	}

	// 修改正文 opens the chapter with the words selected; once they are put
	// right and the chapter left, nothing is left to look at, and the
	// chapter is checked again, being changed.
	if err := tt.Click("修改正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if shownPage(a) != pageWrite || shownEditor(a).SelectedText() != "林婉" {
		t.Fatal(shownPage(a), shownEditor(a).SelectedText())
	}
	f.pieces = []string{`{"findings":[]}`}
	tt.Type("林晚")
	tt.Frame()
	if tt.HasText("待处理 1") {
		t.Fatal("still pending as it is put right", tt.Texts())
	}
	if err := tt.Click("检查"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the check on leaving", func() bool { return len(f.asked) == 2 && checkOf(a).State.Queue.Running == "" })
	tt.Frame()
	if !tt.HasText("没有发现不一致") || !strings.Contains(f.asked[1].Messages[0].Content, "【作者说过是故意的】\n- 一个人走进了没有灯的巷子") {
		t.Fatal(tt.Texts())
	}
}

// A new name the checks find in several chapters is listed once, where it
// first comes, with the other chapters under it; left out of the setting
// once, it is gone from all of them.
func TestANewNameIsListedOnce(t *testing.T) {
	a, tt, _, _ := checkFixture(t)
	if err := tt.Click("立即检查"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the check", func() bool { return tt.HasText(checktext.StatusIssues(3)) })
	text := "老周把旧灯泡放在窗台上。" + strings.Repeat("雨一直下。", 40)
	second, err := a.Store.NewChapter("", "第二章", false)
	if err == nil {
		_, err = a.Store.WriteChapter(second, text, nil)
	}
	if err == nil {
		err = a.Store.WriteCheck(second, workspace.Check{Of: workspace.ProseHash(text), Words: workspace.Chinese.Count(text), By: "test", At: time.Now(),
			Findings: []workspace.Finding{{ID: "n2", Kind: workspace.FindNew, Title: "老周", Quote: "老周把旧灯泡放在窗台上", Name: "老周", Category: "人物"}}})
	}
	if err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	a.loadChecks()
	if err := tt.Click("检查"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("全部 3") || !tt.HasText("新出现 1") || !tt.HasText("人物 · 「第一章」起出现，共 2 章") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("检查-老周"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("其他章节中也出现") || !tt.HasText("老周把旧灯泡放在窗台上") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("忽略"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("全部 2") || !tt.HasText("新出现 0") {
		t.Fatal("left out in one chapter, still listed for another", tt.Texts())
	}
}

func TestPendingNameOpensCurrentFinding(t *testing.T) {
	a, tt, _, _ := checkFixture(t)
	if err := tt.Click("立即检查"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the check", func() bool { return tt.HasText(checktext.StatusIssues(3)) })
	text := "老周把旧灯泡放在窗台上。" + strings.Repeat("雨一直下。", 40)
	second, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.WriteChapter(second, text, nil); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.WriteCheck(second, workspace.Check{Of: workspace.ProseHash(text), Words: workspace.Chinese.Count(text), By: "test", At: time.Now(),
		Findings: []workspace.Finding{{ID: "n2", Kind: workspace.FindNew, Title: "老周", Quote: "老周把旧灯泡放在窗台上", Name: "老周", Category: "人物"}}}); err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	a.loadChecks()
	a.openNode(second)
	tt.Frame()
	group, ok := a.checkPending()
	if !ok {
		t.Fatal("no pending findings")
	}
	opened := false
	for _, item := range group.items {
		if item.text == "「老周」尚未加入设定" {
			item.open(ui.Services{})
			opened = true
		}
	}
	if !opened {
		t.Fatal("no pending 老周")
	}
	if shownPage(a) != pageCheck || checkOf(a).State.Shown != "n2" {
		t.Fatalf("pending name: page %s, finding %s", string(shownPage(a)), checkOf(a).State.Shown)
	}
	tt.Frame()
	if !tt.HasText("其他章节中也出现") || !tt.HasText("老周把旧灯泡放在窗台上") {
		t.Fatal("the grouped finding did not open", tt.Texts())
	}
}

// Findings on one entry are alike where they hold to one sentence of it.
// When to check is the work's: asked only, nothing is checked on leaving;
// the book is checked from the page, after saying how many times the AI
// is used.
func TestCheckOnlyWhenAsked(t *testing.T) {
	a, tt, _, f := checkFixture(t)
	if err := tt.Click("检查时机"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("手动检查"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if checkOf(a).State.When != checkOnDemand || !tt.HasText("手动检查 ▴") {
		t.Fatal(checkOf(a).State.When, tt.Texts())
	}
	shownEditor(a).Focus()
	tt.Type("雨。")
	tt.Frame()
	if err := tt.Click("检查"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if len(f.asked) != 0 || checkOf(a).State.Queue.Running != "" || !tt.HasText("尚未检查") {
		t.Fatal(len(f.asked), tt.Texts())
	}
	if err := tt.Click("检查全书"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("1 章未检查或有改动") || !tt.HasText("每章调用一次 AI，共 1 次。") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("开始检查"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the check of the book", func() bool { return tt.HasText("全部 3") })
	if !tt.HasText("已检查至「第一章」") || !tt.HasText("重新检查") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("重新检查"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(f.asked) != 1 || hintOf(a) != "各章均已检查，正文无新改动" {
		t.Fatal(len(f.asked), hintOf(a))
	}

	// The choice is kept with the work.
	a.loadPrefs()
	if checkOf(a).State.When != checkOnDemand {
		t.Fatal("the choice was not kept")
	}
}

func TestCloseCancelsARunningCheck(t *testing.T) {
	a, tt, id, f := checkFixture(t)
	f.pieces, f.hold = []string{`{"findings":`, `[]}`}, make(chan struct{})
	checkOf(a).State.Queue.Enqueue(id)
	tt.Frame()
	until(t, tt, "the check request", func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return len(f.asked) > 0
	})
	done, store := checkOf(a).State.Runner.Done, a.Store
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if !errors.Is(result.Err, context.Canceled) || result.Store != store {
			t.Fatal("check did not stop on window close", result.Err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("check kept running after window close")
	}
	if len(checkOf(a).State.Queue.Pending) != 0 {
		t.Fatal("queued checks survived window close")
	}
}

func TestCompletedCheckDoesNotLandInAnotherWork(t *testing.T) {
	a, tt, id, _ := checkFixture(t)
	oldStore := a.Store
	checkOf(a).State.Queue.Enqueue(id)
	if _, ok := checkOf(a).State.Queue.Next(); !ok {
		t.Fatal("check did not enter the running state")
	}
	checkOf(a).Start(checkFunc(func(context.Context, string) (workspace.Check, error) {
		return workspace.Check{Of: workspace.ProseHash(checkProse)}, nil
	}), id, ui.Services{})
	select {
	case result := <-checkOf(a).State.Runner.Done:
		if result.Err != nil || result.Store != oldStore {
			t.Fatal(result)
		}
		checkOf(a).State.Runner.Done <- result
	case <-time.After(3 * time.Second):
		t.Fatal("check did not complete")
	}
	next, err := workspace.Create(filepath.Join(t.TempDir(), "next"), "另一本", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	nextID, err := next.NewChapter("", "新章", false)
	if err != nil {
		next.Close()
		t.Fatal(err)
	}
	if err := a.attach(next, next.Root(), false); err != nil {
		next.Close()
		t.Fatal(err)
	}
	next = a.Store
	tt.Frame()
	if checkOf(a).State.Of != next || checkOf(a).State.Queue.Running != "" || len(a.book.Checks()) != 0 || len(checkOf(a).State.Queue.Failed) != 0 {
		t.Fatal("an old result changed the new work", checkOf(a).State)
	}
	if got, err := next.ReadCheck(nextID); err != nil || got.Of != "" {
		t.Fatal("an old result was saved in the new work", got, err)
	}
}

func TestCheckResultStaysStaleWhenProseChangesWhileRunning(t *testing.T) {
	a, tt, id, f := checkFixture(t)
	f.pieces, f.hold = []string{`{"findings":`, strings.TrimPrefix(checkAnswer, `{"findings":`)}, make(chan struct{})
	before := savedText(a)
	checkOf(a).State.Queue.Enqueue(id)
	tt.Frame()
	until(t, tt, "the check request", func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return len(f.asked) > 0
	})
	shownEditor(a).Insert("另一盏灯亮了。")
	a.flush()
	if savedText(a) == before {
		t.Fatal("prose was not changed during the check")
	}
	close(f.hold)
	until(t, tt, "the old prose result", func() bool {
		return a.book.Checks()[id].Of != "" && checkOf(a).State.Queue.Running == ""
	})
	got := a.book.Checks()[id]
	if got.Of != workspace.ProseHash(before) || !got.Stale(savedText(a)) {
		t.Fatal("a completed check was relabelled as current", got, savedText(a))
	}
	stored, err := a.Store.ReadCheck(id)
	if err != nil || stored.Of != got.Of || !stored.Stale(savedText(a)) {
		t.Fatal(stored, err)
	}
}

// SHOT=dir go test ./ext/check -run TestShotCheck draws the page of the checks,
// the foot of a chapter and the settings.
func TestShotCheck(t *testing.T) {
	testshot.Dir(t)
	a, tt, _, _ := checkFixture(t)

	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138}) // the Windows buttons
	if err := tt.Click("立即检查"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the check", func() bool { return tt.HasText(checktext.StatusIssues(3)) })
	checkOf(a).State.WhenOpen = true
	shot(t, tt, "check-foot.png")
	checkOf(a).State.WhenOpen = false
	showPage(a, pageCheck)
	shot(t, tt, "check-page.png")
	checkOf(a).State.Shown = ""
	checkOf(a).State.Tab = 3
	shot(t, tt, "check-new.png")
	checkOf(a).State.Tab = 0
	tt.SetDark(true)
	shot(t, tt, "check-page-dark.png")
	tt.SetDark(false)
	a.openPrefs("reminders")
	shot(t, tt, "check-settings.png")
}

// A chapter too long for the model to be given whole is not said to be
// checked: the check says how much was read.
func TestAChapterReadInPartIsToldSo(t *testing.T) {
	a, tt, id := writingFixture(t, strings.Repeat("她蹲下去拧开灯座。", 4000))
	a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角靠维修路灯谋生，但她很怕黑。", Names: []string{"林晚"}})
	f := &stream{pieces: []string{`{"findings":[]}`}}
	penWithin(a, f, model.Model{ID: "small", Window: 20000, Output: 4000, CompactAt: 80})
	tt.Frame()
	if err := tt.Click("立即检查"); err != nil {
		t.Fatal(err)
	}
	until(t, tt, "the check", func() bool { return a.book.Checks()[id].Of != "" && checkOf(a).State.Queue.Running == "" })
	tt.Frame()
	read := a.book.Checks()[id].Read
	if read <= 0 || read >= 36000 {
		t.Fatalf("read %d", read)
	}
	if want := checktext.StatusPartOK(read, "character"); !tt.HasText(want) || tt.HasText("本章检查完成，未发现不一致") {
		t.Fatal(want, tt.Texts())
	}

}

// The page of prose opens on a chapter before its text is read: that is
// no chapter left, and what its check found stays, on the disk and in the
// count.
func TestShowingAChapterKeepsWhatItsCheckFound(t *testing.T) {
	a, tt, _ := fixture(t)
	quick(a)
	id, err := a.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(id, checkProse, nil); err != nil {
		t.Fatal(err)
	}
	found := workspace.Finding{ID: workspace.NewID(), Kind: workspace.FindName, Title: "「林婉」是不是「林晚」写错了？", Quote: "林婉", Name: "林晚"}
	if err = a.Store.WriteCheck(id, workspace.Check{Of: workspace.ProseHash(checkProse), Words: workspace.Chinese.Count(checkProse), By: "deepseek/deepseek-flash", At: time.Now(), Findings: []workspace.Finding{found}}); err != nil {
		t.Fatal(err)
	}
	a.loadChecks()
	if err = tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	k, err := a.Store.ReadCheck(id)
	if shownChapter(a) != id || err != nil || len(k.Findings) != 1 || len(a.book.Checks()[id].Findings) != 1 {
		t.Fatalf("showing the chapter, what its check found went: %+v %v", k, err)
	}
}

// A check that failed is not told on the page as one that found nothing:
// the page names the first chapter in the book that failed, the same one
// every frame, and the line above says how many failed rather than how
// far the checks went.
func TestAFailedCheckIsNotToldAsClear(t *testing.T) {
	a, tt, id, _ := checkFixture(t)
	second, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.warn(a.loadTree())
	a.loadChecks()
	checkOf(a).State.Queue.Fail(second, errors.New("请求超时"))
	checkOf(a).State.Queue.Fail(id, errors.New("无法连接模型服务"))
	if err := tt.Click("检查"); err != nil {
		t.Fatal(err)
	}
	for range 8 {
		tt.Frame()
		if !tt.HasText("检查未完成") || !tt.HasText("「第一章」检查失败：无法连接模型服务") || tt.HasText("尚未检查") {
			t.Fatal(tt.Texts())
		}
	}
	said := ""
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, "已检查") {
			said = s
		}
	}
	if !strings.HasSuffix(said, "章，2 章失败") {
		t.Fatalf("the line above says %q", said)
	}
}

// A check that failed says why at the chapter's foot, and the whole of it
// on hover where the line is cut short.
func TestAFailedCheckSaysWhyInFull(t *testing.T) {
	a, tt, id, _ := checkFixture(t)
	checkOf(a).State.Queue.Fail(id, errors.New("请在设置中「AI」配置里选择续写和检查模型"))
	tt.Frame()
	line, _, can, tip := a.checkState()
	if want := "检查失败：" + a.errText(checkOf(a).State.Queue.Failed[id]); line != want || tip != want || !can {
		t.Fatalf("%q, tip %q, can %v", line, tip, can)
	}
}
