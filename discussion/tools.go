package discussion

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

// The tools a talk's model may call: four look things up in the book and
// its talks, and one notes the setting. The model names a chapter by its
// title and an entry by its number, never by an ID.
const (
	toolSearchBook  = "search_book"
	toolReadProse   = "read_prose"
	toolLookUp      = "look_up_setting"
	toolSearchTalks = "search_talks"
	toolNote        = "note_setting"
)

const (
	maxHits = 20   // listed by one search
	maxRead = 3000 // runes of prose one read returns
	maxNear = 5    // names offered for one the book has not
	around  = 40   // runes shown on each side of a hit
)

// tools are the tools of a talk of w: note_setting takes w's categories.
func tools(w workspace.Work) []model.Tool {
	var categories []string
	for _, c := range w.Categories {
		categories = append(categories, c.Name)
	}
	cats, _ := json.Marshal(categories)
	query := `"query":{"type":"string","description":"要找的词，越具体越好，比如人名、地名、物件"}`
	from := `"from":{"type":"integer","minimum":0,"description":"跳过前面多少处，接着上次列到的地方往下看"}`
	return []model.Tool{
		{Name: toolSearchBook, Description: "按词搜全书的正文，返回每处所在的章、段号和前后的文字。",
			Schema: json.RawMessage(`{"type":"object","properties":{` + query + `,` + from + `},"required":["query"]}`)},
		{Name: toolReadProse, Description: "读某一章的正文，段号从 1 起；不给段号就从头读。一次读到的字数有上限，读不完会说从哪里接着读。",
			Schema: json.RawMessage(`{"type":"object","properties":{"chapter":{"type":"string","description":"章名，和搜全书结果里写的一样"},"from":{"type":"integer","minimum":1},"to":{"type":"integer","minimum":1}},"required":["chapter"]}`)},
		{Name: toolLookUp, Description: "按名字或词查设定，返回条目的编号、位置、分类、名字、全文和出处。",
			Schema: json.RawMessage(`{"type":"object","properties":{` + query + `},"required":["query"]}`)},
		{Name: toolSearchTalks, Description: "按词搜这本书所有讨论的原文，包括这一次的，返回每处在哪次讨论、是谁说的和前后的文字。",
			Schema: json.RawMessage(`{"type":"object","properties":{` + query + `,` + from + `},"required":["query"]}`)},
		{Name: toolNote, Description: "把这次讨论里定下的东西记进设定文档，一次一个操作：add 新增，edit 改写，move 换位置或分类，name 补名字。规则见系统提示里的「记设定」。",
			Schema: json.RawMessage(`{"type":"object","properties":{` +
				`"op":{"type":"string","enum":["add","edit","move","name"]},` +
				`"entry":{"type":"integer","minimum":1,"description":"edit、move、name 改的条目的编号，即设定里方括号中的数字"},` +
				`"place":{"type":"string","enum":["body","open","dropped"],"description":"body 是设定正文，open 是悬而未决，dropped 是已放弃"},` +
				`"category":{"type":"string","enum":` + string(cats) + `},` +
				`"text":{"type":"string","description":"一句能独立成立的设定"},` +
				`"names":{"type":"array","items":{"type":"string"},"description":"条目说的专有名词，第一个是正式名称"},` +
				`"sources":{"type":"array","items":{"type":"string"},"description":"这条出自的原话，逐字复制自本次讨论里已经说过的话"},` +
				`"author_said":{"type":"string","description":"作者拍板、接受、拒绝或改口的原话，逐字复制"}` +
				`},"required":["op"]}`)},
	}
}

// toolbox runs the calls of a reply to the author's words turn in session.
type toolbox struct {
	store   *workspace.Store
	session string
	turn    string
	model   Model
}

// outcome is what a call returns to the model, whether it failed, and the
// revision a note wrote.
type outcome struct {
	text     string
	failed   bool
	revision string
}

func failed(format string, v ...any) outcome {
	return outcome{text: fmt.Sprintf(format, v...), failed: true}
}

// input is what the tools take, all of them together.
type input struct {
	Query      string   `json:"query"`
	From       int      `json:"from"`
	To         int      `json:"to"`
	Chapter    string   `json:"chapter"`
	Chapters   []string `json:"chapters"`
	Op         string   `json:"op"`
	Entry      int      `json:"entry"`
	Place      string   `json:"place"`
	Category   string   `json:"category"`
	Text       string   `json:"text"`
	Names      []string `json:"names"`
	Sources    []string `json:"sources"`
	AuthorSaid string   `json:"author_said"`
}

func read(c model.Call) (input, error) {
	var in input
	err := json.Unmarshal(c.Input, &in)
	in.Query = strings.TrimSpace(in.Query)
	in.Chapter = strings.TrimSpace(in.Chapter)
	return in, err
}

// run runs c; what fails is told to the model, which may try again.
func (t *toolbox) run(c model.Call) outcome {
	in, err := read(c)
	if err != nil {
		return failed("参数不是约定的 JSON：%v", err)
	}
	switch c.Name {
	case toolSearchBook:
		return t.searchBook(in)
	case toolReadProse:
		return t.readProse(in)
	case toolLookUp:
		return t.lookUp(in)
	case toolSearchTalks:
		return t.searchTalks(in)
	case toolNote:
		return t.note(in)
	}
	return failed("没有叫 %s 的工具", c.Name)
}

