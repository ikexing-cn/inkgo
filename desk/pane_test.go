package desk

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// paneIDs are the entries of paneFixture: two people, what a talk
// suggests about one of them, and a suggestion about none.
type paneIDs struct{ lin, zhou, about, fresh string }

// paneFixture is the writing page of a work whose setting names two
// people and a thing, with two suggestions a talk noted.
func paneFixture(t *testing.T) (*App, *ui.Tester, paneIDs) {
	t.Helper()
	a, tt, _ := writingFixture(t, "小祥把左手揣回兜里。\n林晚没有回头。")
	u, _ := a.Store.Append(a.session, workspace.Event{Role: workspace.RoleUser, Text: "小祥是谁"})
	add := func(by, place, category, text string, names ...string) string {
		rev := workspace.Revision{By: by}
		if by == workspace.ByTalk {
			rev.Session, rev.Turn = a.session, u.ID
		}
		rev.Ops = []workspace.Op{{Op: workspace.OpAdd, Place: place, Category: category, Text: text, Names: names,
			Sources: []workspace.Source{{Chapter: a.write.node, Quote: "小祥把左手揣回兜里。"}}}}
		r, err := a.Store.Revise(rev)
		if err != nil {
			t.Fatal(err)
		}
		return r.Ops[0].Entry
	}
	var ids paneIDs
	ids.lin = add(workspace.ByAuthor, workspace.PlaceBody, "人物", "主角，靠维修路灯谋生，但她很怕黑。", "林晚", "晚晚")
	ids.zhou = add(workspace.ByAuthor, workspace.PlaceBody, "人物", "配角，林晚的同事。左手总揣在兜里，从来不拿出来。", "周祥", "小祥", "saki")
	add(workspace.ByAuthor, workspace.PlaceBody, "物品", "一直挂在林晚脖子上，不知道开哪扇门。", "铜钥匙")
	ids.about = add(workspace.ByTalk, workspace.PlaceOpen, "人物", "“小祥”是顺口的叫法，谁都能喊；“saki”只有特定圈子里的人才用。")
	ids.fresh = add(workspace.ByTalk, workspace.PlaceOpen, workspace.Other, "第一章开头只交代了天气，人物和事件还没出现。")
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return a, tt, ids
}

// The setting is a list of one-line rows; a row opens the page of its
// entry, where what a talk suggests about it is decided.
func TestSettingListAndPage(t *testing.T) {
	a, tt, ids := paneFixture(t)
	if !tt.HasText("待确认") || !tt.HasText("周祥") || !tt.HasText("物品") || tt.HasText("第一章开头只交代了天气，人物和事件还没出现。") {
		t.Fatal("the list", tt.Texts())
	}
	about := map[string]string{}
	for _, it := range a.ideas() {
		about[it.e.ID] = it.about
	}
	if about[ids.about] != ids.zhou || about[ids.fresh] != "" {
		t.Fatal("what the suggestions are about", about)
	}

	// Searching only finds.
	n := len(a.set.Entries)
	a.doc.query = "左手"
	tt.Frame()
	if !tt.HasText("找到 1 条") || tt.HasText("铜钥匙") || len(a.set.Entries) != n {
		t.Fatal("the search", tt.Texts())
	}
	a.doc.query = ""
	tt.Frame()

	if err := tt.Click("设定-" + ids.zhou); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("别名 小祥、saki") || !tt.HasText("「第一章」“小祥把左手揣回兜里。”") || !tt.HasText("AI 建议") || !tt.HasText("删除设定") {
		t.Fatal("the page", tt.Texts())
	}
	if err := tt.Click("采纳-" + ids.about); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if e, _ := a.set.Entry(ids.about); e.Place != workspace.PlaceBody || tt.HasText("AI 建议") {
		t.Fatal("not taken", e.Place, tt.Texts())
	}

	// The rest wait in one place, a click each.
	if err := tt.Click("全部设定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("待确认"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("1 条 AI 建议") || !tt.HasText("新设定") {
		t.Fatal("what waits", tt.Texts())
	}
	if err := tt.Click("忽略-" + ids.fresh); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if e, _ := a.set.Entry(ids.fresh); e.Place != workspace.PlaceDropped || a.doc.todo || tt.HasText("待确认") {
		t.Fatal("not ignored", e.Place, a.doc.todo, tt.Texts())
	}
}

