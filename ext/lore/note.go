package lore

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"context"
	"encoding/json"
	"writer/ext/ai/aiapi"
	"writer/internal/textclip"
	"writer/prompt"
	"writer/talkbook"
	"writer/workspace"
)

// placeLabels name the places of the setting as the panel does.
var placeLabels = map[string]string{workspace.PlaceBody: "正文", workspace.PlaceOpen: "待确认", workspace.PlaceDropped: "已删除"}

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

func (n noteArgs) trim() noteArgs {
	n.Chapter = strings.TrimSpace(n.Chapter)
	return n
}

// describe says what a note does, in the words of the panel's history.
func describe(in noteArgs, set workspace.Setting) string {
	text := strings.TrimSpace(in.Text)
	e, _ := set.EntryByNumber(in.Entry)
	switch in.Op {
	case workspace.OpAdd:
		return placeLabels[in.Place] + " · " + in.Category + "：" + text
	case workspace.OpEdit:
		return "改写「" + textclip.Clip(e.Text, 16) + "」为：" + text
	case workspace.OpName:
		return "名称改为：" + strings.Join(workspace.CleanNames(in.Names), "、")
	case workspace.OpMove:
		if in.Place != "" {
			return "移到" + placeLabels[in.Place] + "：" + textclip.Clip(e.Text, 24)
		}
		return "改归" + in.Category + "：" + textclip.Clip(e.Text, 24)
	case workspace.OpChange:
		chapter := strings.TrimSpace(in.Chapter)
		return placeLabels[in.Place] + " · 「" + chapter + "」起：" + text
	}
	return in.Op
}

