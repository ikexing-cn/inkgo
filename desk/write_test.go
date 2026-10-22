package desk

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/editor"
	"writer/export"
	"writer/shelf"
	"writer/workspace"
)

// writingFixture opens the writing page of a work holding a chapter of
// text.
func writingFixture(t *testing.T, text string) (*App, *ui.Tester, string) {
	t.Helper()
	quick(t)
	a, tt, _ := fixture(t)
	id, err := a.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(id, text, nil); err != nil {
		t.Fatal(err)
	}
	if err = tt.Click("正文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if shownChapter(a) != id || shownEditor(a) == nil {
		t.Fatal("the chapter did not open", tt.Texts())
	}
	return a, tt, id
}

// The chapter's head and the AI's bar start where the prose starts, in a
// window that leaves the prose narrow and in one that centers it; the find
// bar is a band over the page.
func TestTheHeadLinesUpWithTheProse(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("雨从傍晚开始下。", 30))
	a.sideHidden = true // whose 正文 is not the prose's
	ed := shownEditor(a)
	ed.Select(0, ed.Doc.Len())
	a.openRewrite()
	for _, width := range []int{1000, 2200} {
		tt.SetSize(width, 780)
		tt.Frame()
		tt.Frame()
		prose, _ := tt.Find("正文")
		box, ok := ed.RangeBox(0, ed.Doc.Len())
		head, _ := tt.Find("章纲 · 空")
		ask, _ := tt.Find("改写要求")
		if !ok || head.X != prose.X+box.X {
			t.Fatalf("%d wide: the head at %v, the prose at %v", width, head.X, prose.X+box.X)
		}
		// The bar's card has its icon and the box in it.
		if ask.X < prose.X+box.X || ask.X > prose.X+box.X+48 {
			t.Fatalf("%d wide: the rewrite's box at %v, the prose at %v", width, ask.X, prose.X+box.X)
		}
	}
	a.write.rewriting = false
	if err := tt.Click("查找替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	find, _ := tt.Find("查找")
	head, _ := tt.Find("章纲 · 空")
	if find.Y > head.Y {
		t.Fatal("the find bar is under the chapter's head")
	}
}

// Marks are kept beside the text and come back when the chapter opens.
func TestMarksSurviveReopening(t *testing.T) {
	a, tt, id := writingFixture(t, "井边有一把钥匙。")
	shownEditor(a).Select(5, 7)
	shownEditor(a).MarkSelection(editor.MarkForeshadow, "")
	tt.Frame()
	forgetChapter(a)
	a.openNode(id)
	marks := shownEditor(a).Doc.Marks()
	if len(marks) != 1 || shownEditor(a).Doc.Slice(marks[0].Start, marks[0].End) != "钥匙" {
		t.Fatalf("marks %+v", marks)
	}
}

