package desk

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

func TestOutlineReadError(t *testing.T) {
	a, tt, f := outlineFixture(t)
	root := a.Store.Root()
	planClick(t, tt, "正文")
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "outline.json")
	bad := []byte("{ broken outline")
	if err := os.WriteFile(file, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.Open(root); err != nil {
		t.Fatal("bad outline blocked opening work", err)
	}
	a.page = 3
	a.openNode(f.two)
	tt.Frame()
	if a.write.ed == nil {
		t.Fatal("prose unavailable")
	}
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, "故事中 ") {
			t.Fatal("bad outline has story time", text)
		}
	}
	planClick(t, tt, "大纲")
	planHas(t, tt, "大纲无法读取：")
	if toolbar, ok := tt.Find("大纲工具栏"); !ok || toolbar.H != titleHeight || toolbar.Y < titleHeight || toolbar.Y > titleHeight+1 {
		t.Fatal("error page has no outline toolbar", toolbar)
	}
	planHas(t, tt, file)
	planHas(t, tt, "正文仍可照常编辑。")
	if _, ok := tt.Find("新增情节线"); ok {
		t.Fatal("error page still offers outline editing")
	}
	if _, err := a.Store.SaveCard(workspace.Card{Title: "不能覆盖"}, ""); err == nil {
		t.Fatal("corrupt file accepted write")
	}
	if b, _ := os.ReadFile(file); string(b) != string(bad) {
		t.Fatal("bad outline overwritten")
	}
	good := `{"schema":2,"lines":[],"cards":[]}`
	if err := os.WriteFile(file, []byte(good), 0600); err != nil {
		t.Fatal(err)
	}
	planClick(t, tt, "重试")
	if a.plan.loadErr != nil {
		t.Fatal("retry failed", a.plan.loadErr)
	}
	planHas(t, tt, "新增情节线")
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
	planClick(t, tt, "大纲")
	tt.Frame()
	return a, tt, f
}
func planClick(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	tt.Frame()
}
func planType(t *testing.T, tt *ui.Tester, label, text string) {
	t.Helper()
	planClick(t, tt, label)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(text)
	tt.Frame()
}
func planHas(t *testing.T, tt *ui.Tester, text string) {
	t.Helper()
	if !tt.HasText(text) {
		t.Fatal("missing", text, tt.Texts())
	}
}
func planDrag(t *testing.T, tt *ui.Tester, from, to string) {
	t.Helper()
	a, ok := tt.Find(from)
	if !ok {
		t.Fatal(from)
	}
	b, ok := tt.Find(to)
	if !ok {
		t.Fatal(to)
	}
	x, y := a.X+a.W/2, a.Y+a.H/2
	tx, ty := b.X+b.W/2, b.Y+b.H-12
	tt.Press(x, y)
	tt.Move(x+10, y+10)
	tt.Frame()
	tt.Move(tx, ty)
	tt.Frame()
	tt.Release(tx, ty)
	tt.Frame()
	tt.Frame()
}
func planStored(t *testing.T, a *App) workspace.Plan {
	t.Helper()
	p, err := a.Store.Plan()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTheOutlineByChapter(t *testing.T) {
	a, tt, f := outlineFixture(t)
	planHas(t, tt, "3 章 · 3 条情节线 · 6 张卡片，2 张未放进章节")
	planHas(t, tt, "旧城的灯一盏一盏查下去")
	planHas(t, tt, "2 章 · 十月五日 — 十月六日")
	planHas(t, tt, "回溯")
	if !a.planBack(f.two) || a.planBack(f.one) {
		t.Fatal("backtracking")
	}
	planClick(t, tt, fmt.Sprintf("添加卡片-%s|%s|-2", f.one, f.lines[2].ID))
	if !tt.Focused("事件") {
		t.Fatal("new card has no caret")
	}
	planHas(t, tt, "新卡片")
	planClick(t, tt, "保存卡片")
	if len(planStored(t, a).Cards) != 6 {
		t.Fatal("empty title saved")
	}
	tt.Type("小祥数到第十三盏")
	planClick(t, tt, "继续添加")
	planHas(t, tt, "已添加「小祥数到第十三盏」，接着记下一张。")
	if !tt.Focused("事件") || a.plan.card.Title != "" {
		t.Fatal("next card has no fresh input")
	}
	tt.Type("他把数字报错了")
	planClick(t, tt, "保存卡片")
	if a.plan.cardOpen {
		t.Fatal("did not close")
	}
	got := planStored(t, a).Cards
	if got[2].Title != "小祥数到第十三盏" || got[3].Title != "他把数字报错了" {
		t.Fatal(got)
	}
	planClick(t, tt, fmt.Sprintf("添加卡片-%s|%s|-2", f.one, f.lines[2].ID))
	tt.Type("不保存")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.plan.cardOpen || len(planStored(t, a).Cards) != 8 {
		t.Fatal("Esc saved")
	}
	planClick(t, tt, "章纲-"+f.one)
	if !tt.Focused("章纲") {
		t.Fatal("outline has no caret")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("何主任要一份新的灯数。")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	ch, err := a.Store.ReadChapter(f.one)
	if err != nil || ch.Outline != "何主任要一份新的灯数。" {
		t.Fatal(ch, err)
	}
	planClick(t, tt, "卷纲-"+f.volume)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("新的卷纲")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	ch, err = a.Store.ReadChapter(f.volume)
	if err != nil || ch.Outline != "新的卷纲" {
		t.Fatal(ch, err)
	}
	planDrag(t, tt, "卡片-"+f.cards[4].ID, fmt.Sprintf("大纲格-%s|%s|-2", f.one, f.lines[1].ID))
	c, _ := planStored(t, a).Card(f.cards[4].ID)
	if c.Chapter != f.one || c.Line != f.lines[1].ID {
		t.Fatal("drop into chapter", c)
	}
	planDrag(t, tt, "卡片-"+c.ID, "大纲托盘")
	c, _ = planStored(t, a).Card(c.ID)
	if c.Chapter != "" || c.Line != f.lines[1].ID {
		t.Fatal("drop into tray", c)
	}
	planClick(t, tt, "打开正文-"+f.one)
	if a.page != 3 || a.write.node != f.one || a.write.outline != "何主任要一份新的灯数。" {
		t.Fatal("open prose", a.page, a.write.node, a.write.outline)
	}
}

func TestOutlinePlotLines(t *testing.T) {
	a, tt, f := outlineFixture(t)
	planClick(t, tt, "新增情节线")
	if !tt.Focused("情节线的名字") {
		t.Fatal("new line not focused")
	}
	tt.Type("钥匙")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	planHas(t, tt, "钥匙")
	l := a.plan.data.Lines[3]
	planClick(t, tt, "情节线「钥匙」的操作")
	planClick(t, tt, "菜单-左移")
	if a.plan.data.Lines[2].ID != l.ID {
		t.Fatal("line did not move")
	}
	planClick(t, tt, "钥匙")
	tt.Type("铜钥匙")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.plan.data.Lines[2].Name != "铜钥匙" {
		t.Fatal(a.plan.data.Lines)
	}
	planClick(t, tt, "情节线「铜钥匙」的操作")
	planClick(t, tt, "菜单-右移")
	planClick(t, tt, "情节线「铜钥匙」的操作")
	planClick(t, tt, "菜单-删除情节线")
	planHas(t, tt, "已删除情节线「铜钥匙」")
	planClick(t, tt, "情节线「母亲」的操作")
	planClick(t, tt, "菜单-删除情节线")
	p := planStored(t, a)
	if len(p.Cards) != len(f.cards) || slices.ContainsFunc(p.Cards, func(c workspace.Card) bool { return c.Line == f.lines[1].ID }) {
		t.Fatal(p)
	}
	planClick(t, tt, "时间线")
	planHas(t, tt, "不分情节线")
	planClick(t, tt, "新增情节线")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	planHas(t, tt, "新情节线")
}

func TestOutlineSearch(t *testing.T) {
	a, tt, f := outlineFixture(t)
	planType(t, tt, "搜索大纲", "何主任")
	planHas(t, tt, "找到 1 处章名或章纲、2 张卡片")
	planType(t, tt, "搜索大纲", "是谁让他锁")
	planHas(t, tt, "找到 1 张卡片")
	planHas(t, tt, "章节里没有「是谁让他锁」")
	if _, ok := tt.Find("大纲托盘"); !ok {
		t.Fatal("tray hidden")
	}
	planType(t, tt, "搜索大纲", "123232")
	planHas(t, tt, "没有找到「123232」")
	if _, ok := tt.Find("大纲托盘"); ok {
		t.Fatal("tray on empty page")
	}
	if _, ok := tt.Find("大纲章节-" + f.one); ok {
		t.Fatal("board on empty page")
	}
	planClick(t, tt, "空页清除搜索")
	if a.plan.query != "" {
		t.Fatal("clear")
	}
	planType(t, tt, "搜索大纲", "123232")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.plan.query != "" {
		t.Fatal("Esc")
	}
	planClick(t, tt, "时间线")
	planType(t, tt, "搜索大纲", "停电")
	planHas(t, tt, "找到 2 张卡片")
	if !a.cardFound(f.cards[0]) || a.cardFound(f.cards[4]) {
		t.Fatal("timeline search")
	}
}

func TestTheOutlineTimeline(t *testing.T) {
	a, tt, f := outlineFixture(t)
	planClick(t, tt, "时间线")
	planHas(t, tt, "「番外：晴天」还没有卡片")
	planClick(t, tt, "卡片-"+f.cards[1].ID)
	planHas(t, tt, "修改卡片")
	planClick(t, tt, "卡片章节")
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	planClick(t, tt, "保存卡片")
	c, _ := planStored(t, a).Card(f.cards[1].ID)
	if c.Chapter != f.three {
		t.Fatal(c)
	}
	planClick(t, tt, "卡片-"+c.ID)
	planType(t, tt, "事件", "不要保存")
	planClick(t, tt, "取消卡片")
	c, _ = planStored(t, a).Card(c.ID)
	if c.Title == "不要保存" {
		t.Fatal("cancel changed card")
	}
	// Reaching an offscreen time is a real horizontal scroll, as in the browser walk.
	a.plan.scroll[1].X = 200
	tt.Frame()
	tt.Frame()
	planDrag(t, tt, "卡片-"+f.cards[4].ID, fmt.Sprintf("大纲格-|%s|2", f.lines[1].ID))
	c, _ = planStored(t, a).Card(f.cards[4].ID)
	if c.When != "十月六日" || c.Line != f.lines[1].ID || c.Chapter != "" {
		t.Fatal("timeline drop", c)
	}
	a.plan.scroll[1].X = 0
	tt.Frame()
	tt.Frame()
	planClick(t, tt, "时间列-1")
	planType(t, tt, "这一列的时间", "十九年前")
	planClick(t, tt, "保存时间")
	if len(a.plan.data.Columns()[0].Cards) != 2 {
		t.Fatal("adjacent times did not merge", a.plan.data.Columns())
	}
	planClick(t, tt, "时间列-1")
	planType(t, tt, "这一列的时间", "十月七日夜")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	n := 0
	for _, e := range a.plan.data.Cards {
		if e.When == "十月七日夜" {
			n++
		}
	}
	if n != 3 {
		t.Fatal("column time changed only part", n, a.plan.data.Cards)
	}
	a.plan.scroll[1].X = 10000
	tt.Frame()
	tt.Frame()
	cols := a.plan.data.Columns()
	last := len(cols) - 1
	planClick(t, tt, fmt.Sprintf("时间列-%d", last))
	planType(t, tt, "这一列的时间", "十二月")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	c, _ = planStored(t, a).Card(f.cards[5].ID)
	if c.When != "十二月" {
		t.Fatal(c)
	}
	planClick(t, tt, fmt.Sprintf("添加卡片-|%s|-1", f.lines[0].ID))
	tt.Type("雨季结束")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	p := planStored(t, a)
	c = p.Cards[len(p.Cards)-1]
	if c.Title != "雨季结束" {
		t.Fatal(p)
	}
	tt.Frame()
	tt.Frame()
	box, ok := tt.Find("卡片-" + c.ID)
	if !ok || box.W <= 0 || box.H <= 0 || box.X < a.plan.viewport.X+planLaneWidth || box.X+box.W > a.plan.viewport.X+a.plan.viewport.W || a.plan.seen != c.ID {
		t.Fatal("new card not revealed and marked", box, a.plan.viewport, a.plan.seen)
	}
	planClick(t, tt, "卡片-"+c.ID)
	planClick(t, tt, "删除卡片")
	if _, ok := planStored(t, a).Card(c.ID); ok {
		t.Fatal("not deleted")
	}
}

func TestTheChapterInTheStory(t *testing.T) {
	a, tt, f := outlineFixture(t)
	planClick(t, tt, "打开正文-"+f.two)
	label := "故事中 十九年前 — 十月六日"
	planHas(t, tt, label)
	planClick(t, tt, label)
	if a.page != pagePlan || a.plan.mode != 1 || a.plan.seen != f.cards[0].ID {
		t.Fatal("did not open at first timed card", a.page, a.plan.mode, a.plan.seen)
	}
	planClick(t, tt, "正文")
	a.openNode(f.three)
	tt.Frame()
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, "故事中 ") {
			t.Fatal("empty chapter has time", text)
		}
	}
}

