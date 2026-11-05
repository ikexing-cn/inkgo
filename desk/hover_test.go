package desk

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/workspace"
)

func pixelsChanged(a, b image.Image, r ui.Rect) bool {
	for y := int(r.Y) + 2; y < int(r.Y+r.H)-2; y++ {
		for x := int(r.X) + 2; x < int(r.X+r.W)-2; x++ {
			if a.At(x, y) != b.At(x, y) {
				return true
			}
		}
	}
	return false
}

func TestCategoryActionsFollowHoverAndKeyboard(t *testing.T) {
	a, tt, _ := fixture(t)
	a.page = pageCategories
	tt.Move(0, 0)
	tt.Frame()
	tt.Frame()
	edit, row := find(t, tt, "编辑-人物"), find(t, tt, "分类行-人物")
	base := tt.Image()
	tt.Move(row.X+20, row.Y+row.H/2)
	tt.Frame()
	if !pixelsChanged(base, tt.Image(), edit) {
		t.Fatal("category actions did not appear on hover")
	}
	tt.Move(0, 0)
	tt.Frame()
	if pixelsChanged(base, tt.Image(), edit) {
		t.Fatal("category action stayed visible")
	}
	for range 50 {
		tt.Key(0, ui.KeyTab)
		if tt.Focused("编辑-人物") {
			break
		}
	}
	tt.Frame()
	if !tt.Focused("编辑-人物") || !pixelsChanged(base, tt.Image(), edit) {
		t.Fatal("keyboard did not reveal the row actions")
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if catsOf(a).Name != "人物" {
		t.Fatal("keyboard edit did not open category")
	}
	other := find(t, tt, "分类行-其他")
	tt.Move(other.X+10, other.Y+other.H/2)
	click(t, tt, "删除-其他")
	if catsOf(a).Remove == "其他" {
		t.Fatal("其他 could be deleted")
	}
}

func TestTheLastReplyToolsOnlyAppearOnHover(t *testing.T) {
	a, tt, _ := fixture(t)
	u, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleUser, Text: "雨城"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := a.Store.Append(talkOf(a).Session, workspace.Event{Role: workspace.RoleAssistant, ReplyTo: u.ID, At: time.Now(), Model: strings.Repeat("long-model-name-", 20), Text: "雨一直没有停。"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.refresh(); err != nil {
		t.Fatal(err)
	}
	tt.SetSize(1100, 780)
	tt.Move(0, 0)
	tt.Frame()
	tt.Frame()
	copyBox := find(t, tt, "复制-"+m.ID)
	base := tt.Image()
	words := find(t, tt, "雨一直没有停。")
	tt.Move(words.X+5, words.Y+5)
	tt.Frame()
	if !pixelsChanged(base, tt.Image(), copyBox) {
		t.Fatal("last reply tools were not hidden until hover")
	}
	for _, label := range []string{"摘取-" + m.ID, "复制-" + m.ID, "上下文-" + m.ID} {
		r := find(t, tt, label)
		if r.X+r.W > talkOf(a).LastChat.X+talkOf(a).LastChat.W-24+1 || r.Y != copyBox.Y {
			t.Fatal("reply buttons wrapped or escaped", label, r, copyBox)
		}
	}
	tt.Move(0, 0)
	tt.Frame()
	if pixelsChanged(base, tt.Image(), copyBox) {
		t.Fatal("last reply tools stayed visible")
	}
}

func TestForeshadowTabCountsLostAndLate(t *testing.T) {
	a, tt, one := writingFixture(t, "钥匙在井边。")
	if _, err := a.Store.WriteChapter(one, "钥匙在井边。", []workspace.ChapterMark{
		{Kind: workspace.MarkForeshadow, Start: 0, End: 2},
		{Kind: workspace.MarkForeshadow, Orphaned: true, Quote: "旧信"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"第二章", "第三章"} {
		if _, err := a.book.NewChapter("", name, false); err != nil {
			t.Fatal(err)
		}
	}
	forgetChapter(a)
	a.openNode(one)
	hooksOf(a).SetLateAfter(2)
	tt.Frame()
	group := a.pendingGroups()
	if a.paneCounts().waiting != 2 || len(group) != 1 || len(group[0].items) != 2 {
		t.Fatal("lost and late counts disagree", a.paneCounts(), group)
	}
	hooksOf(a).SetLateAfter(3)
	tt.Frame()
	if a.paneCounts().waiting != 1 {
		t.Fatal("tab ignored the changed threshold")
	}
}

func TestCheckSidesHaveTheSameHeight(t *testing.T) {
	a, tt, _, _ := checkFixture(t)
	click(t, tt, "立即检查")
	until(t, tt, "check", func() bool { return len(a.checkItems(false)) == 3 })
	a.openCheck(a.checkItems(false)[0].f.ID)
	tt.Frame()
	tt.Frame()
	var cards []ui.Rect
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, "检查对照-") {
			cards = append(cards, find(t, tt, text))
		}
	}
	if len(cards) != 2 || cards[0].Y != cards[1].Y || cards[0].H != cards[1].H {
		t.Fatal("comparison cards have different heights", cards)
	}
	if tt.HasText("检查") && find(t, tt, "检查").H > kit.HBar {
		t.Fatal("check page still has its own large title")
	}
}
