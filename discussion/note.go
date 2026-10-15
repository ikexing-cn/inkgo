package discussion

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"writer/workspace"
)

// placeLabels name the places of the setting as the panel does.
var placeLabels = map[string]string{workspace.PlaceBody: "正文", workspace.PlaceOpen: "待确认", workspace.PlaceDropped: "已删除"}

// describe says what a note does, in the words of the panel's history.
func (t *toolbox) describe(in input) string {
	text := strings.TrimSpace(in.Text)
	e, _ := t.entry(in.Entry)
	switch in.Op {
	case workspace.OpAdd:
		return placeLabels[in.Place] + " · " + in.Category + "：" + text
	case workspace.OpEdit:
		return "改写「" + clip(e.Text, 16) + "」为：" + text
	case workspace.OpName:
		return "名称改为：" + strings.Join(workspace.CleanNames(in.Names), "、")
	case workspace.OpMove:
		if in.Place != "" {
			return "移到" + placeLabels[in.Place] + "：" + clip(e.Text, 24)
		}
		return "改归" + in.Category + "：" + clip(e.Text, 24)
	}
	return in.Op
}

// entry is the entry numbered n, from 1, in the order entries were added.
func (t *toolbox) entry(n int) (workspace.Entry, bool) {
	set, err := t.store.Setting()
	if err != nil || n < 1 || n > len(set.Entries) {
		return workspace.Entry{}, false
	}
	return set.Entries[n-1], true
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
//
// What breaks a rule is told to the model, which may try again.
func (t *toolbox) note(in input) outcome {
	set, err := t.store.Setting()
	if err != nil {
		return failed("读不出设定：%v", err)
	}
	events, err := t.store.Events(t.session)
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
			words, ok := locate(text, q)
			if !ok {
				continue
			}
			src := workspace.Source{Session: t.session, Message: m.ID, Quote: words}
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
		bad = append(bad, fmt.Sprintf("%s「%s」不是%s里逐字的原文；你这次回复里的话还不能引用", field, clip(q, 40), where))
		return workspace.Source{}, false
	}
	out := workspace.Op{Op: in.Op, Place: in.Place, Category: in.Category, Text: strings.TrimSpace(in.Text), Names: workspace.CleanNames(in.Names)}
	fromAuthor := false
	for _, q := range in.Sources {
		if s, ok := source(q, "sources 里的", false); ok {
			out.Sources = append(out.Sources, s)
			fromAuthor = fromAuthor || slices.ContainsFunc(said, func(m workspace.Event) bool { return m.ID == s.Message && m.Role == workspace.RoleUser })
		}
	}
	// settled takes the author's words that settle the op.
	settled := func() {
		if strings.TrimSpace(in.AuthorSaid) == "" {
			bad = append(bad, "要在 author_said 里引作者拍板、接受、拒绝或改口的原话")
		} else if s, ok := source(in.AuthorSaid, "author_said", true); ok {
			out.Sources = append(out.Sources, s)
		}
	}
	if in.Category != "" && !slices.ContainsFunc(t.store.Work().Categories, func(c workspace.Category) bool { return c.Name == in.Category }) {
		bad = append(bad, fmt.Sprintf("没有「%s」这个分类", in.Category))
	}
	if in.Place != "" && placeNames[in.Place] == "" {
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
		if in.Entry < 1 || in.Entry > len(set.Entries) {
			bad = append(bad, fmt.Sprintf("设定里没有编号 %d 的条目", in.Entry))
			break
		}
		e = set.Entries[in.Entry-1]
		out.Entry = e.ID
		switch in.Op {
		case workspace.OpEdit:
			if out.Text == "" || in.Place != "" || in.Category != "" {
				bad = append(bad, "edit 只改 text（可附 sources）")
			}
		case workspace.OpMove:
			if in.Place == "" && in.Category == "" || out.Text != "" || len(in.Sources) > 0 {
				bad = append(bad, "move 只改 place 或 category")
			}
		case workspace.OpName:
			if len(out.Names) == 0 || out.Text != "" || in.Place != "" || in.Category != "" || len(in.Sources) > 0 {
				bad = append(bad, "name 只给 names")
			} else if len(out.Names) < len(e.Names) || !slices.Equal(out.Names[:len(e.Names)], e.Names) {
				bad = append(bad, fmt.Sprintf("name 只在原有名字后面加别名，原有名字 %s 要原样排在前面", strings.Join(e.Names, "、")))
			}
		}
		if in.Op != workspace.OpName && (e.Place != workspace.PlaceOpen || e.Author || in.Place == workspace.PlaceBody || in.Place == workspace.PlaceDropped) {
			settled()
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
		case in.Op == workspace.OpMove:
			bad = append(bad, "move 不带 names")
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
	did := t.describe(in) // before it changes the entry
	r, err := t.store.Revise(workspace.Revision{By: workspace.ByTalk, Session: t.session, Turn: t.turn,
		Model: t.model.Name, Effort: string(t.model.Effort), Ops: []workspace.Op{out}})
	if err != nil {
		return failed("没有记下：%v", err)
	}
	n := in.Entry
	if in.Op == workspace.OpAdd {
		n = len(set.Entries) + 1
	}
	return outcome{text: fmt.Sprintf("已记下，编号 %d：%s", n, did), revision: r.ID}
}
