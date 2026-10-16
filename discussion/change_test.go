package discussion

import (
	"slices"
	"strings"
	"testing"

	"writer/workspace"
)

// A talk tells the model how the setting changes from chapters on, under
// each entry by its chapter's name, and has in its index each entry as it
// reads now; the model notes a change on the author's words, or as an idea
// that waits, and one from where one holds already takes its words.
func TestATalkTellsAndNotesHowTheSettingChanges(t *testing.T) {
	e, session := setup(t)
	node := func(parent, title string, folder bool) string {
		id, err := e.Store.NewChapter(parent, title, folder)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	node("", "第一章", false)
	vol := node("", "第二卷", true)
	three := node(vol, "第三章：停电", false)
	revise(t, e, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Names: []string{"林晚"}, Text: "林晚用左手拧灯座。"})
	turn(t, e, session, "从第三章起她没了左手。", "那她以后要用右手干活了。")
	ok, err := e.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "对，用右手。"})
	if err != nil {
		t.Fatal(err)
	}
	box := &toolbox{store: e.Store, session: session, turn: ok.ID, model: Model{Name: "deepseek/deepseek-flash"}}
	for _, c := range []struct {
		in   input
		want string // in the refusal; empty when it is noted
	}{
		{input{Op: "change", Entry: 1, Chapter: "第三章", Place: "body", Text: "林晚没了左手，用右手干活。", Sources: []string{"从第三章起她没了左手"}}, ""},
		{input{Op: "change", Entry: 1, Chapter: "第九章", Place: "open", Text: "x", Sources: []string{"那她以后要用右手干活了"}}, "没有叫「第九章」的章"},
		{input{Op: "change", Entry: 1, Place: "open", Text: "x", Sources: []string{"那她以后要用右手干活了"}}, "change 要写从哪一章起"},
		{input{Op: "change", Entry: 1, Chapter: "第二卷", Place: "body", Text: "x", Sources: []string{"那她以后要用右手干活了"}}, "author_said"},
		{input{Op: "change", Entry: 1, Chapter: "第二卷", Place: "dropped", Text: "x", Sources: []string{"那她以后要用右手干活了"}}, "place（body 或 open）"},
		{input{Op: "change", Entry: 1, Chapter: "第二卷", Place: "open", Text: "x", Names: []string{"林晚"}, Sources: []string{"那她以后要用右手干活了"}}, "change 不带 names"},
		{input{Op: "change", Entry: 1, Chapter: "第二卷", Place: "open", Text: "林晚学会了用右手修灯。", Sources: []string{"那她以后要用右手干活了"}}, ""},
		{input{Op: "change", Entry: 1, Chapter: "第三章", Place: "body", Text: "林晚没了左手。", Sources: []string{"对，用右手"}}, ""},
	} {
		out := box.note(c.in)
		if c.want == "" && out.failed || c.want != "" && (!out.failed || !strings.Contains(out.text, c.want)) {
			t.Errorf("%+v: %s", c.in, out.text)
		}
		if c.want == "" && !strings.Contains(out.text, "起：") {
			t.Errorf("noted as %s", out.text)
		}
	}
	set := setting(t, e)
	g := set.Entries[0].Changes
	if len(g) != 2 || g[0].From != three || g[0].Place != workspace.PlaceBody || g[0].Text != "林晚没了左手。" || len(g[0].Sources) != 2 || g[0].Author ||
		g[1].From != vol || g[1].Place != workspace.PlaceOpen {
		t.Fatalf("%+v", g)
	}

	tree, err := e.Store.Chapters()
	if err != nil {
		t.Fatal(err)
	}
	w := e.Store.Work()
	got, err := Assemble(w, 100000, set, tree, nil, "问", Along{})
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{"「第三章：停电」起：林晚没了左手。", "「第二卷」起（悬而未决）：林晚学会了用右手修灯。"}
	if !strings.Contains(got.Request.System, "[1] 林晚：林晚用左手拧灯座。\n  "+strings.Join(lines, "\n  ")) || !slices.Equal(got.Included[0].Changes, lines) || got.Included[0].Text != "林晚：林晚用左手拧灯座。" {
		t.Fatalf("%s\n%+v", got.Request.System, got.Included)
	}
	if index := talkIndex(w, set, viewOf(set, tree, nil)); !strings.HasSuffix(index, "[1] 人物 · 林晚：林晚没了左手") {
		t.Fatal(index)
	}
	for _, q := range []string{"林晚", "右手修灯"} {
		if out := box.lookUp(input{Query: q}); !strings.Contains(out.text, "[1]") || !strings.Contains(out.text, "\n  「第三章：停电」起：林晚没了左手。") {
			t.Fatalf("%s: %s", q, out.text)
		}
	}

	// A chapter in the trash takes its change with it.
	if err = e.Store.TrashChapter(three); err != nil {
		t.Fatal(err)
	}
	if tree, err = e.Store.Chapters(); err != nil {
		t.Fatal(err)
	}
	if got, err = Assemble(w, 100000, set, tree, nil, "问", Along{}); err != nil || strings.Contains(got.Request.System, "第三章") {
		t.Fatal(got.Request.System, err)
	}
}
