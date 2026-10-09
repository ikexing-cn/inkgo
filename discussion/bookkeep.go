package discussion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"writer/model"
	"writer/workspace"
)

const (
	bookReserve = 4096  // the answer's share of the context budget
	bookOutput  = 32768 // what the request allows, reasoning included
	maxOps      = 100   // per revision
)

// The bookkeeper's answer: ops on entries and quotes of messages, by the
// refs of its request.
type quote struct {
	Msg   string `json:"msg"`
	Quote string `json:"quote"`
}

type planOp struct {
	Op       string   `json:"op"`
	Entry    string   `json:"entry,omitempty"`
	Place    string   `json:"place,omitempty"`
	Category string   `json:"category,omitempty"`
	Text     string   `json:"text,omitempty"`
	Names    []string `json:"names,omitempty"`
	Sources  []quote  `json:"sources,omitempty"`
	Accepted *quote   `json:"accepted,omitempty"`
	Because  *quote   `json:"because,omitempty"`
}

type plan struct {
	Ops []planOp `json:"ops"`
}

// decode reads exactly one strict JSON value, allowing a ```json fence.
func decode(s string, v any) error {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json\n") && strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(strings.TrimPrefix(s, "```json\n"), "```")
	}
	d := json.NewDecoder(strings.NewReader(s))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("JSON 不符合约定：%w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("返回了多个 JSON 值或尾随内容")
	}
	return nil
}

// Bookkeeper brings the setting document up with the talk. Each run covers
// the messages of a session it has not covered yet and writes one
// revision; what the model proposes is checked against the rules below
// before anything is written.
type Bookkeeper struct {
	Store *workspace.Store
	Model Model
}

// batch is the messages of a session one revision covers, and the
// exchange before them, already covered, for what the new ones refer to:
// "the second idea" is one the reply before listed.
type batch struct {
	session string
	earlier []workspace.Event
	events  []workspace.Event
}

// earlierTurn is how many messages before a batch go with it.
const earlierTurn = 2

// next returns the messages of session not covered yet, through its last
// reply: a turn still waiting for its reply waits for it.
func (b *Bookkeeper) next(set workspace.Setting, session string) (batch, error) {
	events, err := b.Store.Events(session)
	if err != nil {
		return batch{}, err
	}
	done := set.Processed(session, events)
	last := -1
	for n, e := range events {
		if e.Role == workspace.RoleAssistant {
			last = n
		}
	}
	if last < done {
		return batch{session: session}, nil
	}
	bt := batch{session: session, earlier: events[max(0, done-earlierTurn):done], events: events[done : last+1]}
	// A backlog too long for one request goes in parts, oldest first, each
	// through a reply: the next pass takes the rest.
	for end := len(bt.events) - 2; end >= 0 && !b.fits(set, bt); end-- {
		if bt.events[end].Role == workspace.RoleAssistant {
			bt.events = bt.events[:end+1]
		}
	}
	if !b.fits(set, bt) {
		bt.earlier = nil // what was said before is a help, not a must
	}
	return bt, nil
}

// fits reports whether bt goes in one request within the work's budget.
func (b *Bookkeeper) fits(set workspace.Setting, bt batch) bool {
	payload, _, err := b.input(set, bt)
	return err == nil && Estimate(bookRules)+Estimate(payload)+bookReserve <= b.Store.Work().TokenBudget
}

// Pending lists the sessions with turns the bookkeeper has not covered.
func (b *Bookkeeper) Pending() ([]string, error) {
	set, err := b.Store.Setting()
	if err != nil {
		return nil, err
	}
	sessions, err := b.Store.Sessions()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range sessions {
		bt, err := b.next(set, s)
		if err != nil {
			return nil, err
		}
		if len(bt.events) > 0 {
			out = append(out, s)
		}
	}
	return out, nil
}

// Run makes one attempt at what session has not covered, reporting whether
// there was anything to do. A failed attempt writes nothing.
func (b *Bookkeeper) Run(ctx context.Context, session string) (bool, error) {
	set, err := b.Store.Setting()
	if err != nil {
		return false, err
	}
	bt, err := b.next(set, session)
	if err != nil || len(bt.events) == 0 {
		return false, err
	}
	r, err := b.draft(ctx, set, bt)
	if err != nil {
		return true, err
	}
	_, err = b.Store.Revise(r)
	return true, err
}

// Fail records that what session has not covered could not be taken in, so
// the turns after it go on; Redo can retry it later.
func (b *Bookkeeper) Fail(session, reason string) error {
	set, err := b.Store.Setting()
	if err != nil {
		return err
	}
	bt, err := b.next(set, session)
	if err != nil || len(bt.events) == 0 {
		return err
	}
	_, err = b.Store.Revise(workspace.Revision{By: workspace.ByBookkeeper, Session: session,
		From: bt.events[0].ID, Through: bt.events[len(bt.events)-1].ID, Failed: reason,
		Model: b.Model.Name, Effort: string(b.Model.Effort)})
	return err
}