// What the AI asks the author is answered, not taken: the answer, written
// in the form, takes the question's place in the setting, which keeps the
// question beside it, and the names it was asked of.
func TestAQuestionIsAnswered(t *testing.T) {
	a, tt, _ := paneFixture(t)
	const q = "林晚怕黑，是不是和她母亲有关？"
	r, err := a.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "人物", Names: []string{"林晚", "小晚"}, Text: q}}})
	if err != nil {
		t.Fatal(err)
	}
	id := r.Ops[0].Entry
	if err = a.refresh(); err != nil {
		t.Fatal(err)
	}
	a.doc.todo = true
	tt.Frame()
	if !tt.HasText(q) || !tt.HasText("回答-"+id) || tt.HasText("采纳-"+id) {
		t.Fatal("the question", tt.Texts())
	}
	if err = tt.Click("回答-" + id); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !tt.HasText("回答") || !tt.HasText(q) || tt.HasText("新增设定") || !tt.Focused("设定内容") || a.doc.name != "林晚" || a.doc.names != "小晚" {
		t.Fatal("the form", a.doc.name, a.doc.names, tt.Texts())
	}
	const answer = "是。她母亲死在停电的那一夜。"
	a.doc.text = answer
	tt.Frame()
	if err = tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e, _ := a.set.Entry(id)
	if e.Place != workspace.PlaceBody || e.Text != answer || e.Asked != q || e.Category != "人物" || strings.Join(e.Names, "、") != "林晚、小晚" || a.doc.editing != "" {
		t.Fatalf("answered %+v", e)
	}
	if !a.doc.todo || !tt.HasText("2 条 AI 建议") {
		t.Fatal("back to what waits", tt.Texts())
	}
	a.doc.todo, a.doc.shown = false, id
	tt.Frame()
	if !tt.HasText(answer) || !tt.HasText("回答："+q) {
		t.Fatal("the entry", tt.Texts())
	}
}

// The talks are a list too; one opens from it, and a new one starts there.
func TestTalkList(t *testing.T) {
	a, tt, _ := paneFixture(t)
	old := a.session
	if err := tt.Click("右栏-讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("全部讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("最近") || !a.doc.talks {
		t.Fatal("the talks", tt.Texts())
	}
	if err := tt.Click("新讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.session == old || a.doc.talks || a.page != 3 || !tt.HasText("示例问题") {
		t.Fatal("no new talk beside the prose", a.session, a.doc.talks, a.page)
	}
	a.doc.talks = true
	tt.Frame()
	if err := tt.Click("讨论-" + old); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if a.session != old || a.doc.talks {
		t.Fatal("the talk did not open", a.session)
	}
}

// TestShotPane writes screenshots of the right column when SHOT is a
// directory: SHOT=dir go test ./desk -run ShotPane
func TestShotPane(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	a, tt, ids := paneFixture(t)
	shot := func(name string) {
		tt.Frame()
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	d := &a.doc
	shot("pane-list.png")
	d.query = "左手"
	shot("pane-find.png")
	d.query, d.shown = "", ids.zhou
	shot("pane-entry.png")
	e, _ := a.set.Entry(ids.zhou)
	d.editing, d.text, d.category, d.name, d.names = e.ID, e.Text, e.Category, "周祥", "小祥、saki"
	shot("pane-edit.png")
	d.editing, d.shown, d.text, d.name, d.names = "new", "", "老周是修表的，铺子在旧城小学对面", "老周", ""
	shot("pane-new.png")
	d.editing, d.todo = "", true
	shot("pane-todo.png")
	d.todo, d.history = false, true
	shot("pane-history.png")
	d.history = false
	tt.SetDark(true)
	shot("pane-list-dark.png")
	tt.SetDark(false)
	d.tab, d.talks = paneTalk, true
	shot("pane-talks.png")
	if err := tt.Click("新讨论"); err != nil {
		t.Fatal(err)
	}
	shot("pane-talk-new.png")
}

// A change of names is logged as one, not as a move to no category.
func TestLogNames(t *testing.T) {
	a, _, _ := fixture(t)
	if got := a.describeOp(workspace.Op{Op: workspace.OpName, Names: []string{"周祥", "小祥"}}); got != "名称改为：周祥、小祥" {
		t.Error(got)
	}
	if got := a.describeOp(workspace.Op{Op: workspace.OpName}); !strings.HasPrefix(got, "移除名称：") {
		t.Error(got)
	}
}

// Beside the talk, the setting's count is what waits on the author, as in
// the tab beside the prose, not how many entries there are.
func TestTheSettingCountsWhatWaits(t *testing.T) {
	a, tt, _ := paneFixture(t)
	a.page = 0
	tt.Frame()
	texts := tt.Texts()
	i := slices.Index(texts, "收起右栏")
	if i < 2 || texts[i-2] != "设定" || texts[i-1] != "2" {
		t.Fatal("the count beside the talk", texts)
	}
}

// The setting's form comes with the keyboard in it: a new entry at its
// text, a changed one at its name.
func TestTheEntryFormTakesTheKeyboard(t *testing.T) {
	a, tt, ids := paneFixture(t)
	if err := tt.Click("新增"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !tt.Focused("设定内容") {
		t.Fatal("a new entry starts somewhere else")
	}
	a.doc.editing = ""
	tt.Frame()
	if err := tt.Click("设定-" + ids.zhou); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("修改"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	tt.Frame()
	if !tt.Focused("名字") {
		t.Fatal("a changed entry starts somewhere else")
	}
}

func TestNearFindMeasure(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"甲乙abcdef关键后文", "甲乙abcdef关键后文"},
		{"甲乙abcdefg关键后文", "…bcdefg关键后文"},
		{"甲\n乙abcdefg关键后文", "…bcdefg关键后文"},
	} {
		if got := nearFind(c.text, "关键"); got != c.want {
			t.Errorf("nearFind(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}
