package discussion

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"writer/ext/ai/aiapi"
	"writer/talkbook"

	"writer/internal/textclip"
	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

// The tools a talk's model may call look things up in the book and
// its talks, note the setting, and propose a change of the
// prose. The model names a chapter by its title and an entry by its
// number, never by an ID.
const (
	toolSearchBook  = "search_book"
	toolReadProse   = "read_prose"
	toolSearchTalks = "search_talks"
)

const (
	maxHits = 20    // listed by one search
	maxRead = 10000 // runes of prose one read returns
	around  = 40    // runes shown on each side of a hit
)

// queryArg and fromArg are the arguments search_book and search_talks share.
const (
	queryArg = `"query":{"type":"string","description":"要找的词，越具体越好，比如人名、地名、物件"}`
	fromArg  = `"from":{"type":"integer","minimum":0,"description":"跳过前面多少处，接着上次列到的地方往下看"}`
)

// toolTable holds the tools that remain built into talk during migration.
var toolTable = []aiapi.Tool{
	{
		Name:        toolSearchBook,
		Order:       10,
		Prompt:      aiapi.ToolPrompt{Order: 10, Lookup: "search_book 搜正文"},
		Description: "按词搜全书的正文，返回每处所在的章、段号和前后的文字。",
		Schema:      fixedSchema(`{"type":"object","properties":{` + queryArg + `,` + fromArg + `},"required":["query"]}`),
		Run:         runTool((*toolbox).searchBook),
		Label:       labelTool(searchBookLabel),
	},
	{
		Name:        toolReadProse,
		Order:       20,
		Prompt:      aiapi.ToolPrompt{Order: 20, Lookup: "read_prose 读某章正文"},
		Description: "读某一章的正文，段号从 1 起；不给段号就从头读。一次读到的字数有上限，读不完会说从哪里接着读。",
		Schema:      fixedSchema(`{"type":"object","properties":{"chapter":{"type":"string","description":"章名，和搜全书结果里写的一样"},"from":{"type":"integer","minimum":1},"to":{"type":"integer","minimum":1}},"required":["chapter"]}`),
		Run:         runTool((*toolbox).readProse),
		Label:       labelTool(readProseLabel),
	},
	{
		Name:        toolSearchTalks,
		Order:       40,
		Prompt:      aiapi.ToolPrompt{Order: 50, Lookup: "search_talks 搜以前的讨论"},
		Description: "按词搜这本书所有讨论的原文，包括这一次的，返回每处在哪次讨论、是谁说的和前后的文字。",
		Schema:      fixedSchema(`{"type":"object","properties":{` + queryArg + `,` + fromArg + `},"required":["query"]}`),
		Run:         runTool((*toolbox).searchTalks),
		Label:       labelTool(searchTalksLabel),
	}}

// fixedSchema is schema for every work.
func fixedSchema(schema string) func(workspace.Work) json.RawMessage {
	return func(workspace.Work) json.RawMessage { return json.RawMessage(schema) }
}

func toolsOf(w workspace.Work, table []aiapi.Tool) []model.Tool {
	var out []model.Tool
	for _, tl := range table {
		out = append(out, model.Tool{Name: tl.Name, Description: tl.Description, Schema: tl.Schema(w)})
	}
	return out
}

// arguments is a tool's own arguments, trimmed as the talk reads them.
type arguments[T any] interface {
	trim() T
}

// runTool adapts a tool's typed handler: it decodes the call into T and,
// what does not decode, tells the model, which may try again.
func runTool[T arguments[T]](run func(*toolbox, T) outcome) func(context.Context, aiapi.ToolEnv, json.RawMessage) aiapi.ToolResult {
	return func(ctx context.Context, env aiapi.ToolEnv, input json.RawMessage) aiapi.ToolResult {
		var in T
		if err := json.Unmarshal(input, &in); err != nil {
			return aiapi.ToolResult{Text: fmt.Sprintf("参数不是约定的 JSON：%v", err), Failed: true}
		}
		t := &toolbox{store: env.Store, session: env.Session, turn: env.Turn, at: env.At, cache: env.Shared, ctx: ctx, model: aiapi.Handle{Name: env.Model, Effort: model.Effort(env.Effort)}}
		r := run(t, in.trim())
		return aiapi.ToolResult{Text: r.text, Failed: r.failed, Revision: r.revision, Proposal: r.proposal}
	}
}
func labelTool[T arguments[T]](label func(*labelReader, T) string) func(aiapi.ToolEnv, json.RawMessage) string {
	return func(env aiapi.ToolEnv, input json.RawMessage) string {
		var in T
		json.Unmarshal(input, &in)
		return label(&labelReader{shared: env.Shared}, in.trim())
	}
}