// label says what c does in the author's words, as the reply shows it,
// naming each chapter as the book does.
func (t *toolbox) label(c model.Call) string {
	in, _ := read(c)
	if b, err := t.book(); err == nil {
		in.Chapter = b.named(in.Chapter)
		for k, name := range in.Chapters {
			in.Chapters[k] = b.named(name)
		}
	}
	switch c.Name {
	case toolSearchBook:
		return "搜了全书「" + in.Query + "」"
	case toolReadProse:
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
	case "read_summary": // a tool of older versions, in the talks they had
		return "读了摘要" + quoted(in.Chapters)
	case toolLookUp:
		return "查了设定「" + in.Query + "」"
	case toolSearchTalks:
		return "搜了讨论「" + in.Query + "」"
	case toolNote:
		return "记下：" + t.describe(in)
	}
	return c.Name
}

// quoted is names each in 「」.
func quoted(names []string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString("「" + strings.TrimSpace(n) + "」")
	}
	return b.String()
}

// book is the documents of the work in reading order, each named for the
// model: by its title, or with its volume's before it when two share one.
type book struct {
	tree   workspace.Tree
	docs   []*workspace.Node
	name   map[string]string
	names  []string // each once, in reading order
	byName map[string]*workspace.Node
}

func (t *toolbox) book() (book, error) {
	tree, err := t.store.Chapters()
	if err != nil {
		return book{}, err
	}
	b := book{tree: tree, name: map[string]string{}, byName: map[string]*workspace.Node{}}
	count := map[string]int{}
	tree.Walk(func(n *workspace.Node, _ int) {
		count[n.Title]++
		if !n.Folder {
			b.docs = append(b.docs, n)
		}
	})
	tree.Walk(func(n *workspace.Node, _ int) {
		name := n.Title
		if count[name] > 1 && n.Parent != nil && n.Parent.Title != "" {
			name = n.Parent.Title + "/" + name
		}
		b.name[n.ID] = name
		if _, taken := b.byName[name]; !taken {
			b.byName[name] = n
			b.names = append(b.names, name)
		}
	})
	return b, nil
}

// find is the document the model names, taking its name, in 「」 or 《》
// or not, or else the one name that is it but for spaces and marks, or
// that begins with it, or that has it. Finding none, or more than one, it
// says so for the model, with the names it may mean.
func (b book) find(name string) (*workspace.Node, string) {
	name = strings.Trim(strings.TrimSpace(name), "「」《》")
	if n, ok := b.byName[name]; ok {
		return n, ""
	}
	key := loose(name)
	var same, begins, has []string
	for _, s := range b.names {
		switch k := loose(s); {
		case key == "":
		case k == key:
			same = append(same, s)
		case strings.HasPrefix(k, key):
			begins = append(begins, s)
		case strings.Contains(k, key):
			has = append(has, s)
		}
	}
	for _, found := range [][]string{same, begins, has} {
		switch {
		case len(found) == 1:
			return b.byName[found[0]], ""
		case len(found) > 1:
			return nil, "「" + name + "」对得上好几章：" + quoted(found[:min(len(found), maxNear)]) + "；写全章名。"
		}
	}
	if near := b.near(key); len(near) > 0 {
		return nil, "没有叫「" + name + "」的章；相近的有" + quoted(near) + "。"
	}
	return nil, "没有叫「" + name + "」的章；章名要和搜全书的结果里写的一样。"
}

// named is the name of the document the model names, or what it said
// where that finds none.
func (b book) named(name string) string {
	if n, _ := b.find(name); n != nil {
		return b.name[n.ID]
	}
	return strings.TrimSpace(name)
}

// near is up to maxNear names that share a run of two characters or more
// with key, the longest run first.
func (b book) near(key string) []string {
	type close struct {
		name string
		run  int
	}
	var found []close
	for _, s := range b.names {
		if run := common([]rune(loose(s)), []rune(key)); run >= 2 {
			found = append(found, close{s, run})
		}
	}
	slices.SortStableFunc(found, func(x, y close) int { return y.run - x.run })
	var out []string
	for _, c := range found[:min(len(found), maxNear)] {
		out = append(out, c.name)
	}
	return out
}

