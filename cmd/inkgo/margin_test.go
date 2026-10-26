package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

func TestLayoutMargin(t *testing.T) {
	root := t.TempDir()
	s := layoutBook(t, root)
	node, err := s.NewChapter("", "第87章：灯灭以后", false)
	if err != nil {
		t.Fatal(err)
	}
	text := "整条街的灯是同时灭的。\n林晚站在巷口，手电在工具箱的最底下。\n她没有去摸，一个人走进了没有灯的巷子。\n雨水顺着工具箱的缝隙往下淌。\n井边那把旧钥匙还在，铜绿已经爬满了齿缝。\n她停了一会儿，才把钥匙拾起来。"
	at := utf8.RuneCountInString(text[:strings.Index(text, "旧钥匙")])
	if _, err = s.WriteChapter(node, text, []workspace.ChapterMark{{ID: "key", Kind: workspace.MarkForeshadow, Start: at, End: at + 3, Note: "是谁留下了井边的钥匙"}}); err != nil {
		t.Fatal(err)
	}
	r, err := s.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "林晚很怕黑，宁可绕路也不走没有灯的巷子。", Names: []string{"林晚"}}}})
	if err != nil {
		t.Fatal(err)
	}
	entry := r.Ops[0].Entry
	session, err := s.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "这一章起，她敢独自走进黑巷了"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Revise(workspace.Revision{By: workspace.ByTalk, Session: session, Turn: u.ID, Ops: []workspace.Op{{Op: workspace.OpChange, Entry: entry, From: node, Place: workspace.PlaceOpen, Text: "林晚开始直面怕黑的恐惧，敢独自走进没有灯的巷子。"}}}); err != nil {
		t.Fatal(err)
	}
	if err = s.WriteCheck(node, workspace.Check{Of: workspace.ProseHash(text), At: time.Now(), Findings: []workspace.Finding{{ID: "dark", Kind: workspace.FindConflict, Entry: entry, Quote: "一个人走进了没有灯的巷子", Title: "林晚怕黑，却独自走进黑巷", Says: "林晚很怕黑"}}}); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	a := openLayoutApp(t, root)
	tt := ui.NewTester(a.View, 1266, 780)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	if !tt.HasText("设定待确认") || !tt.HasText("与设定不一致") || !tt.HasText("伏笔 · 在此埋下") {
		t.Fatal("all three margin sources must appear in the full window", tt.Texts())
	}
	shot(t, tt, "-margin")
}
