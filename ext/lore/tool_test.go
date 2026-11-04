package lore

import (
	"fmt"
	"strings"
	"testing"
	"writer/ext/ai/aiapi"
	"writer/talkbook"
	"writer/workspace"
)

func TestLookupResolvesOnlyDisplayedSources(t *testing.T) {
	var set workspace.Setting
	for i := range maxHits + 2 {
		e := workspace.Entry{ID: fmt.Sprint(i), Text: "林晚怕黑", Place: workspace.PlaceBody, Category: "人物"}
		for j := range 4 {
			e.Sources = append(e.Sources, workspace.Source{Session: fmt.Sprintf("s%d-%d", i, j), Message: "message", Quote: "林晚"})
		}
		if i == maxHits+1 {
			e.Names = []string{"林晚"}
		}
		set.Entries = append(set.Entries, e)
	}
	var resolved []string
	out := lookupSetting("林晚", set, talkbook.Of(workspace.Tree{}), "", func(session string) string {
		resolved = append(resolved, session)
		return session
	})
	if out.Failed || !strings.HasPrefix(out.Text, fmt.Sprintf("[%d]", maxHits+2)) || !strings.Contains(out.Text, "还有 2 条没有列出") {
		t.Fatalf("lookup = %+v", out)
	}
	if len(resolved) != maxHits*3 {
		t.Fatalf("resolved %d sources, want %d", len(resolved), maxHits*3)
	}
	for _, session := range resolved {
		if strings.HasSuffix(session, "-3") || strings.HasPrefix(session, fmt.Sprintf("s%d-", maxHits-1)) || strings.HasPrefix(session, fmt.Sprintf("s%d-", maxHits)) {
			t.Fatalf("read undisplayed source %s", session)
		}
	}
}

type toolBook struct{ Store *workspace.Store }

