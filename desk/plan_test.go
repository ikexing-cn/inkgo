package desk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

type planFixtureData struct {
	volume, one, two, three string
	lines                   []workspace.PlotLine
	cards                   []workspace.Card
}

func outlineFixture(t *testing.T) (*App, *ui.Tester, planFixtureData) {
	t.Helper()
	a, tt, _ := fixture(t)
	var f planFixtureData
	chapter := func(parent, title string, folder bool) string {
		id, err := a.Store.NewChapter(parent, title, folder)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	f.volume = chapter("", "第一卷：停电", true)
	f.one = chapter(f.volume, "第一章：灯数", false)
	f.two = chapter(f.volume, "第二章：档案", false)
	f.three = chapter("", "番外：晴天", false)
	for _, name := range []string{"查灯", "母亲", "小祥"} {
		l, err := a.Store.SaveLine(workspace.PlotLine{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		f.lines = append(f.lines, l)
	}
	var err error
	for _, x := range []struct{ title, when, chapter, line, about string }{
		{"母亲在停电那晚失踪", "十九年前", f.two, f.lines[1].ID, ""},
		{"何主任让重新数灯", "十月五日", f.one, f.lines[0].ID, ""},
		{"停电通知", "十月六日", f.two, f.lines[1].ID, "何主任的字"},
		{"小祥改口叫她姐", "十月六日", f.two, f.lines[2].ID, ""},
		{"林晚下井", "十一月", "", f.lines[1].ID, "石板"},
		{"锁着档案柜", "", "", "", "还没想好是谁让他锁的"},
	} {
		after := ""
		if len(f.cards) > 0 {
			after = f.cards[len(f.cards)-1].ID
		}
		e, eerr := a.Store.SaveCard(workspace.Card{Title: x.title, When: x.when, Chapter: x.chapter, Line: x.line, About: x.about}, after)
		if eerr != nil {
			t.Fatal(eerr)
		}
		f.cards = append(f.cards, e)
	}
	err = a.Store.WriteOutline(f.volume, "旧城的灯一盏一盏查下去")
	if err != nil {
		t.Fatal(err)
	}
	err = a.Store.WriteOutline(f.one, "何主任要一份灯数。")
	if err != nil {
		t.Fatal(err)
	}
	click(t, tt, "大纲")
	tt.Frame()
	return a, tt, f
}

func planType(t *testing.T, tt *ui.Tester, label, text string) {
	t.Helper()
	click(t, tt, label)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(text)
	tt.Frame()
}

func TestOutlineReadError(t *testing.T) {
	a, tt, f := outlineFixture(t)
	root := a.Store.Root()
	click(t, tt, "正文")
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "outline.json")
	bad := []byte("{ broken outline")
	if err := os.WriteFile(file, bad, 0600); err != nil {
		t.Fatal(err)
	}
	// The command line accepts a relative work path. Revealing a broken
	// outline must still hand the file manager the store's absolute root.
	t.Chdir(filepath.Dir(root))
	if err := a.Open(filepath.Base(root)); err != nil {
		t.Fatal("bad outline blocked opening work", err)
	}
	showPage(a, pageWrite)
	a.openNode(f.two)
	tt.Frame()
	if shownEditor(a) == nil {
		t.Fatal("prose unavailable")
	}
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, "故事中 ") {
			t.Fatal("bad outline has story time", text)
		}
	}
	var revealed string
	a.Reveal = func(path string) error { revealed = path; return nil }
	click(t, tt, "大纲")
	click(t, tt, "打开所在文件夹")
	if revealed != root {
		t.Fatalf("revealed %q, want absolute work root %q", revealed, root)
	}
}

func TestTheChapterInTheStory(t *testing.T) {
	a, tt, f := outlineFixture(t)
	click(t, tt, "打开正文-"+f.two)
	label := "故事中 十九年前 — 十月六日"
	hasText(t, tt, label)
	click(t, tt, label)
	if shownPage(a) != pagePlan {
		t.Fatal("did not open at first timed card", shownPage(a))
	}
	click(t, tt, "正文")
	a.openNode(f.three)
	tt.Frame()
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, "故事中 ") {
			t.Fatal("empty chapter has time", text)
		}
	}
}

func TestOutlineTrashAndSettingsPane(t *testing.T) {
	a, tt, _ := outlineFixture(t)
	if a.plan.Pane {
		t.Fatal("outline opens with pane")
	}
	click(t, tt, "展开右栏")
	hasText(t, tt, "设定")
	if _, ok := tt.Find("讨论输入"); ok {
		t.Fatal("outline has discussion pane")
	}
	click(t, tt, "收起右栏")
	if a.plan.Pane {
		t.Fatal("pane did not close")
	}
	click(t, tt, "正文")
	click(t, tt, "大纲")
	if a.plan.Pane {
		t.Fatal("pane state changed between pages")
	}
}

func TestOutlineOpensWritingWithItsSavedDraft(t *testing.T) {
	a, tt, f := outlineFixture(t)
	click(t, tt, "章纲-"+f.one)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("何主任要一份新的灯数。")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	click(t, tt, "打开正文-"+f.one)
	if shownPage(a) != pageWrite || shownChapter(a) != f.one || chapterOutline(a) != "何主任要一份新的灯数。" {
		t.Fatal("open prose", shownPage(a), shownChapter(a), chapterOutline(a))
	}
}

func TestOutlineStaysWhenAIIsOff(t *testing.T) {
	a, tt, _ := outlineFixture(t)
	a.turnAI(false)
	tt.Frame()
	hasText(t, tt, "大纲")
	if shownPage(a) != pagePlan {
		t.Fatal("AI switch removed outline")
	}
}