// toolbox runs the calls of a reply to the author's words turn in session.
// In a talk that keeps to the chapter at, the setting is looked up as it
// stands there.
type toolbox struct {
	extra   []aiapi.Tool
	cache   *aiapi.ToolShared
	ctx     context.Context
	allowed []model.Tool
	store   *workspace.Store
	session string
	turn    string
	model   aiapi.Handle
	at      string
}

// outcome is what a call returns to the model, whether it failed, the
// revision a note wrote, and the change of the prose a call proposed.
type outcome struct {
	text     string
	failed   bool
	revision string
	proposal *workspace.Proposal
}

func failed(format string, v ...any) outcome {
	return outcome{text: fmt.Sprintf(format, v...), failed: true}
}

// run runs c; what fails is told to the model, which may try again.
func (t *toolbox) run(c model.Call) outcome {
	if slices.ContainsFunc(t.allowed, func(t model.Tool) bool { return t.Name == c.Name }) {
		for _, tool := range combined(t.extra) {
			if tool.Name == c.Name {
				env := t.env()
				env.Call = c.ID
				r := tool.Run(t.ctx, env, c.Input)
				return outcome{r.Text, r.Failed, r.Revision, r.Proposal}
			}
		}
	}
	return failed("没有叫 %s 的工具", c.Name)
}

// label says what c does in the author's words, as the reply shows it,
// naming each chapter as the book does.
func (t *toolbox) label(c model.Call) string {
	return t.labels([]model.Call{c})[0].Label
}

// labels says what calls do in the author's words, as the reply shows
// them, naming each chapter as the book does.
func (t *toolbox) labels(calls []model.Call) []workspace.Call {
	out := make([]workspace.Call, len(calls))
	for i, c := range calls {
		label := c.Name
		for _, tl := range combined(t.extra) {
			if tl.Name == c.Name && tl.Label != nil {
				label = tl.Label(t.env(), c.Input)
			}
		}
		out[i] = workspace.Call{ID: c.ID, Name: c.Name, Input: c.Input, Label: label}
	}
	return out
}

type labelReader struct{ shared *aiapi.ToolShared }

func (r *labelReader) named(chapter string) string { return r.shared.Named(chapter) }
func (t *toolbox) book() (talkbook.Book, error)    { return t.shared().Book() }
func (t *toolbox) shared() *aiapi.ToolShared {
	if t.cache == nil {
		t.cache = aiapi.NewToolShared(t.store)
	}
	return t.cache
}

// quoted is names each in 「」.
func quoted(names []string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString("「" + strings.TrimSpace(n) + "」")
	}
	return b.String()
}