// Redo retries the messages a failed revision covered, once.
func (b *Bookkeeper) Redo(ctx context.Context, failed string) error {
	set, err := b.Store.Setting()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(set.Revisions, func(r workspace.Revision) bool { return r.ID == failed && r.Failed != "" })
	if i < 0 {
		return errors.New("找不到这次失败的整理")
	}
	old := set.Revisions[i]
	events, err := b.Store.Events(old.Session)
	if err != nil {
		return err
	}
	from := slices.IndexFunc(events, func(e workspace.Event) bool { return e.ID == old.From })
	through := slices.IndexFunc(events, func(e workspace.Event) bool { return e.ID == old.Through })
	if from < 0 || through < from {
		return errors.New("这次整理覆盖的消息已不完整")
	}
	r, err := b.draft(ctx, set, batch{session: old.Session, earlier: events[max(0, from-earlierTurn):from], events: events[from : through+1]})
	if err != nil {
		return err
	}
	r.Redo = failed
	_, err = b.Store.Revise(r)
	return err
}

// draft asks the model for the ops that bring set up with bt, and gives it
// one chance to correct an answer that breaks the rules.
func (b *Bookkeeper) draft(ctx context.Context, set workspace.Setting, bt batch) (workspace.Revision, error) {
	if !b.Model.ready() {
		return workspace.Revision{}, errors.New("请先配置记账模型")
	}
	payload, categories, err := b.input(set, bt)
	if err != nil {
		return workspace.Revision{}, err
	}
	if Estimate(bookRules)+Estimate(payload)+bookReserve > b.Store.Work().TokenBudget {
		return workspace.Revision{}, errors.New("设定文档和这一轮对话超出整理预算，请提高作品预算")
	}
	req := model.Request{System: bookRules, Messages: []model.Message{{Role: workspace.RoleUser, Content: payload}}, MaxTokens: bookOutput}
	answer, err := ask(ctx, b.Model, req, nil)
	if err != nil {
		return workspace.Revision{}, err
	}
	reasoning := answer.Reasoning
	ops, problems := b.check(answer.Text, set, bt, categories)
	if len(problems) > 0 {
		req.Messages = append(req.Messages,
			model.Message{Role: workspace.RoleAssistant, Content: answer.Text},
			model.Message{Role: workspace.RoleUser, Content: "上面的回答有这些问题：\n" + strings.Join(problems, "\n") + "\n请改正后重新给出完整的 JSON。"})
		if answer, err = ask(ctx, b.Model, req, nil); err != nil {
			return workspace.Revision{}, err
		}
		if answer.Reasoning != "" {
			reasoning = strings.TrimSpace(reasoning + "\n\n（改正时）\n" + answer.Reasoning)
		}
		if ops, problems = b.check(answer.Text, set, bt, categories); len(problems) > 0 {
			return workspace.Revision{}, fmt.Errorf("整理结果不合规则：%s", strings.Join(problems, "；"))
		}
	}
	return workspace.Revision{By: workspace.ByBookkeeper, Ops: ops, Base: len(set.Revisions), Session: bt.session,
		From: bt.events[0].ID, Through: bt.events[len(bt.events)-1].ID,
		Model: b.Model.Name, Effort: string(b.Model.Effort), Reasoning: reasoning}, nil
}

// input is what the bookkeeper is shown of set and bt, and the categories
// it may use.
func (b *Bookkeeper) input(set workspace.Setting, bt batch) (string, []string, error) {
	w := b.Store.Work()
	var categories []string
	for _, c := range w.Categories {
		categories = append(categories, c.Name)
	}
	type shownEntry struct {
		Ref      string   `json:"ref"`
		Place    string   `json:"place"`
		Category string   `json:"category"`
		Text     string   `json:"text"`
		Names    []string `json:"names,omitempty"`
		Author   bool     `json:"author,omitempty"`
	}
	type shownMessage struct {
		Ref     string `json:"ref"`
		Who     string `json:"who"`
		Earlier bool   `json:"earlier,omitempty"`
		Text    string `json:"text"`
	}
	in := struct {
		Categories []string       `json:"categories"`
		Setting    []shownEntry   `json:"setting"`
		Messages   []shownMessage `json:"messages"`
	}{Categories: categories, Setting: []shownEntry{}}
	for n, e := range set.Entries {
		in.Setting = append(in.Setting, shownEntry{fmt.Sprintf("e%d", n+1), e.Place, e.Category, e.Text, e.Names, e.Author})
	}
	for n, e := range bt.messages() {
		who := "ai"
		if e.Role == workspace.RoleUser {
			who = "author"
		}
		in.Messages = append(in.Messages, shownMessage{fmt.Sprintf("m%d", n+1), who, n < len(bt.earlier), e.Said()})
	}
	payload, err := json.Marshal(in)
	return string(payload), categories, err
}

