package discussion

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/lore"
	"writer/ext/outline"
	"writer/ext/propose"
	"writer/inference"
	"writer/model"
	"writer/workspace"
)

func allTools() []aiapi.Tool {
	return append(append(lore.TalkTools(), outline.TalkTools()...), propose.TalkTool())
}
func materialTools(m aiapi.Material) []aiapi.Tool {
	var out []aiapi.Tool
	for _, tl := range allTools() {
		if (tl.Name == toolLookUp || tl.Name == toolNote) && !m.Setting || tl.Name == toolReadOutline && !m.Outline {
			continue
		}
		out = append(out, tl)
	}
	return out
}
func tools(w workspace.Work) []model.Tool { return toolsOf(w, combined(allTools())) }
func rulesFor(m aiapi.Material) string    { return rulesWith(m, combined(materialTools(m))) }
func (t *toolbox) external(n string, in any) outcome {
	b, _ := json.Marshal(in)
	for _, tl := range allTools() {
		if tl.Name == n {
			r := tl.Run(context.Background(), t.env(), b)
			return outcome{r.Text, r.Failed, r.Revision, r.Proposal}
		}
	}
	panic(n)
}
func (t *toolbox) note(in noteArgs) outcome           { return t.external(toolNote, in) }
func (t *toolbox) lookUp(in lookUpArgs) outcome       { return t.external(toolLookUp, in) }
func (t *toolbox) readOutline(in outlineArgs) outcome { return t.external(toolReadOutline, in) }

type lookUpArgs struct {
	Query string `json:"query"`
}
type noteArgs struct {
	Op         string   `json:"op"`
	Entry      int      `json:"entry"`
	Chapter    string   `json:"chapter"`
	Place      string   `json:"place"`
	Category   string   `json:"category"`
	Text       string   `json:"text"`
	Names      []string `json:"names"`
	Sources    []string `json:"sources"`
	AuthorSaid string   `json:"author_said"`
}
type outlineArgs struct {
	Line    string `json:"line"`
	Query   string `json:"query"`
	Chapter string `json:"chapter"`
	From    int    `json:"from"`
}

const noteRules = `

记设定（note_setting）：
- 这次讨论里定下了故事的内容（人物、事件、地点、规则……），就用 note_setting 记进设定文档，一次一个操作；没有就不记。寒暄、对正文写法的评价和改稿建议不是设定；提问只记下面说的那一种。
- sources 和 author_said 都要逐字复制本次讨论里已经说过的话；你这次回复里的话还不能引用。
- 作者自己说出的设定 add 到 body，sources 引作者原话。你以前提过、作者这次明确接受的，add 到 body 或把 open 里那条 move 到 body，sources 引你以前的原话，author_said 引作者接受的话。
- 你以前提过、作者没表态、还值得记的点子，add 到 open，sources 引你以前的原话；需要作者裁决的矛盾也放 open。你在这次回复里提出的新点子，等作者下次开口后再记。
- 你以前问过作者、答案会定下故事内容、作者还没回答的问题，add 到 open，text 写成以“？”结尾的问句，sources 引你问的原话。作者答过的不要再问。
- 作者明确不要的：文档里有就 move 到 dropped，没有就 add 到 dropped，author_said 引作者的话。已放弃的不要换个说法加回来。
- 改动作者已经定下的条目（body、dropped 里的，以及作者自己改过的），必须在 author_said 里引作者明确改口或表态的原话。只是你推断出的矛盾，不要改原条目，新增一条 open 写清楚矛盾在哪。
- 设定里的事从某一章起变了（人物受了伤、换了身份、关系变了），不改原条目，用 change 记：entry 是那一条的编号，chapter 是从哪一章（或哪一卷）起，text 写从那以后这一条的完整内容，不是只写变了的那一句。作者说的放 body，sources 或 author_said 引作者原话；你推断的放 open，sources 引你以前的原话。条目下面「「某章」起：……」的几行是已经记下的变化，从那一章起，这一条就是那一行。
- 已有条目说过的不要重复新增，要补充就 edit。text 用简洁的陈述句，忠实于原话，不添加对话里没有的细节；category 拿不准用“其他”。
- names 只给人物、地点、组织、物品、力量体系这几类条目，第一个是正式名称，后面是别名；每个名字都要在条目或对话里逐字出现，一个名字只放在讲它本身的那一条里。已有条目缺了名字，用 name 给出完整的 names，原有名字原样排在前面。
- 作者的消息里，【引用正文《章名》】下面是作者从自己正文里引来的段落，可以引用；但只记作者借它说明或认定的设定，不要把情节逐句记成设定。
- 工具说没有记下时，按它说的改了再记，或者放弃。记下了不用在回复里告诉作者，作者看得到。`
const outlineRules = `- 大纲是作者排的卡片，先后是故事里发生的先后，可能和章节的先后不同。问到情节先后、某条线的进展、还没写的打算时，先用 read_outline 查。大纲和正文对不上时，以正文为准，并告诉作者哪里不一致。
`

