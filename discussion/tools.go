package discussion

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"writer/aiport"

	"writer/inference"
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
	toolLookUp      = "look_up_setting"
	toolSearchTalks = "search_talks"
	toolReadOutline = "read_outline"
	toolNote        = "note_setting"
)

const (
	maxHits  = 20    // listed by one search
	maxRead  = 10000 // runes of prose one read returns
	maxNear  = 5     // names offered for one the book has not
	maxCards = 200   // cards listed by one outline read
	around   = 40    // runes shown on each side of a hit
)

// queryArg and fromArg are the arguments search_book and search_talks share.
const (
	queryArg = `"query":{"type":"string","description":"要找的词，越具体越好，比如人名、地名、物件"}`
	fromArg  = `"from":{"type":"integer","minimum":0,"description":"跳过前面多少处，接着上次列到的地方往下看"}`
)

// tool is one of a talk's tools: what the model is told, how a call runs,
// and how the call reads to the author.
type tool struct {
	name, description string
	schema            func(workspace.Work) json.RawMessage
	run               func(*toolbox, model.Call) outcome
	label             func(*labelReader, model.Call) string
}

// toolTable is every tool of a talk, in the order the model is given them.
var toolTable = []tool{
	{
		name:        toolSearchBook,
		description: "按词搜全书的正文，返回每处所在的章、段号和前后的文字。",
		schema:      fixedSchema(`{"type":"object","properties":{` + queryArg + `,` + fromArg + `},"required":["query"]}`),
		run:         runTool((*toolbox).searchBook),
		label:       labelTool(searchBookLabel),
	},
	{
		name:        toolReadProse,
		description: "读某一章的正文，段号从 1 起；不给段号就从头读。一次读到的字数有上限，读不完会说从哪里接着读。",
		schema:      fixedSchema(`{"type":"object","properties":{"chapter":{"type":"string","description":"章名，和搜全书结果里写的一样"},"from":{"type":"integer","minimum":1},"to":{"type":"integer","minimum":1}},"required":["chapter"]}`),
		run:         runTool((*toolbox).readProse),
		label:       labelTool(readProseLabel),
	},
	{
		name:        toolLookUp,
		description: "按名字或词查设定，返回条目的编号、位置、分类、名字、全文和出处。",
		schema:      fixedSchema(`{"type":"object","properties":{` + queryArg + `},"required":["query"]}`),
		run:         runTool((*toolbox).lookUp),
		label:       labelTool(lookUpLabel),
	},
	{
		name:        toolSearchTalks,
		description: "按词搜这本书所有讨论的原文，包括这一次的，返回每处在哪次讨论、是谁说的和前后的文字。",
		schema:      fixedSchema(`{"type":"object","properties":{` + queryArg + `,` + fromArg + `},"required":["query"]}`),
		run:         runTool((*toolbox).searchTalks),
		label:       labelTool(searchTalksLabel),
	},
	{
		name:        toolReadOutline,
		description: "读作者在大纲里排的卡片和情节线，卡片按故事里发生的先后排，可能和章节先后不同。line 按情节线名查，chapter 按章名或卷名查并附对应的纲，query 按一个词搜发生了什么、说明和故事中的时间；都不给则先给概览再列全部卡片。多个筛选参数按 line > chapter > query 只用第一个。一次最多 200 张，from 跳过前面多少张接着查。查卷时按卷里各章的先后分组，各章内仍按故事先后；#序号始终是全书故事里的位置。",
		schema:      fixedSchema(`{"type":"object","properties":{"line":{"type":"string","description":"情节线名"},"chapter":{"type":"string","description":"章名或卷名，和搜全书结果里写的一样"},` + queryArg + `,` + fromArg + `}}`),
		run:         runTool((*toolbox).readOutline),
		label:       labelTool(readOutlineLabel),
	},
	{
		name:        toolNote,
		description: "把这次讨论里定下的东西记进设定文档，一次一个操作：add 新增，edit 改写，move 换位置或分类，name 补名字，change 记一条从某一章起的变化。规则见系统提示里的「记设定」。",
		schema:      noteSchema,
		run:         runTool((*toolbox).note),
		label:       labelTool(noteLabel),
	},
	{
		name:        toolPropose,
		description: "在正文里提议一处修改，显示在那一章里，作者采纳了才写进正文。一次提议一处。参数按 chapter、at、quote、text 的次序写。",
		schema:      fixedSchema(`{"type":"object","properties":{"chapter":{"type":"string","description":"改哪一章，章名和搜全书结果里写的一样"},"at":{"type":"string","enum":["replace","after","end","all"],"description":"replace 换掉 quote 那段原文；after 接在 quote 之后，在同一段里接着写，text 以换行开头就另起一段；end 接在章末；all 换掉全章"},"quote":{"type":"string","description":"at 是 replace 或 after 时，逐字复制那一章里的原文，在那一章里只出现一次"},"text":{"type":"string","description":"新写的正文，一段一行"}},"required":["chapter","at","text"]}`),
		run:         runTool((*toolbox).propose),
		label:       labelTool(proposeLabel),
	},
}

// fixedSchema is schema for every work.
func fixedSchema(schema string) func(workspace.Work) json.RawMessage {
	return func(workspace.Work) json.RawMessage { return json.RawMessage(schema) }
}

// tools are the tools of a talk of w: note_setting takes w's categories.
func tools(w workspace.Work) []model.Tool {
	return toolsFor(w, aiport.Material{Setting: true, Outline: true})
}

