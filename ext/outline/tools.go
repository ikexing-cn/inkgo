package outline

import (
	"fmt"
	"strings"

	"context"
	"encoding/json"
	"writer/ext/ai/aiapi"
	"writer/talkbook"
	"writer/workspace"
)

type outlineGroup struct {
	head, none string
	rows       []string
}

type outlineArgs struct {
	Line    string `json:"line"`
	Query   string `json:"query"`
	Chapter string `json:"chapter"`
	From    int    `json:"from"`
}

func (o outlineArgs) trim() outlineArgs {
	o.Line = strings.TrimSpace(o.Line)
	o.Query = strings.TrimSpace(o.Query)
	o.Chapter = strings.TrimSpace(o.Chapter)
	return o
}

func (t *toolbox) readOutline(in outlineArgs) outcome {
	p, err := t.Store.Plan()
	if err != nil {
		return failed("大纲文件读不出来，暂时不能查阅。")
	}
	b, err := t.Shared.Book()
	if err != nil {
		return failed("读不出章节，暂时不能查阅大纲。")
	}
	var head string
	var groups []outlineGroup
	switch {
	case in.Line != "":
		var names []string
		var matches []workspace.PlotLine
		for _, l := range p.Lines {
			names = append(names, l.Name)
			if l.Name == in.Line {
				matches = append(matches, l)
			}
		}
		if len(matches) == 0 {
			if len(names) == 0 {
				return failed("没有叫「%s」的情节线；大纲还没有情节线。", in.Line)
			}
			return failed("没有叫「%s」的情节线；现有的情节线：%s。", in.Line, quoted(names))
		}
		g := outlineGroup{head: "情节线「" + in.Line + "」", none: "这条线没有卡片。"}
		for i, c := range p.Cards {
			for _, l := range matches {
				if c.Line == l.ID {
					g.rows = append(g.rows, talkbook.CardLine(p, b, i))
					break
				}
			}
		}
		groups = append(groups, g)
	case in.Chapter != "":
		n, missing := b.Find(in.Chapter)
		if n == nil {
			return failed("%s", missing)
		}
		ids := []string{n.ID}
		if n.Folder {
			for _, ch := range b.Docs {
				if ch != n && ch.Within(n) {
					ids = append(ids, ch.ID)
				}
			}
		}
		outlines, err := t.Store.OutlinesOf(ids...)
		if err != nil {
			return failed("读不出「%s」的纲。", b.Name[n.ID])
		}
		outline := func(n *workspace.Node) string {
			kind := "章纲"
			if n.Folder {
				kind = "卷纲"
			}
			text := strings.TrimSpace(outlines[n.ID])
			if text == "" {
				text = "（没有" + kind + "）"
			}
			return "「" + b.Name[n.ID] + "」" + kind + "：\n" + text
		}
		text := outline(n)
		if !n.Folder {
			groups = append(groups, outlineGroup{text, "这一章没有卡片。", talkbook.ChapterCards(p, b, n.ID)})
			break
		}
		head = text
		count := 0
		for _, ch := range b.Docs {
			if ch == n || !ch.Within(n) {
				continue
			}
			text := outline(ch)
			rows := talkbook.ChapterCards(p, b, ch.ID)
			count += len(rows)
			if len(rows) > 0 || in.From <= 0 {
				groups = append(groups, outlineGroup{text, "这一章没有卡片。", rows})
			}
		}
		if count == 0 {
			head += "\n这一卷没有卡片。"
		}
	case in.Query != "":
		g := outlineGroup{none: "没有含「" + in.Query + "」这个词的卡片。"}
		for i, c := range p.Cards {
			if strings.Contains(c.Title, in.Query) || strings.Contains(c.About, in.Query) || strings.Contains(c.When, in.Query) {
				g.rows = append(g.rows, talkbook.CardLine(p, b, i))
			}
		}
		groups = append(groups, g)
	default:
		counts := map[string]int{}
		unplaced := 0
		g := outlineGroup{none: "大纲是空的。"}
		for i, c := range p.Cards {
			counts[c.Line]++
			if cardChapter(b, c.Chapter) == "" {
				unplaced++
			}
			g.rows = append(g.rows, talkbook.CardLine(p, b, i))
		}
		lines := []string{fmt.Sprintf("大纲共 %d 张卡片，按故事里发生的先后排列。", len(p.Cards))}
		for _, l := range p.Lines {
			lines = append(lines, fmt.Sprintf("情节线「%s」：%d 张。", l.Name, counts[l.ID]))
		}
		lines = append(lines, fmt.Sprintf("没有情节线：%d 张。未放进章节：%d 张。", counts[""], unplaced))
		head = strings.Join(lines, "\n")
		groups = append(groups, g)
	}
	return outcome{Text: outlinePage(head, groups, in.From)}
}