// check turns an answer into ops, or says which rules it breaks:
//
//  1. Every quote is verbatim in the message it names.
//  2. Into the body goes what the author said, or what the AI said with the
//     author's words accepting it; into dropped, what the author refused.
//  3. What the author settled, the body and dropped and any entry the author
//     touched, changes only on the author's words, quoted as because.
//  4. Categories are the work's.
//  5. Names are given only to entries of NameCategories, each is said in
//     the entry or the talk shown, and is no other entry's. The name op
//     only adds names after an entry's, so needs no because: another name
//     settles nothing.
func (b *Bookkeeper) check(text string, set workspace.Setting, bt batch, categories []string) ([]workspace.Op, []string) {
	var p plan
	if err := decode(text, &p); err != nil {
		return nil, []string{err.Error()}
	}
	if p.Ops == nil {
		return nil, []string{"缺少 ops 数组"}
	}
	if len(p.Ops) > maxOps {
		return nil, []string{fmt.Sprintf("一次最多 %d 个操作", maxOps)}
	}
	shown := bt.messages()
	message := func(ref string) (workspace.Event, bool) {
		var n int
		if _, err := fmt.Sscanf(ref, "m%d", &n); err != nil || n < 1 || n > len(shown) || ref != fmt.Sprintf("m%d", n) {
			return workspace.Event{}, false
		}
		return shown[n-1], true
	}
	var ops []workspace.Op
	var problems []string
	for k, op := range p.Ops {
		var bad []string
		// source checks q and, for authorOnly, that the author said it.
		source := func(q *quote, field string, authorOnly bool) (workspace.Source, bool) {
			if q == nil {
				bad = append(bad, field+" 缺失")
				return workspace.Source{}, false
			}
			m, ok := message(q.Msg)
			said := m.Said()
			words, found := locate(said, q.Quote)
			switch {
			case !ok:
				bad = append(bad, fmt.Sprintf("%s 指向不存在的消息 %q", field, q.Msg))
			case !found:
				bad = append(bad, fmt.Sprintf("%s 的 quote「%s」不是 %s 中逐字的原文", field, clip(q.Quote, 40), q.Msg))
			case authorOnly && m.Role != workspace.RoleUser:
				bad = append(bad, fmt.Sprintf("%s 必须引用作者（author）的原话", field))
			default:
				src := workspace.Source{Session: bt.session, Message: m.ID, Quote: words}
				if i := strings.Index(said, words); i >= 0 {
					start := utf8.RuneCountInString(said[:i])
					if qt, ok := m.QuoteAt(start, start+utf8.RuneCountInString(words)); ok {
						src.Chapter = qt.Chapter
					}
				}
				return src, true
			}
			return workspace.Source{}, false
		}
		var sources []workspace.Source
		fromAuthor := false
		for _, q := range op.Sources {
			if s, ok := source(&q, "sources", false); ok {
				sources = append(sources, s)
				if m, _ := message(q.Msg); m.Role == workspace.RoleUser {
					fromAuthor = true
				}
			}
		}
		if op.Category != "" && !slices.Contains(categories, op.Category) {
			bad = append(bad, fmt.Sprintf("未知分类 %q", op.Category))
		}
		out := workspace.Op{Op: op.Op, Place: op.Place, Category: op.Category, Text: strings.TrimSpace(op.Text),
			Names: workspace.CleanNames(op.Names), Sources: sources}
		switch op.Op {
		case workspace.OpAdd:
			if op.Entry != "" {
				bad = append(bad, "add 不带 entry")
			}
			if out.Text == "" || op.Category == "" || len(op.Sources) == 0 {
				bad = append(bad, "add 需要 text、category 和 sources")
			}
			switch op.Place {
			case workspace.PlaceBody:
				if !fromAuthor {
					if s, ok := source(op.Accepted, "accepted", true); ok {
						out.Sources = append(out.Sources, s)
					}
				}
			case workspace.PlaceDropped:
				if s, ok := source(op.Because, "because", true); ok {
					out.Sources = append(out.Sources, s)
				}
			case workspace.PlaceOpen:
			default:
				bad = append(bad, fmt.Sprintf("未知位置 %q", op.Place))
			}
		case workspace.OpEdit, workspace.OpMove, workspace.OpName:
			var n int
			if _, err := fmt.Sscanf(op.Entry, "e%d", &n); err != nil || n < 1 || n > len(set.Entries) || op.Entry != fmt.Sprintf("e%d", n) {
				bad = append(bad, fmt.Sprintf("entry %q 不在 setting 里", op.Entry))
				break
			}
			e := set.Entries[n-1]
			out.Entry = e.ID
			if op.Op == workspace.OpEdit && (out.Text == "" || op.Place != "" || op.Category != "") {
				bad = append(bad, "edit 只改 text（可附 sources）")
			}
			if op.Op == workspace.OpMove && ((op.Place == "" && op.Category == "") || out.Text != "" || len(op.Sources) > 0) {
				bad = append(bad, "move 只改 place 或 category")
			}
			if op.Op == workspace.OpName {
				if len(out.Names) == 0 || out.Text != "" || op.Place != "" || op.Category != "" || len(op.Sources) > 0 {
					bad = append(bad, "name 只给 names")
				} else if len(out.Names) < len(e.Names) || !slices.Equal(out.Names[:len(e.Names)], e.Names) {
					bad = append(bad, fmt.Sprintf("name 只在原有名字后面加别名，%s 的原有名字 %s 要原样排在前面", op.Entry, strings.Join(e.Names, "、")))
				}
				break // adding another name changes nothing settled
			}
			if op.Place != "" && op.Place != workspace.PlaceBody && op.Place != workspace.PlaceOpen && op.Place != workspace.PlaceDropped {
				bad = append(bad, fmt.Sprintf("未知位置 %q", op.Place))
			}
			if e.Place != workspace.PlaceOpen || e.Author || op.Place == workspace.PlaceBody || op.Place == workspace.PlaceDropped {
				if s, ok := source(op.Because, "because", true); ok {
					out.Sources = append(out.Sources, s)
				}
			}
		default:
			bad = append(bad, fmt.Sprintf("未知操作 %q", op.Op))
		}
		if len(out.Names) > 0 {
			category := op.Category
			if e, ok := set.Entry(out.Entry); ok && category == "" {
				category = e.Category
			}
			switch {
			case op.Op == workspace.OpMove:
				bad = append(bad, "move 不带 names")
			case !slices.Contains(workspace.NameCategories, category):
				bad = append(bad, fmt.Sprintf("只有%s分类的条目带 names", strings.Join(workspace.NameCategories, "、")))
			default:
				text := out.Text
				if e, ok := set.Entry(out.Entry); ok && text == "" {
					text = e.Text
				}
				for _, name := range out.Names {
					if !saidIn(name, text, shown) {
						bad = append(bad, fmt.Sprintf("名字「%s」没有在条目或对话里出现过", name))
					}
					if i := slices.IndexFunc(set.Entries, func(e workspace.Entry) bool {
						return e.ID != out.Entry && e.Place != workspace.PlaceDropped && slices.Contains(e.Names, name)
					}); i >= 0 {
						bad = append(bad, fmt.Sprintf("名字「%s」已经在 e%d 里，一个名字只放在一条里", name, i+1))
					}
				}
			}
		}
		if len(bad) > 0 {
			problems = append(problems, fmt.Sprintf("第 %d 个操作：%s", k+1, strings.Join(bad, "；")))
			continue
		}
		ops = append(ops, out)
	}
	return ops, problems
}