// note writes one op the model asks for into the setting, if it keeps to
// these rules:
//
//  1. Every quote is verbatim in words said in this talk before; the
//     author's words, for what settles.
//  2. Into the body goes what the author said, or what the AI said with the
//     author's words accepting it; into dropped, what the author refused.
//  3. What the author settled, the body and dropped and any entry the author
//     touched, changes only on the author's words.
//  4. Categories are the work's.
//  5. Names are given only to entries of NameCategories, each is said in
//     the entry or the talk, and is no other entry's. The name op only adds
//     names after an entry's, so needs none of the author's words.
//  6. A change from a chapter on is of a settled entry, and holds on the
//     author's words; the AI's waits on the author. One that holds from
//     where one holds already takes its words.
//
// What breaks a rule is told to the model, which may try again.
func (t *toolbox) note(in noteArgs) outcome {
	set, err := t.Store.Setting()
	if err != nil {
		return failed("读不出设定：%v", err)
	}
	events, err := t.Store.Events(t.Session)
	if err != nil {
		return failed("读不出这次讨论：%v", err)
	}
	var said []workspace.Event // what was said before, the newest first
	for n := len(events) - 1; n >= 0; n-- {
		if r := events[n].Role; r == workspace.RoleUser || r == workspace.RoleAssistant {
			said = append(said, events[n])
		}
	}
	var bad []string
	// source finds q in what was said, or in the author's words alone.
	source := func(q, field string, author bool) (workspace.Source, bool) {
		q = strings.TrimSpace(q)
		for _, m := range said {
			if author && m.Role != workspace.RoleUser {
				continue
			}
			text := m.Text
			if m.Role == workspace.RoleUser {
				text = m.Said()
			}
			words, ok := aiapi.Locate(text, q)
			if !ok {
				continue
			}
			src := workspace.Source{Session: t.Session, Message: m.ID, Quote: words}
			if i := strings.Index(text, words); i >= 0 && m.Role == workspace.RoleUser {
				start := utf8.RuneCountInString(text[:i])
				if qt, ok := m.QuoteAt(start, start+utf8.RuneCountInString(words)); ok {
					src.Chapter = qt.Chapter
				}
			}
			return src, true
		}
		where := "这次讨论里已经说过的话"
		if author {
			where = "作者在这次讨论里说过的话"
		}
		bad = append(bad, fmt.Sprintf("%s「%s」不是%s里逐字的原文；你这次回复里的话还不能引用", field, textclip.Clip(q, 40), where))
		return workspace.Source{}, false
	}
	out := workspace.Op{Op: in.Op, Place: in.Place, Category: in.Category, Text: strings.TrimSpace(in.Text), Names: workspace.CleanNames(in.Names)}
	// cite keeps a source once, though sources and author_said quote the
	// same words.
	cite := func(s workspace.Source) {
		if !slices.Contains(out.Sources, s) {
			out.Sources = append(out.Sources, s)
		}
	}
	fromAuthor := false
	for _, q := range in.Sources {
		if s, ok := source(q, "sources 里的", false); ok {
			cite(s)
			fromAuthor = fromAuthor || slices.ContainsFunc(said, func(m workspace.Event) bool { return m.ID == s.Message && m.Role == workspace.RoleUser })
		}
	}
	// settled takes the author's words that settle the op.
	settled := func() {
		if strings.TrimSpace(in.AuthorSaid) == "" {
			bad = append(bad, "要在 author_said 里引作者拍板、接受、拒绝或改口的原话")
		} else if s, ok := source(in.AuthorSaid, "author_said", true); ok {
			cite(s)
		}
	}
	if in.Category != "" && !slices.ContainsFunc(t.Store.Work().Categories, func(c workspace.Category) bool { return c.Name == in.Category }) {
		bad = append(bad, fmt.Sprintf("没有「%s」这个分类", in.Category))
	}
	if in.Place != "" && toolPlaceNames[in.Place] == "" {
		bad = append(bad, fmt.Sprintf("没有 %q 这个位置", in.Place))
	}
	var e workspace.Entry
	switch in.Op {
	case workspace.OpAdd:
		if in.Entry != 0 {
			bad = append(bad, "add 不带 entry")
		}
		if out.Text == "" || in.Category == "" || in.Place == "" || len(in.Sources) == 0 {
			bad = append(bad, "add 要有 place、category、text 和 sources")
		}
		switch {
		case in.Place == workspace.PlaceDropped, in.Place == workspace.PlaceBody && !fromAuthor:
			settled()
		}
	case workspace.OpEdit, workspace.OpMove, workspace.OpName:
		var ok bool
		e, ok = set.EntryByNumber(in.Entry)
		if !ok {
			bad = append(bad, fmt.Sprintf("设定里没有编号 %d 的条目", in.Entry))
			break
		}
		out.Entry = e.ID
		switch in.Op {
		case workspace.OpEdit:
			if out.Text == "" || in.Place != "" || in.Category != "" {
				bad = append(bad, "edit 要有改好的 text，不带 place、category（换位置或分类用 move）")
			}
		case workspace.OpMove:
			if in.Place == "" && in.Category == "" || out.Text != "" || len(in.Sources) > 0 {
				bad = append(bad, "move 要有 place 或 category，不带 text、sources（改写用 edit）")
			}
		case workspace.OpName:
			if len(out.Names) == 0 || out.Text != "" || in.Place != "" || in.Category != "" || len(in.Sources) > 0 {
				bad = append(bad, "name 要有 names，不带 text、place、category、sources")
			} else if len(out.Names) < len(e.Names) || !slices.Equal(out.Names[:len(e.Names)], e.Names) {
				bad = append(bad, fmt.Sprintf("name 只在原有名字后面加别名，原有名字 %s 要原样排在前面", strings.Join(e.Names, "、")))
			}
		}
		if in.Op != workspace.OpName && (e.Place != workspace.PlaceOpen || e.Author || in.Place == workspace.PlaceBody || in.Place == workspace.PlaceDropped) {
			settled()
		}
	case workspace.OpChange:
		var ok bool
		e, ok = set.EntryByNumber(in.Entry)
		if !ok {
			bad = append(bad, fmt.Sprintf("设定里没有编号 %d 的条目", in.Entry))
			break
		}
		out.Entry = e.ID
		if e.Place != workspace.PlaceBody {
			bad = append(bad, "change 只给设定正文里的条目记变化")
		}
		if out.Text == "" || in.Category != "" || len(in.Sources) == 0 || in.Place != workspace.PlaceBody && in.Place != workspace.PlaceOpen {
			bad = append(bad, "change 要有 chapter、place（body 或 open）、text 和 sources，不带 category")
		}
		b, err := t.Shared.Book()
		if err != nil {
			return failed("读不出章节：%v", err)
		}
		if strings.TrimSpace(in.Chapter) == "" {
			bad = append(bad, "change 要写从哪一章起（chapter）")
		} else if n, missing := b.Find(in.Chapter); n == nil {
			bad = append(bad, strings.TrimSuffix(missing, "。"))
		} else {
			out.From = n.ID
		}
		in.Chapter = b.Named(in.Chapter)
		if in.Place == workspace.PlaceBody {
			if !fromAuthor {
				settled()
			}
			if k := slices.IndexFunc(e.Changes, func(g workspace.Change) bool { return g.Place == workspace.PlaceBody && g.From == out.From }); k >= 0 {
				out = workspace.Op{Op: workspace.OpEditChange, Entry: e.ID, Change: e.Changes[k].ID, Text: out.Text, Sources: out.Sources}
			}
		}
	default:
		bad = append(bad, fmt.Sprintf("没有 %q 这个操作", in.Op))
	}
	if len(out.Names) > 0 {
		category, text := in.Category, out.Text
		if category == "" {
			category = e.Category
		}
		if text == "" {
			text = e.Text
		}
		switch {
		case in.Op == workspace.OpMove || in.Op == workspace.OpChange:
			bad = append(bad, in.Op+" 不带 names")
		case !slices.Contains(workspace.NameCategories, category):
			bad = append(bad, fmt.Sprintf("只有%s分类的条目带 names", strings.Join(workspace.NameCategories, "、")))
		default:
			for _, name := range out.Names {
				if !strings.Contains(text, name) && !slices.ContainsFunc(said, func(m workspace.Event) bool { return strings.Contains(m.Said(), name) }) {
					bad = append(bad, fmt.Sprintf("名字「%s」没有在条目或对话里出现过", name))
				}
				if n := slices.IndexFunc(set.Entries, func(x workspace.Entry) bool {
					return x.ID != out.Entry && x.Place != workspace.PlaceDropped && slices.Contains(x.Names, name)
				}); n >= 0 {
					bad = append(bad, fmt.Sprintf("名字「%s」已经在编号 %d 的条目里，一个名字只放在一条里", name, n+1))
				}
			}
		}
	}
	if len(bad) > 0 {
		return failed("没有记下：%s。", strings.Join(bad, "；"))
	}
	did := describe(in, set) // before it changes the entry
	r, err := t.Store.Revise(workspace.Revision{By: workspace.ByTalk, Session: t.Session, Turn: t.Turn,
		Model: t.Model, Effort: t.Effort, Ops: []workspace.Op{out}})
	if err != nil {
		return failed("没有记下：%v", err)
	}
	n := in.Entry
	if in.Op == workspace.OpAdd {
		n = len(set.Entries) + 1
	}
	return outcome{Text: fmt.Sprintf("已记下，编号 %d：%s", n, did), Revision: r.ID}
}