// The export writes a file for each thing ticked, here, with no dialog to
// ask where, into the work's 导出 folder; what cannot be exported is said
// and left out. The pen name stays with the work and this machine, what is
// ticked and as what with this machine.
func TestExportTheWork(t *testing.T) {
	a, tt, id := writingFixture(t, "雨下着。")
	x := &a.write.exporting
	open := func() {
		t.Helper()
		if err := clickMenu(tt, "文件", "菜单-导出"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	// The dialog's title says 导出 too: the button is the one after 取消,
	// past its padding and the gap between them.
	out := func() {
		t.Helper()
		tt.ClickAt(right(t, tt, "取消")+32, func() float32 { r, _ := tt.Find("取消"); return r.Y + r.H/2 }())
		tt.Frame()
	}
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(a.Store.Root(), "导出", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	open()
	if !x.open || x.selection.Scope != export.ScopeWork || !tt.HasText("笔名") || tt.HasText("段首加两个全角空格") {
		t.Fatal("the dialog does not start from the prose of the whole work, as Word", tt.Texts())
	}
	x.pen = "雨声"
	out()
	z, err := zip.OpenReader(filepath.Join(a.Store.Root(), "导出", "测试作品.docx"))
	if err != nil {
		t.Fatal(err)
	}
	var doc []byte
	for _, f := range z.File {
		if f.Name == "word/document.xml" {
			r, _ := f.Open()
			doc, _ = io.ReadAll(r)
			r.Close()
		}
	}
	z.Close()
	if x.open || !strings.Contains(string(doc), "作者：雨声") || !strings.Contains(string(doc), "雨下着。") {
		t.Fatalf("the manuscript: %s", doc)
	}
	if a.Store.Work().Pen != "雨声" || a.shelf.Export.Pen != "雨声" {
		t.Fatal("the pen name is not kept")
	}

	// The outline alone, none written: it is said, and nothing goes.
	a.shelf.Export.Prose, a.shelf.Export.Outline = false, true
	open()
	if x.pen != "雨声" || !tt.HasText("所选范围内还没有写章纲，不导出大纲。") {
		t.Fatal("the pen name or why the outline cannot go", tt.Texts())
	}
	out()
	if !x.open {
		t.Fatal("导出 went with nothing to export")
	}
	shut(tt, &x.open)

	// The three as plain text, into one folder.
	if err := a.Store.WriteOutline(id, "她走了"); err != nil {
		t.Fatal(err)
	}
	a.shelf.Export.Prose, a.shelf.Export.Setting = true, true
	open()
	if err := tt.Click("纯文本"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("段首加两个全角空格") || tt.HasText("笔名") {
		t.Fatal("plain text asks for a pen name, or not about the indent", tt.Texts())
	}
	out()
	if got := read("测试作品.txt"); got != "测试作品\n\n第一章\n\n　　雨下着。\n" {
		t.Fatalf("prose %q", got)
	}
	if got := read("测试作品-大纲.txt"); got != "测试作品 · 大纲\n\n第一章\n\n她走了\n" {
		t.Fatalf("outline %q", got)
	}
	if got := read("测试作品-设定.txt"); !strings.Contains(got, "人物\n\n主角怕黑") {
		t.Fatalf("setting %q", got)
	}
	if !strings.HasPrefix(a.notice, "已导出 3 个文件到 ") {
		t.Fatal(a.notice)
	}
	s, _, err := shelf.Load(a.books.file)
	if err != nil || s.Export == nil || *s.Export != (shelf.Export{Options: export.Options{Prose: true, Outline: true, Setting: true, Text: true}, Pen: "雨声"}) {
		t.Fatalf("this machine keeps %+v %v", s.Export, err)
	}

	// The chapter open, named after it.
	a.shelf.Export.Outline, a.shelf.Export.Setting = false, false
	open()
	if err := tt.Click(export.ScopeChapter); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	out()
	if got := read("测试作品-第一章.txt"); got != "第一章\n\n　　雨下着。\n" {
		t.Fatalf("a chapter %q", got)
	}
}

// A work never opened on this machine opens on its prose, as a new one
// does, as the prototype opens every book; one fetched without .writer/
// keeps where it was left all the same.
func TestAWorkNewHereOpensOnItsProse(t *testing.T) {
	a, _, _ := fixture(t)
	root := a.Store.Root()
	a.Close()
	if err := os.RemoveAll(filepath.Join(root, ".writer")); err != nil {
		t.Fatal(err)
	}
	b := newApp(a.ai.ConfigFile())
	if err := b.Open(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	if b.page != pageWrite {
		t.Fatal("opened on", b.page)
	}
	// Where it was left is kept, though .writer/ was not there.
	tb := ui.NewTester(b.View, 1266, 780)
	tb.Frame()
	if err := tb.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tb.Frame()
	if _, err := os.Stat(filepath.Join(root, ".writer", "desk.json")); err != nil || b.errorText() != "" {
		t.Fatal(err, b.errorText())
	}
	b.openCreate()
	b.name, b.books.at = "新书", filepath.Join(t.TempDir(), "新书")
	if err := b.create(); err != nil || b.errorText() != "" {
		t.Fatal(err, b.errorText())
	}
	if b.page != pageWrite {
		t.Fatal("a new work opened on", b.page)
	}
}

// In the narrowest window the column gives way to the page, and the counts
// to the chapter's title.
func TestNarrowWindowKeepsThePage(t *testing.T) {
	a, _, _ := writingFixture(t, "雨从傍晚开始下。")
	if w := a.panelWidth(1000); w != panelMin {
		t.Fatal(w)
	}
	if w := a.panelWidth(1280); w != panelWidth {
		t.Fatal(w)
	}
	tt := ui.NewTester(a.View, 1000, 700)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("8 字") || tt.HasText("全书 8 字") {
		t.Fatal(tt.Texts())
	}
}

// Writing in focus leaves the page alone; quoting into the talk brings
// the column back, since the talk is in it.
func TestFocusLeavesThePageAlone(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨下了一夜。")
	if err := tt.Click("专注写作"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("章节-第一章") || tt.HasText("右栏-设定") || !tt.HasText("退出专注") {
		t.Fatal("the tree or the column is still there", tt.Texts())
	}
	if err := tt.Click("退出专注"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("章节-第一章") || !tt.HasText("右栏-设定") {
		t.Fatal(tt.Texts())
	}
	a.write.focus = true
	shownEditor(a).Select(0, 2)
	a.quoteToTalk()
	tt.Frame()
	if a.write.focus || !tt.HasText("右栏-讨论") {
		t.Fatal("quoting kept the column away", tt.Texts())
	}
	// F11 from the prose goes in and out.
	shownEditor(a).Focus()
	tt.Key(0, ui.KeyF11)
	tt.Frame()
	if !a.write.focus {
		t.Fatal("F11 did not leave the page alone")
	}
	tt.Key(0, ui.KeyF11)
	tt.Frame()
	if a.write.focus {
		t.Fatal("F11 did not bring the tree and the column back")
	}
}

// The writing switches in the settings are kept as they are ticked, not
// only with the models' save.
func TestWritingSwitchesKeepAsTicked(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置-AI"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("停笔后自动续写"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if p := savedPrefs(t, a); !p.AutoPredict {
		t.Fatalf("prefs %+v", p)
	}
}

// Ctrl+N makes a chapter after the one open, ready to be named.
func TestCtrlNMakesAChapter(t *testing.T) {
	a, tt, one := writingFixture(t, "一")
	shownEditor(a).Focus()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyN)
	tt.Frame()
	if shownChapter(a) == one || a.write.renaming != shownChapter(a) {
		t.Fatal("no new chapter to name", shownChapter(a), a.write.renaming)
	}
	if n, _ := chapters(a).Node(shownChapter(a)); n.Folder || n.Parent.Children[0].ID != one {
		t.Fatal("the new chapter is not after the one open")
	}
}

// Ctrl+/ shows the shortcuts; Esc puts them away.
func TestShortcutsHelp(t *testing.T) {
	a, tt, _ := writingFixture(t, "一")
	tt.Key(ui.Cmd, ui.KeySlash)
	tt.Frame()
	if !a.keysOpen || !tt.HasText("快捷键") || !tt.HasText("下一章 / 上一章") {
		t.Fatal("no shortcuts", a.keysOpen, tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.keysOpen {
		t.Fatal("the shortcuts stay")
	}
}

// The window is named after the work, and on the writing page after the
// chapter open too.
func TestWindowTitle(t *testing.T) {
	a, tt, _ := writingFixture(t, "一")
	var titles []string
	a.SetTitle = func(s string) bool { titles = append(titles, s); return true }
	work := a.Store.Work().Name
	tt.Frame()
	tt.Frame()
	if want := "第一章 — " + work; len(titles) != 1 || titles[0] != want {
		t.Fatalf("titles %q, want once %q", titles, want)
	}
	if err := tt.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if titles[len(titles)-1] != work {
		t.Fatalf("discussing: %q", titles)
	}
}

// A folder lists each chapter with the start of its outline, or of its
// prose when it has no outline.
func TestFolderShowsTheGists(t *testing.T) {
	a, tt, _ := writingFixture(t, "一")
	vol, err := a.Store.NewChapter("", "第一卷", true)
	if err != nil {
		t.Fatal(err)
	}
	planned, _ := a.Store.NewChapter(vol, "第二章", false)
	written, _ := a.Store.NewChapter(vol, "第三章", false)
	if err = a.Store.WriteOutline(planned, "她找到\n井底的钥匙"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(written, "老周说出旧事。"+strings.Repeat("雨", 300), nil); err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.openNode(vol)
	tt.Frame()
	if !tt.HasText("她找到 井底的钥匙") || !tt.HasText("老周说出旧事。"+strings.Repeat("雨", 193)) {
		t.Fatal(tt.Texts())
	}
}

// A chapter changed and left asks the AI nothing when it is checked only
// on demand: there is no summary to write.
func TestLeavingAChangedChapterAsksNothing(t *testing.T) {
	a, tt, _ := writingFixture(t, strings.Repeat("她蹲下去拧开灯座。", 60))
	f := &stream{pieces: []string{"不该有人问。"}}
	penWith(a, f)
	a.ck.when = checkOnDemand
	two, err := a.Store.NewChapter("", "第二章", false)
	if err != nil {
		t.Fatal(err)
	}
	a.SetError(a.loadTree())
	shownEditor(a).MoveTo(shownEditor(a).Doc.Len(), false)
	shownEditor(a).Insert("她推开门。")
	a.openNode(two)
	for i := 0; i < 5; i++ {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.asked) > 0 {
		t.Fatalf("leaving asked the AI %d times", len(f.asked))
	}
}

// The settings' reminders count the foreshadowings though the writing page
// was never shown: they read the tree and the marks themselves.
func TestRemindersBeforeTheWritingPage(t *testing.T) {
	a, tt, _ := fixture(t)
	var first string
	for i := 1; i <= 12; i++ {
		id, err := a.Store.NewChapter("", fmt.Sprintf("第%d章", i), false)
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			first = id
		}
	}
	marks := []workspace.ChapterMark{{ID: "f1", Kind: workspace.MarkForeshadow, Start: 0, End: 2}}
	if _, err := a.Store.WriteChapter(first, "钥匙在井里。", marks); err != nil {
		t.Fatal(err)
	}
	if treeRead(a) {
		t.Fatal("the writing page was shown")
	}
	a.write.lateAfter = lateMin
	a.openPrefs("提醒与检查")
	tt.Frame()
	if !tt.HasText("当前有 1 个伏笔久未回收。") {
		t.Fatal(tt.Texts())
	}
}

// The prose keeps the keyboard as what is around it comes and goes: the
// title bar and the sidebar in focus mode, the sidebar hidden, the find bar
// over it.
func TestTheProseKeepsTheKeyboard(t *testing.T) {
	a, tt, _ := writingFixture(t, "林晚说，林晚走了。")
	shownEditor(a).Focus()
	tt.Frame()
	tt.Frame()
	steps := []struct {
		name string
		do   func()
	}{
		{"focus mode", func() { tt.Key(0, ui.KeyF11) }},
		{"out of focus mode", func() { tt.Key(0, ui.KeyF11) }},
		{"the sidebar hidden", func() { a.sideHidden = true }},
		{"the sidebar shown", func() { a.sideHidden = false }},
		{"the find bar over it", func() { a.write.finding = true; a.write.query = "林晚" }},
	}
	for _, s := range steps {
		if s.name == "the find bar over it" {
			shownEditor(a).Focus() // opened from the menu, the find bar is not focused
		}
		s.do()
		tt.Frame()
		tt.Frame()
		if !tt.Focused("正文") {
			t.Errorf("%s: the prose lost the keyboard", s.name)
		}
	}
}
