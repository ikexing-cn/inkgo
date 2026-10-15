package discussion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"writer/model"
	"writer/workspace"
)

// call is a tool call of the model's, its input given as a map.
func call(id, name string, in map[string]any) model.Call {
	b, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}
	return model.Call{ID: id, Name: name, Input: b}
}

// A reply looks things up and notes the setting before it answers: each
// step's calls are written before they run and each result after, the
// next step sees them, and the reply records what all steps used.
func TestReplyLooksUpAndNotes(t *testing.T) {
	e, session := setup(t)
	id, err := e.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Store.WriteChapter(id, "雨下了一夜。\n林晚点起灯，手还在抖。", nil); err != nil {
		t.Fatal(err)
	}
	used := model.Usage{Input: 100, Output: 10}
	f := &fake{steps: []model.Reply{
		{Text: "我先翻一下。", Reasoning: "要查正文", Usage: used, Thinking: []model.Thought{{Text: "要查正文"}}, Calls: []model.Call{
			call("c1", toolSearchBook, map[string]any{"query": "林晚"}),
			call("c2", toolNote, map[string]any{"op": "add", "place": "body", "category": "人物", "text": "林晚怕黑", "names": []string{"林晚"}, "sources": []string{"林晚怕黑"}}),
		}},
		{Text: "第一章里她点灯时手在抖，和怕黑对得上。", Reasoning: "对上了", Usage: used},
	}}
	e.Talk = Model{Model: limits, Completer: f, Name: "deepseek/deepseek-flash", Effort: model.EffortHigh}
	var stepped []workspace.Event
	e.Step = func(ev workspace.Event) { stepped = append(stepped, ev) }
	got, err := e.Say(context.Background(), session, workspace.Event{Text: "林晚怕黑，这和第一章对得上吗？"})
	if err != nil {
		t.Fatal(err)
	}
	events, err := e.Store.Events(session)
	if err != nil {
		t.Fatal(err)
	}
	var roles []string
	for _, ev := range events {
		roles = append(roles, ev.Role)
	}
	if strings.Join(roles, " ") != "user call result result assistant" || len(stepped) != 3 || stepped[0].ID != events[1].ID {
		t.Fatalf("events %v, stepped %d", roles, len(stepped))
	}
	c := events[1]
	if c.ReplyTo != got.User.ID || c.Text != "我先翻一下。" || len(c.Calls) != 2 || c.Calls[0].Label != "搜了全书「林晚」" || c.Calls[1].Label != "记下：正文 · 人物：林晚怕黑" {
		t.Fatalf("call %+v", c)
	}
	set := setting(t, e)
	if len(set.Entries) != 1 || set.Entries[0].Place != workspace.PlaceBody || set.Entries[0].Names[0] != "林晚" {
		t.Fatalf("setting %+v", set.Entries)
	}
	r := set.Revisions[0]
	if r.By != workspace.ByTalk || r.Session != session || r.Turn != got.User.ID || r.Model != "deepseek/deepseek-flash" || r.Effort != "high" {
		t.Fatalf("revision %+v", r)
	}
	found, noted := events[2], events[3]
	if found.Call != "c1" || !strings.Contains(found.Text, "共 1 处，在「第一章」") || !strings.Contains(found.Text, "第 2 段：林晚点起灯") || found.Failed ||
		noted.Call != "c2" || noted.Text != "已记下，编号 1：正文 · 人物：林晚怕黑" || noted.Revision != r.ID {
		t.Fatalf("results %+v %+v", found, noted)
	}

	second := f.calls[1].Messages
	n := len(second)
	if n < 4 || second[n-3].Role != workspace.RoleAssistant || len(second[n-3].Calls) != 2 || len(second[n-3].Thinking) != 1 ||
		second[n-2].Role != "tool" || second[n-2].CallID != "c1" || second[n-1].CallID != "c2" || second[n-1].Content != noted.Text {
		t.Fatalf("second step %+v", second)
	}
	reply := got.Reply
	if reply.Text != "第一章里她点灯时手在抖，和怕黑对得上。" || reply.Reasoning != "要查正文\n\n对上了" || reply.Capped ||
		*reply.Usage != (workspace.Usage{Input: 200, Output: 20}) || reply.Sent.Part(workspace.PartLooked) == 0 {
		t.Fatalf("reply %+v", reply)
	}
	if parts := reply.Sent.Parts; parts[len(parts)-1].Name != workspace.PartInput {
		t.Fatalf("looked out of place: %+v", parts)
	}
}