func noteLabel(r *labelReader, in noteArgs) string {
	in.Chapter = r.Shared.Named(in.Chapter)
	var set workspace.Setting
	if in.Op == workspace.OpEdit || in.Op == workspace.OpMove {
		set = r.setting()
	}
	return "记下：" + describe(in, set)
}

// toolPlaceNames name the places of the setting as the talk's rules do.
var toolPlaceNames = map[string]string{workspace.PlaceBody: "设定正文", workspace.PlaceOpen: "悬而未决", workspace.PlaceDropped: "已放弃"}

type lookUpArgs struct {
	Query string `json:"query"`
}

func (l lookUpArgs) trim() lookUpArgs {
	l.Query = strings.TrimSpace(l.Query)
	return l
}

func (t *toolbox) lookUp(in lookUpArgs) outcome {
	if in.Query == "" {
		return failed("query 不能为空")
	}
	set, err := t.Shared.Setting()
	if err != nil {
		return failed("读不出设定：%v", err)
	}
	b, err := t.Shared.Book()
	if err != nil {
		return failed("读不出章节：%v", err)
	}
	titles := map[string]string{}
	title := func(session string) string {
		if name, ok := titles[session]; ok {
			return name
		}
		events, _ := t.Store.Events(session)
		name := talkTitle(events)
		if session == t.Session {
			name = "这一次的讨论"
		}
		titles[session] = name
		return name
	}
	return lookupSetting(in.Query, set, b, t.At, title)
}