func toolsFor(w workspace.Work, material aiport.Material) []model.Tool {
	var out []model.Tool
	for _, tl := range toolTable {
		if (tl.name == toolLookUp || tl.name == toolNote) && !material.Setting || tl.name == toolReadOutline && !material.Outline {
			continue
		}
		out = append(out, model.Tool{Name: tl.name, Description: tl.description, Schema: tl.schema(w)})
	}
	return out
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

// arguments is a tool's own arguments, trimmed as the talk reads them.
type arguments[T any] interface {
	trim() T
}

// runTool adapts a tool's typed handler: it decodes the call into T and,
// what does not decode, tells the model, which may try again.
func runTool[T arguments[T]](run func(*toolbox, T) outcome) func(*toolbox, model.Call) outcome {
	return func(t *toolbox, c model.Call) outcome {
		var in T
		err := json.Unmarshal(c.Input, &in)
		in = in.trim()
		if err != nil {
			return failed("参数不是约定的 JSON：%v", err)
		}
		return run(t, in)
	}
}

// labelTool adapts a tool's typed label; a call that does not decode is
// read as far as it came, since labels ignore that error.
func labelTool[T arguments[T]](label func(*labelReader, T) string) func(*labelReader, model.Call) string {
	return func(r *labelReader, c model.Call) string {
		var in T
		json.Unmarshal(c.Input, &in)
		return label(r, in.trim())
	}
}

// findTool is the tool named, if there is one.
func findTool(name string) (tool, bool) {
	for _, tl := range toolTable {
		if tl.name == name {
			return tl, true
		}
	}
	return tool{}, false
}

// toolbox runs the calls of a reply to the author's words turn in session.
// In a talk that keeps to the chapter at, the setting is looked up as it
// stands there.
type toolbox struct {
	allowed []model.Tool
	store   *workspace.Store
	session string
	turn    string
	model   inference.Model
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
	tl, ok := findTool(c.Name)
	if !ok || !slices.ContainsFunc(t.allowed, func(tl model.Tool) bool { return tl.Name == c.Name }) {
		return failed("没有叫 %s 的工具", c.Name)
	}
	return tl.run(t, c)
}

// label says what c does in the author's words, as the reply shows it,
// naming each chapter as the book does.
func (t *toolbox) label(c model.Call) string {
	return t.labels([]model.Call{c})[0].Label
}

// labels says what calls do in the author's words, as the reply shows
// them, naming each chapter as the book does.
func (t *toolbox) labels(calls []model.Call) []workspace.Call {
	r := &labelReader{t: t}
	out := make([]workspace.Call, len(calls))
	for i, c := range calls {
		label := c.Name
		if tl, ok := findTool(c.Name); ok {
			label = tl.label(r, c)
		}
		out[i] = workspace.Call{ID: c.ID, Name: c.Name, Input: c.Input, Label: label}
	}
	return out
}

// labelReader reads what labels name at most once, and only when asked:
// the book for a chapter, the setting for a note that edits or moves.
type labelReader struct {
	t       *toolbox
	book    book
	hasBook bool
	set     workspace.Setting
	hasSet  bool
}

// named is the name of the document the model names, or what it said
// where that finds none.
func (r *labelReader) named(chapter string) string {
	if chapter == "" {
		return ""
	}
	if !r.hasBook {
		r.hasBook = true
		r.book, _ = r.t.book()
	}
	if r.book.name == nil {
		return chapter
	}
	return r.book.named(chapter)
}

func (r *labelReader) setting() workspace.Setting {
	if !r.hasSet {
		r.hasSet = true
		r.set, _ = r.t.store.Setting()
	}
	return r.set
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
	return bookOf(tree), nil
}

// bookOf is tree as the model names its nodes.
func bookOf(tree workspace.Tree) book {
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
	return b
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
	for _, n := range b.docs {
		for k, p := range paragraphs(texts[n.ID]) {
			if strings.Contains(p, in.Query) {
				found = append(found, hit{b.name[n.ID], fmt.Sprintf("第 %d 段：%s", k+1, snippet(p, in.Query))})
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

// placeNames name the places of the setting as the talk's rules do.
var placeNames = map[string]string{workspace.PlaceBody: "设定正文", workspace.PlaceOpen: "悬而未决", workspace.PlaceDropped: "已放弃"}

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
	set, err := t.store.Setting()
	if err != nil {
		return failed("读不出设定：%v", err)
	}
	b, err := t.book()
	if err != nil {
		return failed("读不出章节：%v", err)
	}
	titles := map[string]string{}
	title := func(session string) string {
		if name, ok := titles[session]; ok {
			return name
		}
		events, _ := t.store.Events(session)
		name := talkTitle(events)
		if session == t.session {
			name = "这一次的讨论"
		}
		titles[session] = name
		return name
	}
	return lookupSetting(in.Query, set, b, t.at, title)
}

func lookUpLabel(_ *labelReader, in lookUpArgs) string {
	return "查了设定「" + in.Query + "」"
}

func lookupSetting(query string, set workspace.Setting, b book, at string, title func(string) string) outcome {
	chapters, later := b.name, changes(set, b.tree)
	if at != "" {
		set, _ = set.At(b.tree.Order(), at)
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
		head := workspace.EntryRef(numbers[e.ID]) + fmt.Sprintf("（%s｜%s", placeNames[e.Place], e.Category)
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
		return outcome{text: "设定里没有「" + query + "」。"}
	case len(matched) > maxHits:
		return outcome{text: strings.Join(found, "\n") + fmt.Sprintf("\n还有 %d 条没有列出，换一个更具体的词再查。", len(matched)-maxHits)}
	}
	return outcome{text: strings.Join(found, "\n")}
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