// A reply asks at most ten times: the tenth is told to answer with what
// was found and may call nothing, and the reply says it stopped there.
func TestReplyStopsAtTheLastStep(t *testing.T) {
	e, session := setup(t)
	f := &fake{}
	for n := 1; n < maxSteps; n++ {
		f.steps = append(f.steps, model.Reply{Calls: []model.Call{call(fmt.Sprint(n), toolLookUp, map[string]any{"query": "雨"})}})
	}
	f.steps = append(f.steps, model.Reply{Text: "就查到这些。", Calls: []model.Call{call("x", toolLookUp, map[string]any{"query": "雨"})}})
	e.Talk = Model{Model: limits, Completer: f}
	got, err := e.Say(context.Background(), session, workspace.Event{Text: "雨的规则有哪些？"})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != maxSteps || f.calls[maxSteps-2].NoCalls || !f.calls[maxSteps-1].NoCalls {
		t.Fatalf("%d steps", len(f.calls))
	}
	last := f.calls[maxSteps-1].Messages
	if !strings.HasSuffix(last[len(last)-1].Content, capNote) || strings.HasSuffix(f.calls[maxSteps-2].Messages[len(f.calls[maxSteps-2].Messages)-1].Content, capNote) {
		t.Fatal("the last step was not told")
	}
	if !got.Reply.Capped || got.Reply.Text != "就查到这些。" {
		t.Fatalf("reply %+v", got.Reply)
	}
	events, _ := e.Store.Events(session)
	if len(events) != 1+2*(maxSteps-1)+1 {
		t.Fatalf("%d events", len(events))
	}
}

// What the tools return may take only the room the request left; past it
// the model is told to answer and may look nothing more up.
func TestLookingUpStopsWhenTheRoomIsFull(t *testing.T) {
	e, session := setup(t)
	id, err := e.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Store.WriteChapter(id, strings.Repeat("雨下了一夜，灯一直亮着。\n", 200), nil); err != nil {
		t.Fatal(err)
	}
	probe, err := Assemble(e.Store.Work(), 100000, workspace.Setting{}, nil, "雨", nil)
	if err != nil {
		t.Fatal(err)
	}
	small := limits
	small.Window = small.Output + 100000 - probe.Left + 200 // the index must fit first
	f := &fake{steps: []model.Reply{
		{Calls: []model.Call{call("1", toolReadProse, map[string]any{"chapter": "第一章"})}},
		{Text: "雨一直下。"},
	}}
	e.Talk = Model{Model: small, Completer: f}
	got, err := e.Say(context.Background(), session, workspace.Event{Text: "雨"})
	if err != nil {
		t.Fatal(err)
	}
	second := f.calls[1]
	if !second.NoCalls || second.Messages[len(second.Messages)-1].Content != fullNote || got.Reply.Capped {
		t.Fatalf("%+v", second.Messages)
	}
	events, _ := e.Store.Events(session)
	if !strings.Contains(events[2].Text, "雨下了一夜") {
		t.Fatal("the result was not kept whole in the talk")
	}
}

// A note keeps to its rules: verbatim quotes of what
// was said before, the author's words for what settles, the work's
// categories, and names given once.
func TestNoteKeepsTheRules(t *testing.T) {
	e, session := setup(t)
	u, _ := turn(t, e, session, "主角叫林晚，她怕黑。", "也许她小时候在矿井里迷过路？")
	ok, err := e.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: "矿井这个好，就这样。另外雨不会停。"})
	if err != nil {
		t.Fatal(err)
	}
	revise(t, e,
		workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Names: []string{"林晚"}, Text: "林晚是主角"},
		workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "规则", Text: "雨每年停一天"})
	box := &toolbox{store: e.Store, session: session, turn: ok.ID, model: Model{Name: "deepseek/deepseek-flash"}}
	for _, c := range []struct {
		in   input
		want string // in the refusal; empty when it is noted
	}{
		{input{Op: "add", Place: "body", Category: "人物", Text: "她小时候在矿井里迷过路", Sources: []string{"她小时候在矿井里迷过路"}}, "author_said"},
		{input{Op: "add", Place: "body", Category: "人物", Text: "她小时候在矿井里迷过路", Sources: []string{"她小时候在矿井里迷过路"}, AuthorSaid: "也许她小时候"}, "作者在这次讨论里说过的话"},
		{input{Op: "add", Place: "open", Category: "人物", Text: "她怕水", Sources: []string{"她怕水"}}, "不是这次讨论里已经说过的话里逐字的原文"},
		{input{Op: "add", Place: "open", Category: "魔法", Text: "雨不会停", Sources: []string{"雨不会停"}}, "没有「魔法」这个分类"},
		{input{Op: "add", Place: "open", Category: "人物", Text: "主角怕黑", Names: []string{"晚晚"}, Sources: []string{"她怕黑"}}, "名字「晚晚」没有在条目或对话里出现过"},
		{input{Op: "add", Place: "open", Category: "人物", Text: "林晚怕黑", Names: []string{"林晚"}, Sources: []string{"她怕黑"}}, "已经在编号 1 的条目里"},
		{input{Op: "add", Place: "open", Category: "规则", Text: "雨不会停", Names: []string{"雨"}, Sources: []string{"雨不会停"}}, "只有人物、"},
		{input{Op: "edit", Entry: 1, Text: "林晚是女主角"}, "author_said"},
		{input{Op: "edit", Entry: 9, Text: "林晚是女主角"}, "没有编号 9"},
		{input{Op: "name", Entry: 1, Names: []string{"晚"}}, "原有名字 林晚 要原样排在前面"},
		{input{Op: "move", Entry: 2, Place: "dropped"}, "author_said"},
		{input{Op: "add", Place: "body", Category: "人物", Text: "她小时候在矿井里迷过路", Sources: []string{"她小时候在矿井里迷过路"}, AuthorSaid: "矿井这个好，就这样"}, ""},
		{input{Op: "move", Entry: 2, Place: "dropped", AuthorSaid: "雨不会停"}, ""},
		{input{Op: "add", Place: "body", Category: "规则", Text: "雨不会停", Sources: []string{"雨不会停"}}, ""},
		{input{Op: "name", Entry: 1, Names: []string{"林晚", "主角"}}, ""},
	} {
		out := box.note(c.in)
		if c.want == "" && out.failed || c.want != "" && (!out.failed || !strings.Contains(out.text, c.want)) {
			t.Errorf("%+v: %s", c.in, out.text)
		}
	}
	set := setting(t, e)
	if len(set.Entries) != 4 || set.Entries[1].Place != workspace.PlaceDropped || set.Entries[0].Names[1] != "主角" {
		t.Fatalf("%+v", set.Entries)
	}
	mine := set.Entries[2].Sources
	if len(mine) != 2 || mine[0].Message == u.ID || mine[1].Message != ok.ID || mine[1].Quote != "矿井这个好，就这样" {
		t.Fatalf("sources %+v", mine)
	}
}