// paragraphs are a chapter's lines that say something, numbered from 1.
func paragraphs(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// snippet is text around the first query in it.
func snippet(text, query string) string {
	text = prompt.OneLine(text)
	i := strings.Index(text, query)
	if i < 0 {
		return textclip.Clip(text, 2*around)
	}
	before, after := text[:i], text[i+len(query):]
	return textclip.Tail(before, around) + query + textclip.Clip(after, around)
}

// hit is a place a search found: where, and the words around.
type hit struct {
	where, line string
}

// hits lists found from from: how many and where first, then each, at
// most maxHits, and how to go on when more are left.
func hits(found []hit, from int, none string) string {
	if len(found) == 0 {
		return none
	}
	var places []string
	for _, h := range found {
		if !slices.Contains(places, h.where) {
			places = append(places, h.where)
		}
	}
	head := fmt.Sprintf("共 %d 处，在%s", len(found), quoted(places[:min(len(places), 10)]))
	if len(places) > 10 {
		head += fmt.Sprintf("等 %d 处", len(places))
	}
	lines := []string{head + "。"}
	from = min(max(from, 0), len(found))
	end := min(from+maxHits, len(found))
	for _, h := range found[from:end] {
		lines = append(lines, "「"+h.where+"」"+h.line)
	}
	if end < len(found) || from > 0 {
		more := fmt.Sprintf("这里列的是第 %d–%d 处。", from+1, end)
		if end < len(found) {
			more += fmt.Sprintf("要看后面的，from 填 %d 再搜一次。", end)
		}
		lines = append(lines, more)
	}
	return strings.Join(lines, "\n")
}

// searchArgs are the arguments of search_book and search_talks.
type searchArgs struct {
	Query string `json:"query"`
	From  int    `json:"from"`
}

func (s searchArgs) trim() searchArgs {
	s.Query = strings.TrimSpace(s.Query)
	return s
}

func (t *toolbox) searchBook(in searchArgs) outcome {
	if in.Query == "" {
		return failed("query 不能为空")
	}
	b, err := t.book()
	if err != nil {
		return failed("读不出章节：%v", err)
	}
	var found []hit
	texts, err := t.store.Texts()
	if err != nil {
		return failed("读不出正文：%v", err)
	}
	for _, n := range b.Docs {
		for k, p := range paragraphs(texts[n.ID]) {
			if strings.Contains(p, in.Query) {
				found = append(found, hit{b.Name[n.ID], fmt.Sprintf("第 %d 段：%s", k+1, snippet(p, in.Query))})
			}
		}
	}
	return outcome{text: hits(found, in.From, "全书的正文里没有「"+in.Query+"」。")}
}

func searchBookLabel(_ *labelReader, in searchArgs) string {
	return "搜了全书「" + in.Query + "」"
}

type readProseArgs struct {
	Chapter string `json:"chapter"`
	From    int    `json:"from"`
	To      int    `json:"to"`
}

func (r readProseArgs) trim() readProseArgs {
	r.Chapter = strings.TrimSpace(r.Chapter)
	return r
}

func (t *toolbox) readProse(in readProseArgs) outcome {
	b, err := t.book()
	if err != nil {
		return failed("读不出章节：%v", err)
	}
	n, missing := b.Find(in.Chapter)
	if n == nil {
		return failed("%s", missing)
	}
	if n.Folder {
		return failed("「%s」是一卷，没有正文；读它下面的章。", b.Name[n.ID])
	}
	ch, err := t.store.ReadChapter(n.ID)
	if err != nil {
		return failed("读不出「%s」：%v", b.Name[n.ID], err)
	}
	ps := paragraphs(ch.Text)
	if len(ps) == 0 {
		return outcome{text: "「" + b.Name[n.ID] + "」还没有正文。"}
	}
	from, to := max(in.From, 1), in.To
	if to == 0 || to > len(ps) {
		to = len(ps)
	}
	if from > len(ps) || to < from {
		return failed("「%s」共 %d 段，没有第 %d–%d 段。", b.Name[n.ID], len(ps), from, to)
	}
	lines := []string{fmt.Sprintf("「%s」共 %d 段。", b.Name[n.ID], len(ps))}
	runes := 0
	for k := from; k <= to; k++ {
		p := ps[k-1]
		if runes > 0 && runes+len([]rune(p)) > maxRead {
			lines = append(lines, fmt.Sprintf("（读到第 %d 段为止；接着读，from 填 %d。）", k-1, k))
			break
		}
		runes += len([]rune(p))
		lines = append(lines, fmt.Sprintf("第 %d 段：%s", k, p)) // not "[k]", which names an entry
	}
	return outcome{text: strings.Join(lines, "\n")}
}

func readProseLabel(r *labelReader, in readProseArgs) string {
	in.Chapter = r.named(in.Chapter)
	switch {
	case in.From > 0 && in.To >= in.From:
		if in.To == in.From {
			return fmt.Sprintf("读了「%s」第 %d 段", in.Chapter, in.From)
		}
		return fmt.Sprintf("读了「%s」第 %d–%d 段", in.Chapter, in.From, in.To)
	case in.From > 1:
		return fmt.Sprintf("读了「%s」第 %d 段起", in.Chapter, in.From)
	}
	return "读了「" + in.Chapter + "」"
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

func (t *toolbox) searchTalks(in searchArgs) outcome {
	if in.Query == "" {
		return failed("query 不能为空")
	}
	sessions, err := t.store.Sessions()
	if err != nil {
		return failed("读不出讨论：%v", err)
	}
	var found []hit
	for _, s := range sessions {
		events, err := t.store.Events(s)
		if err != nil {
			return failed("读不出讨论：%v", err)
		}
		title := talkTitle(events)
		if s == t.session {
			title = "这一次的讨论"
		}
		for _, e := range events {
			who, said := "作者", e.Said()
			switch e.Role {
			case workspace.RoleUser:
			case workspace.RoleAssistant:
				who, said = "AI", e.Text
			default:
				continue
			}
			if strings.Contains(said, in.Query) {
				found = append(found, hit{title, who + "：" + snippet(said, in.Query)})
			}
		}
	}
	return outcome{text: hits(found, in.From, "讨论里没有说过「"+in.Query+"」。")}
}

func searchTalksLabel(_ *labelReader, in searchArgs) string {
	return "搜了讨论「" + in.Query + "」"
}

func (t *toolbox) env() aiapi.ToolEnv {
	return aiapi.ToolEnv{Store: t.store, Session: t.session, Turn: t.turn, At: t.at, Model: t.model.Name, Effort: string(t.model.Effort), Shared: t.shared()}
}
func combined(extra []aiapi.Tool) []aiapi.Tool {
	out := make([]aiapi.Tool, 0, len(toolTable)+len(extra))
	out = append(out, toolTable...)
	out = append(out, extra...)
	slices.SortStableFunc(out, func(a, b aiapi.Tool) int { return cmp.Compare(a.Order, b.Order) })
	return out
}
