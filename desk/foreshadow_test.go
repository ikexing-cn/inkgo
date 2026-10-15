package desk

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"writer/workspace"
)

// The author buries a foreshadowing with a note, pays it off in a later
// chapter, takes that back, gives it up and takes it up again.
func TestForeshadowBuryAndPay(t *testing.T) {
	a, tt, one := writingFixture(t, "她修了七年路灯，却不敢走进没有灯的巷子。\n小祥在身后叫她。")
	w := &a.write
	w.ed.Select(0, 20)
	a.openBury()
	tt.Frame()
	if !tt.HasText("记为伏笔，填写备注") {
		t.Fatal("no note asked for", tt.Texts())
	}
	w.fore.note = "她怕黑的真正原因"
	if err := tt.Click("确定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	ch, err := a.Store.ReadChapter(one)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Marks) != 1 || ch.Marks[0].Kind != workspace.MarkForeshadow || ch.Marks[0].Note != "她怕黑的真正原因" ||
		ch.Marks[0].ID == "" || ch.Marks[0].At == "" || ch.Marks[0].Quote != "她修了七年路灯，却不敢走进没有灯的巷子。" {
		t.Fatalf("buried %+v", ch.Marks)
	}
	id := ch.Marks[0].ID

	two, err := a.Store.NewChapter("", "停电", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(two, "七岁那年的那个晚上，她也是这样站着。", nil); err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.openNode(two)
	tt.Frame()
	if f, ok := a.foreshadowOf(id); !ok || f.status() != foreOpen || f.gap != 1 || f.here {
		t.Fatalf("from the next chapter %+v", f)
	}
	a.openPay("") // nothing selected: nothing to pay off with
	if w.fore.paying || a.hint == "" {
		t.Fatal("paying off without a passage")
	}
	a.hint = "" // the hint floats over the top of the column
	w.ed.Select(0, 18)
	a.openPay("")
	tt.Frame()
	if !tt.HasText("选择要回收的伏笔") || !tt.HasText("第一章 · 1 章前埋下") || w.fore.pick != id {
		t.Fatal("nothing to pick", w.fore.pick, tt.Texts())
	}
	if err = tt.Click("确定回收"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	f, _ := a.foreshadowOf(id)
	if f.status() != foreDone || len(f.pays) != 1 || f.pays[0].node != two || f.pays[0].quote != "七岁那年的那个晚上，她也是这样站着。" || f.gap != 1 {
		t.Fatalf("paid off %+v", f)
	}
	if !a.doc.fore || !tt.HasText("取消回收") || !tt.HasText("回收在「停电」，相隔 1 章") || !tt.HasText("埋在「第一章」") {
		t.Fatal("the two are not shown together", tt.Texts())
	}
	if ch, _ = a.Store.ReadChapter(two); len(ch.Marks) != 1 || ch.Marks[0].Kind != workspace.MarkPayoff || ch.Marks[0].Of != id {
		t.Fatalf("payoff on disk %+v", ch.Marks)
	}

	// Its note is changed from another chapter: on the disk.
	a.renoteFore(f, "怕黑")
	if ch, _ = a.Store.ReadChapter(one); ch.Marks[0].Note != "怕黑" || ch.Marks[0].ID != id {
		t.Fatalf("renoted %+v", ch.Marks)
	}
	tt.Frame()
	if err = tt.Click("查看原文"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if s, e := w.ed.Selection(); w.node != one || s != 0 || e != 20 {
		t.Fatalf("opened %s at %d-%d", w.node, s, e)
	}

	if err = tt.Click("取消回收"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if f, _ = a.foreshadowOf(id); f.status() != foreOpen {
		t.Fatalf("still paid off %+v", f)
	}
	if ch, _ = a.Store.ReadChapter(two); len(ch.Marks) != 0 {
		t.Fatalf("the payoff stayed %+v", ch.Marks)
	}
	if err = tt.Click("放弃伏笔"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if f, _ = a.foreshadowOf(id); f.status() != foreDropped || !tt.HasText("已放弃") {
		t.Fatalf("not dropped %+v", f)
	}
	if err = tt.Click("恢复伏笔"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if f, _ = a.foreshadowOf(id); f.status() != foreOpen {
		t.Fatalf("not taken up again %+v", f)
	}

	// The list: one open, searched by its note.
	if err = tt.Click("全部伏笔"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !tt.HasText("未回收") || !tt.HasText("怕黑") {
		t.Fatal("the list", tt.Texts())
	}
	w.fore.query = "钥匙"
	tt.Frame()
	if !tt.HasText("未找到") {
		t.Fatal("found what is not there", tt.Texts())
	}
}

// The picker lists what the chapter or those before it buried, counting
// back from the chapter: what a later chapter buries is not there yet.
func TestThePickerListsWhatCameBefore(t *testing.T) {
	a, tt, one := writingFixture(t, "她修了七年路灯，却不敢走进没有灯的巷子。")
	w := &a.write
	two, err := a.Store.NewChapter("", "停电", false)
	if err != nil {
		t.Fatal(err)
	}
	later := []workspace.ChapterMark{{Kind: workspace.MarkForeshadow, Start: 0, End: 8, Quote: "井边有一把钥匙。", Note: "钥匙开哪扇门", ID: "key"}}
	if _, err = a.Store.WriteChapter(two, "井边有一把钥匙。", later); err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.openNode(one)
	tt.Frame()
	w.ed.Select(0, 8)
	a.openPay("")
	tt.Frame()
	if !tt.HasText("选择要回收的伏笔") || tt.HasText("钥匙开哪扇门") || w.fore.pick != "" {
		t.Fatal("a later chapter's is listed", tt.Texts())
	}
	w.fore.paying = false
	a.openNode(two)
	tt.Frame()
	w.ed.Select(0, 8)
	a.openPay("")
	tt.Frame()
	if !tt.HasText("钥匙开哪扇门") || !tt.HasText("本章埋下") || w.fore.pick != "key" {
		t.Fatal("the chapter's own is not listed", tt.Texts())
	}
}

// A foreshadowing whose sentence was deleted waits on the author, who puts
// it on another sentence or gives it up.
func TestForeshadowLost(t *testing.T) {
	a, tt, one := writingFixture(t, "井边有一把钥匙。\n她没有去捡。")
	w := &a.write
	w.ed.Select(0, 8)
	a.openBury()
	w.fore.note = "钥匙"
	a.bury()
	id := w.ed.Doc.Marks()[0].Key
	w.ed.Select(0, 8)
	w.ed.Insert("")
	tt.Frame()
	f, _ := a.foreshadowOf(id)
	if f.status() != foreLost || f.quote != "井边有一把钥匙。" {
		t.Fatalf("not lost %+v", f)
	}
	a.doc.fore = true
	tt.Frame()
	if !tt.HasText("原句已删") {
		t.Fatal("does not wait on the author", tt.Texts())
	}
	if err := tt.Click("原句已删"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.hint = ""
	a.rehang(f)
	if a.hint == "" {
		t.Fatal("hung on nothing")
	}
	text := w.ed.Doc.String()
	at := len([]rune(text[:strings.Index(text, "她没有去捡。")]))
	w.ed.Select(at, at+6)
	if err := tt.Click("更换原句"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if f, _ = a.foreshadowOf(id); f.status() != foreOpen || f.quote != "她没有去捡。" || f.note != "钥匙" {
		t.Fatalf("not hung again %+v", f)
	}
	ch, err := a.Store.ReadChapter(one)
	if err != nil || len(ch.Marks) != 1 || ch.Marks[0].Orphaned || ch.Marks[0].Quote != "她没有去捡。" {
		t.Fatalf("on disk %+v %v", ch.Marks, err)
	}
}

// A foreshadowing saved before they had IDs gets one that stays the same,
// so it can be paid off.
func TestForeshadowWithoutID(t *testing.T) {
	a, tt, one := writingFixture(t, "井边有一把钥匙。")
	if _, err := a.Store.WriteChapter(one, "井边有一把钥匙。", []workspace.ChapterMark{{Kind: workspace.MarkForeshadow, Start: 5, End: 7}}); err != nil {
		t.Fatal(err)
	}
	a.write.node = ""
	a.openNode(one)
	tt.Frame()
	fs := a.foreshadows()
	if len(fs) != 1 || fs[0].id == "" {
		t.Fatalf("%+v", fs)
	}
	m, err := a.Store.Marks()
	if err != nil || m[one][0].ID != fs[0].id {
		t.Fatalf("another id on the disk: %+v %v", m, err)
	}
	a.write.ed.Select(0, 4)
	a.openPay(fs[0].id)
	a.pay(fs[0].id)
	if f, _ := a.foreshadowOf(fs[0].id); f.status() != foreDone {
		t.Fatalf("not paid off %+v", f)
	}
}

// foreshadowOf is the foreshadowing of id as the page has it.
func (a *App) foreshadowOf(id string) (foreshadow, bool) {
	for _, f := range a.foreshadows() {
		if f.id == id {
			return f, true
		}
	}
	return foreshadow{}, false
}

func TestShotForeshadow(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	a, tt, one := writingFixture(t, "雨从傍晚开始下，一直没有停。\n林晚背着工具箱，沿着亮着的灯往旧城走。她修了七年路灯，却从来不敢在天黑以后走进没有灯的巷子。\n巷口那盏灯的灯罩上，有人用指甲刻了一个“晚”字。")
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
	w := &a.write
	find := func(q string) (int, int) {
		text := w.ed.Doc.String()
		at := len([]rune(text[:strings.Index(text, q)]))
		return at, at + len([]rune(q))
	}
	a.doc.fore = true
	shot("fore-empty.png")
	s, e := find("她修了七年路灯，却从来不敢在天黑以后走进没有灯的巷子。")
	w.ed.Select(s, e)
	a.openBury()
	w.fore.note = "她怕黑的真正原因"
	shot("fore-bury.png")
	a.bury()
	s, e = find("巷口那盏灯的灯罩上，有人用指甲刻了一个“晚”字。")
	w.ed.Select(s, e)
	a.openBury()
	w.fore.note = "灯罩上的“晚”字是谁刻的"
	a.bury()
	if _, err := a.Store.WriteChapter(one, w.ed.Doc.String(), append(storedMarks(w.ed.Doc.Marks()),
		workspace.ChapterMark{Kind: workspace.MarkForeshadow, ID: "lost", Orphaned: true, Quote: "周祥把左手揣在兜里，从来不拿出来。", Note: "小祥的左手"})); err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= 40; i++ {
		if _, err := a.Store.NewChapter("", fmt.Sprintf("第%d章", i), false); err != nil {
			t.Fatal(err)
		}
	}
	two, err := a.Store.NewChapter("", "停电", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Store.WriteChapter(two, "整条街的灯是同时灭的。\n林晚站在巷口，手电在工具箱最底下。她没有去摸。\n七岁那年的那个晚上，她也是这样站着。母亲说去井边打水，让她别动，等灯亮。灯一直没有亮。\n小祥在身后叫她：“姐，你别进去。”",
		[]workspace.ChapterMark{{Kind: workspace.MarkForeshadow, ID: "sis", Start: 82, End: 98, Note: "小祥为什么突然改口叫她“姐”"}}); err != nil {
		t.Fatal(err)
	}
	if err = a.loadTree(); err != nil {
		t.Fatal(err)
	}
	a.write.node = ""
	a.openNode(two)
	shot("fore-list.png")
	s, e = find("七岁那年的那个晚上，她也是这样站着。母亲说去井边打水，让她别动，等灯亮。灯一直没有亮。")
	w.ed.Select(s, e)
	a.openPay("")
	shot("fore-pay.png")
	a.pay(w.fore.pick)
	shot("fore-detail.png")
	tt.SetDark(true)
	shot("fore-detail-dark.png")
	tt.SetDark(false)
	a.showFore("lost")
	shot("fore-lost.png")
	a.openPrefs("提醒与检查")
	shot("fore-settings.png")
}