// The tools read the book by the names the model is shown: a title, or
// its volume's before it when two share one. A long chapter is read a
// part at a time, and long lists a page at a time.
func TestToolsReadTheBook(t *testing.T) {
	e, session := setup(t)
	write := func(parent, title, text string) string {
		id, err := e.Store.NewChapter(parent, title, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = e.Store.WriteChapter(id, text, nil); err != nil {
			t.Fatal(err)
		}
		return id
	}
	v1, _ := e.Store.NewChapter("", "第一卷", true)
	v2, _ := e.Store.NewChapter("", "第二卷", true)
	first := write(v1, "雨", strings.Repeat("灯亮着。"+strings.Repeat("雨", 600)+"\n", 6))
	write(v2, "雨", "又下雨了。")
	if err := e.Store.WriteSummary(first, workspace.Summary{Text: "林晚回到旧城。", Of: "旧的"}); err != nil {
		t.Fatal(err)
	}
	turn(t, e, session, "灯是谁点的？", "也许是她妹妹。")
	box := &toolbox{store: e.Store, session: session}
	for _, c := range []struct {
		name string
		in   map[string]any
		want []string
	}{
		{toolSearchBook, map[string]any{"query": "灯亮着"}, []string{"共 6 处，在「第一卷/雨」。", "「第一卷/雨」第 6 段：灯亮着。"}},
		{toolSearchBook, map[string]any{"query": "雪"}, []string{"全书的正文里没有「雪」。"}},
		{toolReadProse, map[string]any{"chapter": "《第二卷/雨》"}, []string{"「第二卷/雨」共 1 段。\n[1] 又下雨了。"}},
		{toolReadProse, map[string]any{"chapter": "第一卷/雨", "from": 2}, []string{"[2] 灯亮着", "[5] 灯亮着", "（读到第 5 段为止；接着读，from 填 6。）"}},
		{toolReadProse, map[string]any{"chapter": "第一卷"}, []string{"是一卷，没有正文"}},
		{toolReadSummary, map[string]any{"chapters": []string{"第一卷/雨", "第二卷/雨"}}, []string{"「第一卷/雨」：林晚回到旧城。\n（这份摘要写好以后，正文又改过。）", "「第二卷/雨」还没有摘要"}},
		{toolSearchTalks, map[string]any{"query": "妹妹"}, []string{"共 1 处，在「这一次的讨论」。", "AI：也许是她妹妹。"}},
	} {
		out := box.run(call("1", c.name, c.in))
		for _, w := range c.want {
			if !strings.Contains(out.text, w) {
				t.Errorf("%s %v: %s lacks %q", c.name, c.in, out.text, w)
			}
		}
	}
	if out := box.run(call("1", toolReadProse, map[string]any{"chapter": "第一卷/雨", "from": 2})); strings.Contains(out.text, "[6]") {
		t.Error("read past the cap")
	}
	// The setting is looked up by number, the entries a name matches first.
	revise(t, e, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "规则", Text: "灯是林晚的妹妹点的。"},
		workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "人物", Text: "她靠修灯过活。", Names: []string{"林晚", "晚晚"}})
	out := box.run(call("1", toolLookUp, map[string]any{"query": "林晚"})).text
	named, said := strings.Index(out, "[2]（悬而未决｜人物｜林晚、晚晚）她靠修灯过活。"), strings.Index(out, "[1]（设定正文｜规则）灯是林晚的妹妹点的。")
	if named < 0 || said < named {
		t.Errorf("look up: %s", out)
	}
	if out := box.run(call("1", toolLookUp, map[string]any{"query": "雪"})).text; out != "设定里没有「雪」。" {
		t.Errorf("look up nothing: %s", out)
	}
}