// saidIn reports whether name is in text or in a message shown.
func saidIn(name, text string, shown []workspace.Event) bool {
	if strings.Contains(text, name) {
		return true
	}
	return slices.ContainsFunc(shown, func(m workspace.Event) bool { return strings.Contains(m.Said(), name) })
}

// clip shortens s to n runes for a message.
func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// messages are the batch's messages after the earlier ones, as the model
// sees them.
func (bt batch) messages() []workspace.Event {
	return append(slices.Clone(bt.earlier), bt.events...)
}

// copyMarks are what a model drops or adds copying words: spaces, Markdown
// marks and quotation marks.
const copyMarks = "*_`#>\"'“”‘’「」"

// locate finds quote in text verbatim or, failing that, ignoring copyMarks,
// and returns the text's own words for it.
func locate(text, quote string) (string, bool) {
	skip := func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune(copyMarks, r) }
	q := []rune(strings.Map(func(r rune) rune {
		if skip(r) {
			return -1
		}
		return r
	}, quote))
	if len(q) == 0 {
		return "", false // no words to find
	}
	if strings.Contains(text, quote) {
		return quote, true
	}
	var kept []rune
	var at []int // the byte offset in text of each rune kept
	for i, r := range text {
		if !skip(r) {
			kept, at = append(kept, r), append(at, i)
		}
	}
	for i := 0; i+len(q) <= len(kept); i++ {
		if slices.Equal(kept[i:i+len(q)], q) {
			last := i + len(q) - 1
			return text[at[i] : at[last]+utf8.RuneLen(kept[last])], true
		}
	}
	return "", false
}
