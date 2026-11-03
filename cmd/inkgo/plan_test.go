package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/ai"
	"writer/desk"
	"writer/workspace"
)

func TestLayoutOutline(t *testing.T) {
	root := t.TempDir()
	s, err := workspace.Create(filepath.Join(root, "work"), "雨城来信", "网文")
	if err != nil {
		t.Fatal(err)
	}
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var chapters []string
	for _, vol := range []struct {
		title, outline string
		chapters       []string
	}{
		{"第一卷：雨季", "", []string{"第1章：巷口", "第2章：井沿"}},
		{"第二卷：回声", "", []string{"第46章：两把钥匙"}},
		{"第三卷：停电", "旧城的灯一盏一盏查下去，查到二十六年前那次停电，和林晚母亲没回来的那一晚。", []string{"第84章：灯罩", "第85章：灯数", "第86章：档案", "第87章：停电"}},
	} {
		id, e := s.NewChapter("", vol.title, true)
		check(e)
		check(s.WriteOutline(id, vol.outline))
		for _, title := range vol.chapters {
			c, e := s.NewChapter(id, title, false)
			check(e)
			chapters = append(chapters, c)
		}
	}
	c, e := s.NewChapter("", "番外：晴天", false)
	check(e)
	chapters = append(chapters, c)
	var lines []workspace.PlotLine
	for _, name := range []string{"查灯", "母亲的下落", "小祥"} {
		l, e := s.SaveLine(workspace.PlotLine{Name: name})
		check(e)
		lines = append(lines, l)
	}
	var cards []workspace.Card
	for _, x := range []struct {
		title, when, about string
		ch, line           int
	}{
		{"母亲在停电那晚失踪", "二十六年前·秋", "全城停电，母亲去井边打水，让七岁的林晚别动、等灯亮。", 6, 1},
		{"周祥来当学徒", "今年·四月", "林晚带他修巷口的灯，他总把左手揣在兜里。", 0, 2},
		{"井沿上多了一道新刻痕", "今年·四月", "", 1, 1},
		{"两把钥匙都打不开小学侧门", "今年·七月", "", 2, 0},
		{"灯罩里刻着“晚”字", "十月三日", "", 3, 0},
		{"何主任让重新数灯", "十月五日", "", 4, 0},
		{"档案室里的停电通知", "十月六日", "二十六年前的通知。", 5, 1},
		{"全城停电，林晚走进巷子", "十月六日夜", "", 6, 0},
		{"小祥改口叫她“姐”", "十月六日夜", "", 6, 2},
		{"林晚下井", "十一月", "掀开井口的石板。", -1, 1},
		{"何主任为什么锁着档案柜", "", "还没想好是谁让他锁的。", -1, -1},
	} {
		chapter, line, after := "", "", ""
		if x.ch >= 0 {
			chapter = chapters[x.ch]
		}
		if x.line >= 0 {
			line = lines[x.line].ID
		}
		if len(cards) > 0 {
			after = cards[len(cards)-1].ID
		}
		c, e := s.SaveCard(workspace.Card{Title: x.title, When: x.when, About: x.about, Chapter: chapter, Line: line}, after)
		check(e)
		cards = append(cards, c)
	}
	check(s.WriteOutline(chapters[5], "林晚趁何主任不在翻档案柜，只找到一张旧的停电通知。"))
	check(s.Close())
	a := desk.New(ai.New(filepath.Join(root, "providers.yaml")), modules...)
	check(a.Open(filepath.Join(root, "work")))
	defer a.Close()
	tt := ui.NewTester(a.View, 1266, 780)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	click := func(label string) { t.Helper(); check(tt.Click(label)); tt.Frame(); tt.Frame() }
	click("大纲")
	if !tt.HasText("8 章 · 3 条情节线 · 11 张卡片，2 张未放进章节") {
		t.Fatal(tt.Texts())
	}
	shot(t, tt, "-outline-chapters")
	click(fmt.Sprintf("添加卡片-%s|%s|-2", chapters[0], lines[0].ID))
	tt.Type("小祥数到第十三盏")
	shot(t, tt, "-outline-card")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	click("时间线")
	for _, label := range []string{"搜索大纲", "时间线", "展开右栏"} {
		r, ok := tt.Find(label)
		if !ok || r.W <= 0 || r.X+r.W > 1266 {
			t.Fatalf("toolbar outside window: %s %+v", label, r)
		}
	}
	shot(t, tt, "-outline-timeline")
	click("搜索大纲")
	tt.Type("123232")
	tt.Frame()
	if !tt.HasText("没有找到「123232」") {
		t.Fatal(tt.Texts())
	}
	shot(t, tt, "-outline-none")
	// An auxiliary file error must leave the work open and occupy the page.
	check(os.WriteFile(filepath.Join(root, "work", "outline.json"), []byte("{"), 0600))
	click("正文")
	click("大纲")
	if !tt.HasText("大纲文件读不出来") || !tt.HasText("文件在结尾处断了") {
		t.Fatal(tt.Texts())
	}
	shot(t, tt, "-outline-error")
}