func lookUpLabel(_ *labelReader, in lookUpArgs) string {
	return "查了设定「" + in.Query + "」"
}

func lookupSetting(query string, set workspace.Setting, b talkbook.Book, at string, title func(string) string) outcome {
	chapters, later := b.Name, talkbook.Changes(set, b.Tree)
	if at != "" {
		set, _ = set.At(b.Tree.Order(), at)
		later = nil
	}
	var named, said []int
	for n, e := range set.Entries {
		matchesName := slices.ContainsFunc(e.Names, func(name string) bool { return strings.Contains(name, query) || strings.Contains(query, name) })
		if !matchesName &&
			!strings.Contains(prompt.Text(e), query) && !slices.ContainsFunc(later[e.ID], func(c string) bool { return strings.Contains(c, query) }) {
			continue
		}
		if matchesName {
			named = append(named, n)
		} else {
			said = append(said, n)
		}
	}
	matched := append(named, said...)
	numbers := set.EntryNumbers()
	var found []string
	for _, n := range matched[:min(len(matched), maxHits)] {
		e := set.Entries[n]
		head := workspace.EntryRef(numbers[e.ID]) + fmt.Sprintf("（%s｜%s", toolPlaceNames[e.Place], e.Category)
		if len(e.Names) > 0 {
			head += "｜" + strings.Join(e.Names, "、")
		}
		lines := []string{head + "）" + prompt.Text(e)}
		for _, c := range later[e.ID] {
			lines = append(lines, "  "+c)
		}
		for _, src := range e.Sources[:min(len(e.Sources), 3)] {
			where := ""
			switch {
			case src.Message != "":
				where = "讨论「" + title(src.Session) + "」"
			case src.Chapter != "":
				where = "正文「" + chapters[src.Chapter] + "」"
			}
			lines = append(lines, "  出处：「"+textclip.Clip(prompt.OneLine(src.Quote), 80)+"」"+where)
		}
		found = append(found, strings.Join(lines, "\n"))
	}
	switch {
	case len(found) == 0:
		return outcome{Text: "设定里没有「" + query + "」。"}
	case len(matched) > maxHits:
		return outcome{Text: strings.Join(found, "\n") + fmt.Sprintf("\n还有 %d 条没有列出，换一个更具体的词再查。", len(matched)-maxHits)}
	}
	return outcome{Text: strings.Join(found, "\n")}
}