func toolFixture(t *testing.T) (*toolBook, string) {
	t.Helper()
	s, err := workspace.Create(t.TempDir(), "雨夜", "网文", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	session, err := s.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	return &toolBook{Store: s}, session
}
func turn(t *testing.T, e *toolBook, session, user, reply string) (workspace.Event, workspace.Event) {
	t.Helper()
	u, err := e.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: user})
	if err != nil {
		t.Fatal(err)
	}
	a, err := e.Store.Append(session, workspace.Event{Role: workspace.RoleAssistant, Text: reply, ReplyTo: u.ID})
	if err != nil {
		t.Fatal(err)
	}
	return u, a
}
func revise(t *testing.T, e *toolBook, ops ...workspace.Op) workspace.Revision {
	t.Helper()
	r, err := e.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func setting(t *testing.T, e *toolBook) workspace.Setting {
	t.Helper()
	set, err := e.Store.Setting()
	if err != nil {
		t.Fatal(err)
	}
	return set
}
func TestNoteKeepsTheRules(t *testing.T) {
	e, session := toolFixture(t)
	u, _ := turn(t, e, session, "主角叫林晚，她怕黑。", "也许她小时候在矿井里迷过路？")
	ok, err := e.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "矿井这个好，就这样。另外雨不会停。"})
	if err != nil {
		t.Fatal(err)
	}
	revise(t, e,
		workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Names: []string{"林晚"}, Text: "林晚是主角"},
		workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "规则", Text: "雨每年停一天"})
	box := &toolbox{aiapi.ToolEnv{Store: e.Store, Session: session, Turn: ok.ID, Model: "deepseek/deepseek-flash", Shared: aiapi.NewToolShared(e.Store)}}
	for _, c := range []struct {
		in   noteArgs
		want string // in the refusal; empty when it is noted
	}{
		{noteArgs{Op: "add", Place: "body", Category: "人物", Text: "她小时候在矿井里迷过路", Sources: []string{"她小时候在矿井里迷过路"}}, "author_said"},
		{noteArgs{Op: "add", Place: "body", Category: "人物", Text: "她小时候在矿井里迷过路", Sources: []string{"她小时候在矿井里迷过路"}, AuthorSaid: "也许她小时候"}, "作者在这次讨论里说过的话"},
		{noteArgs{Op: "add", Place: "open", Category: "人物", Text: "她怕水", Sources: []string{"她怕水"}}, "不是这次讨论里已经说过的话里逐字的原文"},
		{noteArgs{Op: "add", Place: "open", Category: "魔法", Text: "雨不会停", Sources: []string{"雨不会停"}}, "没有「魔法」这个分类"},
		{noteArgs{Op: "add", Place: "open", Category: "人物", Text: "主角怕黑", Names: []string{"晚晚"}, Sources: []string{"她怕黑"}}, "名字「晚晚」没有在条目或对话里出现过"},
		{noteArgs{Op: "add", Place: "open", Category: "人物", Text: "林晚怕黑", Names: []string{"林晚"}, Sources: []string{"她怕黑"}}, "已经在编号 1 的条目里"},
		{noteArgs{Op: "add", Place: "open", Category: "规则", Text: "雨不会停", Names: []string{"雨"}, Sources: []string{"雨不会停"}}, "只有人物、"},
		{noteArgs{Op: "edit", Entry: 1, Text: "林晚是女主角"}, "author_said"},
		{noteArgs{Op: "edit", Entry: 9, Text: "林晚是女主角"}, "没有编号 9"},
		{noteArgs{Op: "edit", Entry: 1, Category: "人物", Text: "林晚是女主角", AuthorSaid: "矿井这个好，就这样"}, "edit 要有改好的 text，不带 place、category（换位置或分类用 move）。"},
		{noteArgs{Op: "name", Entry: 1, Names: []string{"晚"}}, "原有名字 林晚 要原样排在前面"},
		{noteArgs{Op: "move", Entry: 2, Place: "dropped"}, "author_said"},
		{noteArgs{Op: "add", Place: "body", Category: "人物", Text: "她小时候在矿井里迷过路", Sources: []string{"她小时候在矿井里迷过路"}, AuthorSaid: "矿井这个好，就这样"}, ""},
		{noteArgs{Op: "move", Entry: 2, Place: "dropped", AuthorSaid: "雨不会停"}, ""},
		{noteArgs{Op: "add", Place: "body", Category: "规则", Text: "雨不会停", Sources: []string{"雨不会停"}}, ""},
		{noteArgs{Op: "name", Entry: 1, Names: []string{"林晚", "主角"}}, ""},
		{noteArgs{Op: "add", Place: "dropped", Category: "规则", Text: "雨会停", Sources: []string{"雨不会停"}, AuthorSaid: "雨不会停"}, ""},
	} {
		out := box.note(c.in)
		if c.want == "" && out.Failed || c.want != "" && (!out.Failed || !strings.Contains(out.Text, c.want)) {
			t.Errorf("%+v: %s", c.in, out.Text)
		}
	}
	set := setting(t, e)
	if len(set.Entries) != 5 || set.Entries[1].Place != workspace.PlaceDropped || set.Entries[0].Names[1] != "主角" || len(set.Entries[4].Sources) != 1 {
		t.Fatalf("%+v", set.Entries)
	}
	mine := set.Entries[2].Sources
	if len(mine) != 2 || mine[0].Message == u.ID || mine[1].Message != ok.ID || mine[1].Quote != "矿井这个好，就这样" {
		t.Fatalf("sources %+v", mine)
	}
}

func TestNoteEditLabelNamesTheOriginalSetting(t *testing.T) {
	e, _ := toolFixture(t)
	revise(t, e, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "旧设定"})
	r := &labelReader{aiapi.ToolEnv{Store: e.Store, Shared: aiapi.NewToolShared(e.Store)}}
	if got := noteLabel(r, noteArgs{Op: workspace.OpEdit, Entry: 1, Text: "新设定"}); got != "记下：改写「旧设定」为：新设定" {
		t.Fatal(got)
	}
}
