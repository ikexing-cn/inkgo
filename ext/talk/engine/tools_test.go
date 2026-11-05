package discussion

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"writer/ext/ai/aiapi"

	"writer/model"
	"writer/workspace"
)

func BenchmarkSearchBook(b *testing.B) {
	st, err := workspace.Create(filepath.Join(b.TempDir(), "work"), "基准作品", "非虚构", "zh-Hans")
	if err != nil {
		b.Fatal(err)
	}
	defer st.Close()
	for i := range 100 {
		id, err := st.NewChapter("", fmt.Sprintf("第%d章", i+1), false)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := st.WriteChapter(id, strings.Repeat("雨落在旧城的街道上", 300)+"林晚", nil); err != nil {
			b.Fatal(err)
		}
	}
	box := toolbox{extra: allTools(), allowed: tools(workspace.Work{Language: workspace.Chinese}), store: st}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if got := box.searchBook(searchArgs{Query: "林晚"}); got.failed || !strings.Contains(got.text, "共 100 处") {
			b.Fatalf("search: %+v", got)
		}
	}
}

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
// next step sees them, and each records what its own request used.
func TestReplyLooksUpAndNotes(t *testing.T) {
	e, session := setup(t)
	id, err := e.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Store.WriteChapter(id, "雨下了一夜。\n林晚点起灯，手还在抖。", nil); err != nil {
		t.Fatal(err)
	}
	first, last := model.Usage{Input: 100, Output: 10}, model.Usage{Input: 180, Output: 20}
	f := &fake{steps: []model.Reply{
		{Text: "我先翻一下。", Reasoning: "要查正文", Usage: first, Thinking: []model.Thought{{Text: "要查正文"}}, Calls: []model.Call{
			call("c1", toolSearchBook, map[string]any{"query": "林晚"}),
			call("c2", toolNote, map[string]any{"op": "add", "place": "body", "category": "人物", "text": "林晚怕黑", "names": []string{"林晚"}, "sources": []string{"林晚怕黑"}}),
		}},
		{Text: "第一章里她点灯时手在抖，和怕黑对得上。", Reasoning: "对上了", Usage: last},
	}}
	e.Talk = aiapi.Handle{Model: aiapi.Model(limits), Completer: f, Name: "deepseek/deepseek-flash", Effort: model.EffortHigh}
	var stepped []workspace.Event
	e.Step = func(ev workspace.Event) { stepped = append(stepped, ev) }
	got, err := e.Say(context.Background(), session, workspace.Event{Text: "林晚怕黑，这和第一章对得上吗？"})
	if err != nil {
		t.Fatal(err)
	}
	// Each step is a request of the talk; the second sends what was looked
	// up.
	if len(f.purposes) != 2 || f.purposes[0].Feature != workspace.FeatureTalk || f.purposes[1].Talk != session ||
		slices.ContainsFunc(f.purposes[0].Parts, func(p workspace.Part) bool { return p.Name == workspace.PartLooked }) ||
		!slices.ContainsFunc(f.purposes[1].Parts, func(p workspace.Part) bool { return p.Name == workspace.PartLooked && p.Tokens > 0 }) {
		t.Fatalf("the record would say %+v", f.purposes)
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
	if c.ReplyTo != got.User.ID || c.Text != "我先翻一下。" || len(c.Calls) != 2 || c.Usage == nil || *c.Usage != (workspace.Usage{Input: 100, Output: 10}) || c.Calls[0].Label != "搜了全书「林晚」" || c.Calls[1].Label != "记下：正文 · 人物：林晚怕黑" {
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
		*reply.Usage != (workspace.Usage{Input: 180, Output: 20}) || reply.Sent.Part(workspace.PartLooked) == 0 {
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
	e.Talk = aiapi.Handle{Model: aiapi.Model(limits), Completer: f}
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

// What the tools return may take only the room the request left, as the
// provider counted the request before where it did; past it the model is
// told to answer and may look nothing more up.
func TestLookingUpStopsWhenTheRoomIsFull(t *testing.T) {
	e, session := setup(t)
	id, err := e.Store.NewChapter("", "第一章", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Store.WriteChapter(id, strings.Repeat("雨下了一夜，灯一直亮着。\n", 200), nil); err != nil {
		t.Fatal(err)
	}
	probe, err := Assemble(e.Store.Work(), 100000, workspace.Setting{}, workspace.Tree{}, nil, "雨", Along{Tools: allTools(), Material: aiapi.Material{Setting: true, Outline: true}})
	if err != nil {
		t.Fatal(err)
	}
	small := limits
	small.Window = small.Output + 100000 - probe.Left + 200 // the index must fit first
	f := &fake{steps: []model.Reply{
		{Calls: []model.Call{call("1", toolReadProse, map[string]any{"chapter": "第一章"})}},
		{Text: "雨一直下。"},
	}}
	e.Talk = aiapi.Handle{Model: aiapi.Model(small), Completer: f}
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

	// Room enough by estimate, but not as counted.
	f = &fake{steps: []model.Reply{
		{Calls: []model.Call{call("1", toolReadProse, map[string]any{"chapter": "第一章"})}, Usage: model.Usage{Input: limits.Input() - 600, Output: 100}},
		{Text: "雨一直下。"},
	}}
	e.Talk = aiapi.Handle{Model: aiapi.Model(limits), Completer: f}
	if _, err = e.Say(context.Background(), session, workspace.Event{Text: "雨"}); err != nil {
		t.Fatal(err)
	}
	if second = f.calls[1]; !second.NoCalls || second.Messages[len(second.Messages)-1].Content != fullNote {
		t.Fatalf("counted %+v", second.Messages[len(second.Messages)-1])
	}
	f = &fake{steps: []model.Reply{
		{Calls: []model.Call{call("1", toolReadProse, map[string]any{"chapter": "第一章"})}},
		{Text: "雨一直下。"},
	}}
	e.Talk = aiapi.Handle{Model: aiapi.Model(limits), Completer: f}
	if _, err = e.Say(context.Background(), session, workspace.Event{Text: "雨"}); err != nil {
		t.Fatal(err)
	}
	if second = f.calls[1]; second.NoCalls {
		t.Fatal("no room by estimate")
	}
}

// The tools read the book by the names the model is shown: a title, or
// its volume's before it when two share one, or what is one name only but
// for spaces and marks, or begins it, or is in it; for a name the book has
// not, the model is told those it may mean. A long chapter is read a part
// at a time, and long lists a page at a time.
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
	write(v1, "雨", strings.Repeat("灯亮着。"+strings.Repeat("雨", 2000)+"\n", 6))
	write(v2, "雨", "又下雨了。")
	write(v2, "第三章：停电", "灯灭了。")
	turn(t, e, session, "灯是谁点的？", "也许是她妹妹。")
	box := &toolbox{extra: allTools(), allowed: tools(workspace.Work{Language: workspace.Chinese}), store: e.Store, session: session}
	for _, c := range []struct {
		name string
		in   map[string]any
		want []string
	}{
		{toolSearchBook, map[string]any{"query": "灯亮着"}, []string{"共 6 处，在「第一卷/雨」。", "「第一卷/雨」第 6 段：灯亮着。"}},
		{toolSearchBook, map[string]any{"query": "雪"}, []string{"全书的正文里没有「雪」。"}},
		{toolReadProse, map[string]any{"chapter": "《第二卷/雨》"}, []string{"「第二卷/雨」共 1 段。\n第 1 段：又下雨了。"}},
		{toolReadProse, map[string]any{"chapter": "第一卷/雨", "from": 2}, []string{"第 2 段：灯亮着", "第 5 段：灯亮着", "（读到第 5 段为止；接着读，from 填 6。）"}},
		{toolReadProse, map[string]any{"chapter": "第一卷"}, []string{"是一卷，没有正文"}},
		{toolReadProse, map[string]any{"chapter": "第三章 停电"}, []string{"「第三章：停电」共 1 段。\n第 1 段：灯灭了。"}},
		{toolReadProse, map[string]any{"chapter": "第三章"}, []string{"「第三章：停电」共 1 段。"}},
		{toolReadProse, map[string]any{"chapter": "停电"}, []string{"「第三章：停电」共 1 段。"}},
		{toolReadProse, map[string]any{"chapter": "雨"}, []string{"「雨」对得上好几章：「第一卷/雨」「第二卷/雨」；写全章名。"}},
		{toolReadProse, map[string]any{"chapter": "第三张：停电"}, []string{"没有叫「第三张：停电」的章；相近的有「第三章：停电」。"}},
		{toolReadProse, map[string]any{"chapter": "第九章"}, []string{"没有叫「第九章」的章；章名要和搜全书的结果里写的一样。"}},
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
	if got := box.label(call("1", toolReadProse, map[string]any{"chapter": "停电"})); got != "读了「第三章：停电」" {
		t.Errorf("labels %s", got)
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

func TestReadSummaryIsAnUnknownTool(t *testing.T) {
	box := &toolbox{extra: allTools(), allowed: tools(workspace.Work{Language: workspace.Chinese})}
	c := call("old", "read_summary", map[string]any{"chapters": []string{"第三章", "雪"}})
	if out := box.run(c); !out.failed || out.text != "没有叫 read_summary 的工具" {
		t.Fatalf("retired tool: %+v", out)
	}
	if got := box.label(c); got != "read_summary" {
		t.Fatalf("unknown tool label = %q", got)
	}
}

func TestSnippetMeasure(t *testing.T) {
	text := strings.Repeat("甲", around+1) + "\n关键\n" + strings.Repeat("a", around+1)
	want := "…" + strings.Repeat("甲", around-1) + " 关键 " + strings.Repeat("a", around-1) + "…"
	if got := snippet(text, "关键"); got != want {
		t.Fatalf("snippet = %q, want %q", got, want)
	}
}

func TestSearchBookIgnoresDamagedCompanions(t *testing.T) {
	for _, companion := range []string{"marks.json", "outline.txt"} {
		t.Run(companion, func(t *testing.T) {
			e, _ := setup(t)
			id, err := e.Store.NewChapter("", "章节", false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := e.Store.WriteChapter(id, "雨中的林晚", nil); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(e.Store.Root(), "chapters", "章节."+companion)
			if companion == "outline.txt" {
				dir, err := e.Store.ExtDir("outline")
				if err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(dir, id+".txt")
			}
			if companion == "marks.json" {
				err = os.WriteFile(path, []byte("broken"), 0600)
			} else {
				err = os.Mkdir(path, 0700)
			}
			if err != nil {
				t.Fatal(err)
			}
			box := toolbox{extra: allTools(), allowed: tools(workspace.Work{Language: workspace.Chinese}), store: e.Store}
			out := box.searchBook(searchArgs{Query: "林晚"})
			if out.failed || out.text != "共 1 处，在「章节」。\n「章节」第 1 段：雨中的林晚" {
				t.Fatalf("prose-only search = %+v", out)
			}
		})
	}
}

func TestLabelsWithoutUnneededReads(t *testing.T) {
	box := toolbox{extra: allTools(), allowed: tools(workspace.Work{Language: workspace.Chinese})}
	for _, c := range []struct {
		name string
		in   map[string]any
		want string
	}{
		{toolSearchBook, map[string]any{"query": "林晚"}, "搜了全书「林晚」"},
		{toolSearchTalks, map[string]any{"query": "雨"}, "搜了讨论「雨」"},
		{toolReadOutline, map[string]any{"line": "主线"}, "查了大纲「主线」这条线"},
		{toolNote, map[string]any{"op": "add", "place": "body", "category": "人物", "text": "林晚怕黑"}, "记下：正文 · 人物：林晚怕黑"},
	} {
		if got := box.label(call("1", c.name, c.in)); got != c.want {
			t.Fatalf("label = %q, want %q", got, c.want)
		}
	}

}

// Every tool's schema lists exactly the fields its argument type decodes,
// so a schema and its type cannot drift apart.
func TestToolSchemasMatchTheirArguments(t *testing.T) {
	args := map[string]any{
		toolSearchBook:  searchArgs{},
		toolReadProse:   readProseArgs{},
		toolLookUp:      lookUpArgs{},
		toolSearchTalks: searchArgs{},
		toolReadOutline: outlineArgs{},
		toolNote:        noteArgs{},
		toolPropose:     proposeArgs{},
	}
	if len(args) != len(combined(allTools())) {
		t.Fatalf("%d argument types for %d tools", len(args), len(combined(allTools())))
	}
	for _, tl := range combined(allTools()) {
		typ, ok := args[tl.Name]
		if !ok {
			t.Fatalf("no argument type for %s", tl.Name)
		}
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(tl.Schema(workspace.Work{Language: workspace.Chinese}), &schema); err != nil {
			t.Fatalf("%s schema: %v", tl.Name, err)
		}
		var properties []string
		for name := range schema.Properties {
			properties = append(properties, name)
		}
		var fields []string
		rt := reflect.TypeOf(typ)
		for i := range rt.NumField() {
			fields = append(fields, strings.Split(rt.Field(i).Tag.Get("json"), ",")[0])
		}
		slices.Sort(fields)
		slices.Sort(properties)
		if !slices.Equal(fields, properties) {
			t.Errorf("%s: arguments %v, schema %v", tl.Name, fields, properties)
		}
	}
}