var proposeRules = propose.TalkTool().Prompt.Section
var talkRules = talkIntro + talkManners + talkMust + settingRules + contentRule + lookupStart + lookupOutline + lookupSettingRule + lookupEnd + outlineRules + lookupStop + noteRules + proposeRules

const (
	toolLookUp      = "look_up_setting"
	toolReadOutline = "read_outline"
	toolNote        = "note_setting"
)

func TestContributedToolPanicFailsCallAndTalkContinuesWithOwnerFailure(t *testing.T) {
	e, session := setup(t)
	h, err := ext.New(ext.Options{}, ext.Module{ID: "ai", Setup: func(r *ext.Registrar) { ext.Declare(r, aiapi.Tools) }}, ext.Module{ID: "broken", Optional: []string{"ai"}, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, aiapi.Tools, "panic", aiapi.Tool{Name: "panic", Schema: func(workspace.Work) json.RawMessage { return json.RawMessage(`{"type":"object"}`) }, Run: func(context.Context, aiapi.ToolEnv, json.RawMessage) aiapi.ToolResult { panic("tool broke") }})
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	e.Tools = aiapi.SnapshotTools(h)
	f := &fake{steps: []model.Reply{{Calls: []model.Call{call("bad", "panic", map[string]any{})}}, {Text: "还可以继续。"}}}
	e.Talk = inference.Model{Model: limits, Completer: f}
	got, err := e.Say(context.Background(), session, workspace.Event{Text: "你好"})
	if err != nil || got.Reply.Text != "还可以继续。" {
		t.Fatal(got, err)
	}
	events, err := e.Store.Events(session)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range events {
		if ev.Role == workspace.RoleResult {
			found = ev.Failed && strings.Contains(ev.Text, "tool broke")
		}
	}
	if !found {
		t.Fatal(events)
	}
	ledger := h.Ledger("broken")
	if ledger.State != ext.Failed || len(ledger.Errors) != 1 || ledger.Errors[0].Target != "ai.tools.panic" {
		t.Fatal(ledger)
	}
	if h.Ledger("ai").State != ext.On {
		t.Fatal("AI stopped with optional tool owner")
	}
}

func TestToolPromptsFollowToolsIndependentlyOfMaterial(t *testing.T) {
	for _, m := range []aiapi.Material{{}, {Setting: true, Outline: true}} {
		off := rulesWith(m, combined(nil))
		on := rulesWith(m, combined(allTools()))
		for _, text := range []string{"记设定（note_setting）", "look_up_setting 查设定的全文和出处", "read_outline 查大纲的卡片和情节线", "- 大纲是作者排的卡片"} {
			if strings.Contains(off, text) || !strings.Contains(on, text) {
				t.Fatal(text)
			}
		}
	}
}

const lookupStart = `

查阅：
- 回答要用到书里的具体内容而手头没有时，先查再说，不要凭印象编：search_book 搜正文，read_prose 读某章正文，`

const lookupOutline = `read_outline 查大纲的卡片和情节线，`

const lookupSettingRule = `look_up_setting 查设定的全文和出处，`

const lookupEnd = `search_talks 搜以前的讨论。
`

const indexNote = "设定较多，这里只列目录：每条的编号、分类、名字和第一句。要看全文和出处，用 look_up_setting 查。"

func (t *toolbox) propose(in proposeArgs) outcome { return t.external(toolPropose, in) }