// noteSchema is note_setting's schema, its categories the work's.
func noteSchema(w workspace.Work) json.RawMessage {
	var categories []string
	for _, c := range w.Categories {
		categories = append(categories, c.Name)
	}
	cats, _ := json.Marshal(categories)
	return json.RawMessage(`{"type":"object","properties":{` +
		`"op":{"type":"string","enum":["add","edit","move","name","change"]},` +
		`"entry":{"type":"integer","minimum":1,"description":"edit、move、name、change 改的条目的编号，即设定里方括号中的数字"},` +
		`"chapter":{"type":"string","description":"change 从哪一章（或哪一卷）起，章名和搜全书结果里写的一样"},` +
		`"place":{"type":"string","enum":["body","open","dropped"],"description":"body 是设定正文，open 是悬而未决，dropped 是已放弃"},` +
		`"category":{"type":"string","enum":` + string(cats) + `},` +
		`"text":{"type":"string","description":"一句能独立成立的设定；change 写从那一章起这一条的全文"},` +
		`"names":{"type":"array","items":{"type":"string"},"description":"条目说的专有名词，第一个是正式名称"},` +
		`"sources":{"type":"array","items":{"type":"string"},"description":"这条出自的原话，逐字复制自本次讨论里已经说过的话"},` +
		`"author_said":{"type":"string","description":"作者拍板、接受、拒绝或改口的原话，逐字复制"}` +
		`},"required":["op"]}`)
}

type toolbox struct{ aiapi.ToolEnv }
type labelReader struct{ aiapi.ToolEnv }

func (r *labelReader) setting() workspace.Setting { s, _ := r.Shared.Setting(); return s }

type outcome = aiapi.ToolResult

func failed(f string, v ...any) outcome { return outcome{Text: fmt.Sprintf(f, v...), Failed: true} }
func fixedSchema(s string) func(workspace.Work) json.RawMessage {
	return func(workspace.Work) json.RawMessage { return json.RawMessage(s) }
}

type arguments[T any] interface{ trim() T }

func runTool[T arguments[T]](run func(*toolbox, T) outcome) func(context.Context, aiapi.ToolEnv, json.RawMessage) aiapi.ToolResult {
	return func(_ context.Context, e aiapi.ToolEnv, in json.RawMessage) aiapi.ToolResult {
		var a T
		if err := json.Unmarshal(in, &a); err != nil {
			return failed("参数不是约定的 JSON：%v", err)
		}
		return run(&toolbox{e}, a.trim())
	}
}
func labelTool[T arguments[T]](label func(*labelReader, T) string) func(aiapi.ToolEnv, json.RawMessage) string {
	return func(e aiapi.ToolEnv, in json.RawMessage) string {
		var a T
		json.Unmarshal(in, &a)
		return label(&labelReader{e}, a.trim())
	}
}

const queryArg = `"query":{"type":"string","description":"要找的词，越具体越好，比如人名、地名、物件"}`
const maxHits = 20

// TalkTools returns the tools contributed by this extension.
func TalkTools() []aiapi.Tool {
	return []aiapi.Tool{
		{
			Name: "look_up_setting", Order: 30, Prompt: aiapi.ToolPrompt{Order: 40, Lookup: "look_up_setting 查设定的全文和出处", SettingIndex: "要看全文和出处，用 look_up_setting 查。"},
			Description: "按名字或词查设定，返回条目的编号、位置、分类、名字、全文和出处。",
			Schema:      fixedSchema(`{"type":"object","properties":{` + queryArg + `},"required":["query"]}`),
			Run:         runTool((*toolbox).lookUp),
			Label:       labelTool(lookUpLabel),
		},
		{
			Name: "note_setting", Order: 60, Prompt: aiapi.ToolPrompt{Order: 60, Section: noteRules},
			Description: "把这次讨论里定下的东西记进设定文档，一次一个操作：add 新增，edit 改写，move 换位置或分类，name 补名字，change 记一条从某一章起的变化。规则见系统提示里的「记设定」。",
			Schema:      noteSchema,
			Run:         runTool((*toolbox).note),
			Label:       labelTool(noteLabel),
		},
	}
}

// talkTitle names a talk by what the author said first, as the talks
// list does.
func talkTitle(events []workspace.Event) string {
	for _, e := range events {
		if e.Role == workspace.RoleUser {
			return textclip.Clip(prompt.OneLine(e.Text), 14)
		}
	}
	return "新对话"
}