func TestOutlineStaysAtTheEdges(t *testing.T) {
	a, tt, f := outlineFixture(t)
	before, _ := tt.Find("大纲章节-" + f.one)
	add, _ := tt.Find("新增情节线")
	a.plan.scroll[0].X = 200
	tt.Frame()
	tt.Frame()
	after, _ := tt.Find("大纲章节-" + f.one)
	addAfter, _ := tt.Find("新增情节线")
	if before.X != after.X || add.X != addAfter.X {
		t.Fatal("sticky columns moved", before, after, add, addAfter)
	}
	a.plan.scroll[0].Y = 100
	tt.Frame()
	tt.Frame()
	head, _ := tt.Find("新增情节线")
	if head.Y != add.Y {
		t.Fatal("header scrolled")
	}
	a.plan.scroll[0].Y = 0
	a.plan.scroll[0].X = 0
	tt.Frame()
	a.turnAI(false)
	tt.Frame()
	planHas(t, tt, "大纲")
	if a.page != pagePlan {
		t.Fatal("AI switch removed outline")
	}
}

func TestOutlineTrashAndSettingsPane(t *testing.T) {
	a, tt, f := outlineFixture(t)
	if a.plan.pane {
		t.Fatal("outline opens with pane")
	}
	planClick(t, tt, "展开右栏")
	planHas(t, tt, "设定")
	if _, ok := tt.Find("讨论输入"); ok {
		t.Fatal("outline has discussion pane")
	}
	planClick(t, tt, "收起右栏")
	if a.plan.pane {
		t.Fatal("pane did not close")
	}
	if err := a.Store.TrashChapter(f.two); err != nil {
		t.Fatal(err)
	}
	a.plan.loaded = false
	tt.Frame()
	tt.Frame()
	if len(a.cardsOf("")) != 5 {
		t.Fatal("trashed cards absent from tray")
	}
	if err := a.Store.RestoreChapter(f.two); err != nil {
		t.Fatal(err)
	}
	a.plan.loaded = false
	tt.Frame()
	tt.Frame()
	if len(a.cardsOf(f.two)) != 3 {
		t.Fatal("restored cards not back")
	}
	planClick(t, tt, "正文")
	planClick(t, tt, "大纲")
	if a.plan.pane {
		t.Fatal("pane state changed between pages")
	}
}