func readOutlineLabel(r *labelReader, in outlineArgs) string {
	chapter := r.Shared.Named(in.Chapter)
	switch {
	case in.Line != "":
		return "查了大纲「" + in.Line + "」这条线"
	case chapter != "":
		return "查了大纲「" + chapter + "」的卡片"
	case in.Query != "":
		return "查了大纲「" + in.Query + "」"
	}
	return "查了大纲"
}

// outlinePage shares one page limit across a volume's chapters.
func outlinePage(head string, groups []outlineGroup, from int) string {
	total := 0
	for _, g := range groups {
		total += len(g.rows)
	}
	from = min(max(from, 0), total)
	end := min(from+maxCards, total)
	var lines []string
	if head != "" {
		lines = append(lines, head)
	}
	at := 0
	for _, g := range groups {
		start, stop := max(from-at, 0), min(end-at, len(g.rows))
		at += len(g.rows)
		if len(g.rows) > 0 && start >= stop {
			continue
		}
		if g.head != "" {
			lines = append(lines, g.head)
		}
		if len(g.rows) == 0 {
			lines = append(lines, g.none)
		} else {
			lines = append(lines, g.rows[start:stop]...)
		}
	}
	if total > 0 && from == total {
		lines = append(lines, "后面没有卡片了。")
	} else if end < total || from > 0 {
		more := fmt.Sprintf("这里列的是第 %d–%d 张，共 %d 张。", from+1, end, total)
		if end < total {
			more += fmt.Sprintf("要看后面的，from 填 %d 再查一次。", end)
		}
		lines = append(lines, more)
	}
	return strings.Join(lines, "\n")
}

type toolbox struct{ aiapi.ToolEnv }
type labelReader struct{ aiapi.ToolEnv }

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
const fromArg = `"from":{"type":"integer","minimum":0,"description":"跳过前面多少处，接着上次列到的地方往下看"}`
const maxCards = 200

func quoted(n []string) string { return talkbook.Quoted(n) }

// TalkTools returns the tools contributed by this extension.
func TalkTools() []aiapi.Tool {
	return []aiapi.Tool{
		{
			Name: "read_outline", Order: 50, Prompt: aiapi.ToolPrompt{Order: 30, Lookup: "read_outline 查大纲的卡片和情节线", Notes: outlineRules},
			Description: "读作者在大纲里排的卡片和情节线，卡片按故事里发生的先后排，可能和章节先后不同。line 按情节线名查，chapter 按章名或卷名查并附对应的纲，query 按一个词搜发生了什么、说明和故事中的时间；都不给则先给概览再列全部卡片。多个筛选参数按 line > chapter > query 只用第一个。一次最多 200 张，from 跳过前面多少张接着查。查卷时按卷里各章的先后分组，各章内仍按故事先后；#序号始终是全书故事里的位置。",
			Schema:      fixedSchema(`{"type":"object","properties":{"line":{"type":"string","description":"情节线名"},"chapter":{"type":"string","description":"章名或卷名，和搜全书结果里写的一样"},` + queryArg + `,` + fromArg + `}}`),
			Run:         runTool((*toolbox).readOutline),
			Label:       labelTool(readOutlineLabel),
		},
	}
}

func cardChapter(b talkbook.Book, id string) string {
	if n, ok := b.Tree.Node(id); ok && !n.Folder {
		return b.Name[id]
	}
	return ""
}