// loose is s without its spaces and marks, as a model may give a name in
// other words.
func loose(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// common is how long the longest run is that a and b share.
func common(a, b []rune) int {
	best, prev := 0, make([]int, len(b)+1)
	for _, x := range a {
		cur := make([]int, len(b)+1)
		for k, y := range b {
			if x == y {
				cur[k+1] = prev[k] + 1
				best = max(best, cur[k+1])
			}
		}
		prev = cur
	}
	return best
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
		return clip(text, 2*around)
	}
	before, after := []rune(text[:i]), []rune(text[i+len(query):])
	s := query
	if len(before) > around {
		s = "…" + string(before[len(before)-around:]) + s
	} else {
		s = string(before) + s
	}
	if len(after) > around {
		s += string(after[:around]) + "…"
	} else {
		s += string(after)
	}
	return s
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

func (t *toolbox) searchBook(in input) outcome {
	if in.Query == "" {
		return failed("query 不能为空")
	}
	b, err := t.book()
	if err != nil {
		return failed("读不出章节：%v", err)
	}
	var found []hit
	for _, n := range b.docs {
		ch, err := t.store.ReadChapter(n.ID)
		if err != nil {
			return failed("读不出「%s」：%v", b.name[n.ID], err)
		}
		for k, p := range paragraphs(ch.Text) {
			if strings.Contains(p, in.Query) {
				found = append(found, hit{b.name[n.ID], fmt.Sprintf("第 %d 段：%s", k+1, snippet(p, in.Query))})
			}
		}
	}
	return outcome{text: hits(found, in.From, "全书的正文里没有「"+in.Query+"」。")}
}

func (t *toolbox) readProse(in input) outcome {
	b, err := t.book()
	if err != nil {
		return failed("读不出章节：%v", err)
	}
	n, missing := b.find(in.Chapter)
	if n == nil {
		return failed("%s", missing)
	}
	if n.Folder {
		return failed("「%s」是一卷，没有正文；读它下面的章。", b.name[n.ID])
	}
	ch, err := t.store.ReadChapter(n.ID)
	if err != nil {
		return failed("读不出「%s」：%v", b.name[n.ID], err)
	}
	ps := paragraphs(ch.Text)
	if len(ps) == 0 {
		return outcome{text: "「" + b.name[n.ID] + "」还没有正文。"}
	}
	from, to := max(in.From, 1), in.To
	if to == 0 || to > len(ps) {
		to = len(ps)
	}
	if from > len(ps) || to < from {
		return failed("「%s」共 %d 段，没有第 %d–%d 段。", b.name[n.ID], len(ps), from, to)
	}
	lines := []string{fmt.Sprintf("「%s」共 %d 段。", b.name[n.ID], len(ps))}
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

// placeNames name the places of the setting as the talk's rules do.
var placeNames = map[string]string{workspace.PlaceBody: "设定正文", workspace.PlaceOpen: "悬而未决", workspace.PlaceDropped: "已放弃"}

func (t *toolbox) lookUp(in input) outcome {
	if in.Query == "" {
		return failed("query 不能为空")
	}
	set, err := t.store.Setting()
	if err != nil {
		return failed("读不出设定：%v", err)
	}
	titles, chapters := t.titles(), t.chapterNames()
	var named, said []string
	for n, e := range set.Entries {
		if !slices.ContainsFunc(e.Names, func(name string) bool { return strings.Contains(name, in.Query) || strings.Contains(in.Query, name) }) &&
			!strings.Contains(prompt.Text(e), in.Query) {
			continue
		}
		head := fmt.Sprintf("[%d]（%s｜%s", n+1, placeNames[e.Place], e.Category)
		if len(e.Names) > 0 {
			head += "｜" + strings.Join(e.Names, "、")
		}
		lines := []string{head + "）" + prompt.Text(e)}
		for _, src := range e.Sources[:min(len(e.Sources), 3)] {
			where := ""
			switch {
			case src.Message != "":
				where = "讨论「" + titles[src.Session] + "」"
			case src.Chapter != "":
				where = "正文「" + chapters[src.Chapter] + "」"
			}
			lines = append(lines, "  出处：「"+clip(prompt.OneLine(src.Quote), 80)+"」"+where)
		}
		if slices.ContainsFunc(e.Names, func(name string) bool { return strings.Contains(name, in.Query) || strings.Contains(in.Query, name) }) {
			named = append(named, strings.Join(lines, "\n"))
		} else {
			said = append(said, strings.Join(lines, "\n"))
		}
	}
	found := append(named, said...)
	switch {
	case len(found) == 0:
		return outcome{text: "设定里没有「" + in.Query + "」。"}
	case len(found) > maxHits:
		return outcome{text: strings.Join(found[:maxHits], "\n") + fmt.Sprintf("\n还有 %d 条没有列出，换一个更具体的词再查。", len(found)-maxHits)}
	}
	return outcome{text: strings.Join(found, "\n")}
}

// chapterNames are the names of the documents, by ID; what fails to read
// names none.
func (t *toolbox) chapterNames() map[string]string {
	b, err := t.book()
	if err != nil {
		return nil
	}
	return b.name
}

// titles name the talks of the work by what the author said first.
func (t *toolbox) titles() map[string]string {
	out := map[string]string{}
	sessions, _ := t.store.Sessions()
	for _, s := range sessions {
		events, _ := t.store.Events(s)
		out[s] = talkTitle(events)
		if s == t.session {
			out[s] = "这一次的讨论"
		}
	}
	return out
}

// talkTitle names a talk by what the author said first, as the talks
// list does.
func talkTitle(events []workspace.Event) string {
	for _, e := range events {
		if e.Role == workspace.RoleUser {
			return clip(prompt.OneLine(e.Text), 14)
		}
	}
	return "新对话"
}

func (t *toolbox) searchTalks(in input) outcome {
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
