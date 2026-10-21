// Package discussion runs the talk with the author: a reply looks things
// up in the book with tools and keeps the setting document up with the
// talk.
package discussion

import (
	"errors"
	"fmt"
	"slices"

	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

// Context is what one talk request sends, the entries of the setting and
// the prose it showed, and what each part took; Left is the room left for
// what the tools return.
type Context struct {
	Request  model.Request
	Included []workspace.ContextItem
	Sent     *workspace.Sent
	Left     int
}

// talkPlaces are the places of the setting as the talk shows them, in the
// order it does.
var talkPlaces = []prompt.Place{
	{Place: workspace.PlaceBody, Heading: "设定正文（作者定下的，必须遵守）："},
	{Place: workspace.PlaceDropped, Label: "已放弃", Heading: "已放弃（不要再推荐，换个说法也不行）："},
	{Place: workspace.PlaceOpen, Label: "悬而未决", Heading: "悬而未决（作者还没表态，可以讨论）："},
}

// indexNote opens the setting when it goes as an index.
const indexNote = "设定较多，这里只列目录：每条的编号、分类、名字和第一句。要看全文和出处，用 look_up_setting 查。"

// digestNote opens the digest of the talk compacted.
const digestNote = "之前的讨论已经整理成下面的摘要。原话还在，要用时用 search_talks 查。"

// numbered is line with the entry's number before it: the talk's model
// names an entry by it to note the setting. Entries are never removed, so
// a number stays the entry's.
func numbered(set workspace.Setting, line prompt.Line) prompt.Line {
	at := map[string]int{}
	for n, e := range set.Entries {
		at[e.ID] = n + 1
	}
	return func(e workspace.Entry) string { return fmt.Sprintf("[%d] %s", at[e.ID], line(e)) }
}

// changes are how each entry of set reads from chapters on, by its ID, as
// the talk tells them after it: each change that holds, then each the AI
// suggested that waits, from the chapter of tree it starts at.
func changes(set workspace.Setting, tree workspace.Tree) map[string][]string {
	o, name := tree.Order(), bookOf(tree).name
	out := map[string][]string{}
	for _, e := range set.Entries {
		for _, g := range e.Holding(o) {
			out[e.ID] = append(out[e.ID], "「"+name[g.From]+"」起："+prompt.OneLine(g.Text))
		}
		for _, g := range e.Changes {
			if _, ok := o[g.From]; ok && g.Place == workspace.PlaceOpen {
				out[e.ID] = append(out[e.ID], "「"+name[g.From]+"」起（悬而未决）："+prompt.OneLine(g.Text))
			}
		}
	}
	return out
}

// timed is line with the changes of the entry after it, a line each.
func timed(line prompt.Line, changes map[string][]string) prompt.Line {
	return func(e workspace.Entry) string {
		s := line(e)
		for _, c := range changes[e.ID] {
			s += "\n  " + c
		}
		return s
	}
}

// latest is set as it reads after its last changes: the talk's index has
// the entries as they are now.
func latest(set workspace.Setting, tree workspace.Tree) workspace.Setting {
	o := tree.Order()
	out := workspace.Setting{Entries: slices.Clone(set.Entries), Revisions: set.Revisions}
	for i, e := range out.Entries {
		if hold := e.Holding(o); len(hold) > 0 {
			out.Entries[i].Text = hold[len(hold)-1].Text
		}
	}
	return out
}

// Along is what goes with a talk besides the talk: what the author is
// writing as they talk beside the prose, or the chapter a talk opened to
// draft or rewrite keeps to.
type Along struct {
	Beside *Writing
	Fixed  *Fixed
}

// Assemble builds a talk request of at most room tokens, in the order a
// provider's cache can reuse: the rules, the setting, what goes with the
// chapter a talk keeps to, the digest of the talk compacted, the complete
// exchanges after it, and the author's words. The exchanges go from the
// newest as far as they fit. The setting goes whole when it fits after the
// talk, each entry with how it changes from the chapters of tree on, and
// as an index when it does not; the index, the digest and what goes with
// the chapter must fit. A talk that keeps to a chapter has the setting as
// it stands there.
func Assemble(w workspace.Work, room int, set workspace.Setting, tree workspace.Tree, events []workspace.Event, input string, along Along) (Context, error) {
	digest, turns := uncompacted(events)
	out, _, err := lay(w, room, set, tree, talk{digest: digest.Text, turns: turns, input: input, beside: along.Beside, near: along.Beside != nil, fixed: along.Fixed})
	return out, err
}

// talk is what a talk request carries besides the rules and the setting.
type talk struct {
	digest string     // of the talk compacted, if any
	turns  []exchange // the complete exchanges after it
	input  string     // the last words, which must fit
	beside *Writing   // what the author is writing, sent with input
	near   bool       // the author talks beside the prose, as the rules say
	fixed  *Fixed     // the chapter the talk keeps to, if any
	oldest bool       // keep the oldest turns that fit, not the newest
}

// view is the setting as a talk shows it: whole, each entry with its
// changes after it, and as an index each entry as it reads after them;
// or, in a talk that keeps to a chapter, as it stands there either way,
// with how many changes come after.
type view struct {
	whole, index workspace.Setting
	later        map[string][]string
	ahead        int
	at           string // the chapter it stands at, as the model names it
}

func viewOf(set workspace.Setting, tree workspace.Tree, f *Fixed) view {
	if f == nil {
		return view{whole: set, index: latest(set, tree), later: changes(set, tree)}
	}
	at, ahead := set.At(tree.Order(), f.Chapter)
	return view{whole: at, index: at, ahead: ahead, at: f.Title}
}

// note is what follows the setting of v: how many changes come after the
// chapter it stands at.
func (v view) note() string {
	if v.ahead == 0 {
		return ""
	}
	return fmt.Sprintf("\n\n以上是写到「%s」时的设定；后面的章节里还有 %d 处变化，没有列出。", v.at, v.ahead)
}

// lay builds a request of at most room tokens for t, as Assemble tells,
// and how many of its turns went.
func lay(w workspace.Work, room int, set workspace.Setting, tree workspace.Tree, t talk) (Context, int, error) {
	out := Context{} // the reply may take all the model gives: its output is kept out of the room
	rules := talkRules
	switch {
	case t.fixed != nil:
		rules += "\n\n" + originRules[t.fixed.Origin]
	case t.near:
		rules += "\n\n" + besideRules
	}
	tooLong := errors.New("当前发言超出模型的上下文上限；请缩短，或在设置中「AI」配置里调大上下文上限")
	out.Request.Tools = tools(w)
	told := withTools(rules, out.Request.Tools)
	b := prompt.NewBudget(room)
	input := t.input
	if !b.Must(workspace.PartRules, told) || !b.Must(workspace.PartInput, input) {
		return out, 0, tooLong
	}
	v := viewOf(set, tree, t.fixed)
	index := talkIndex(w, set, v)
	if !b.Must(workspace.PartIndex, index) {
		return out, 0, errors.New("设定的目录超出模型的上下文上限，请在设置中「AI」配置里调大上下文上限")
	}
	var digest string
	if t.digest != "" {
		digest = "\n\n" + digestNote + "\n\n" + t.digest
		if !b.Must(workspace.PartDigest, digest) {
			return out, 0, errors.New("之前讨论的摘要超出模型的上下文上限，请在设置中「AI」配置里调大上下文上限")
		}
	}
	// What goes with the chapter the talk keeps to comes before the talk: it
	// changes as the chapter does, the talk only grows.
	var fixed string
	if t.fixed != nil {
		for _, p := range t.fixed.Pieces {
			if !b.Must(p.Part, "\n\n"+p.Text) {
				return out, 0, errors.New("这一章要带上的内容超出模型的上下文上限，请在设置中「AI」配置里调大上下文上限")
			}
			fixed += "\n\n" + p.Text
			out.Included = append(out.Included, workspace.ContextItem{Text: p.Item})
		}
	}
	// What the author is writing goes with their words: they asked beside it.
	if t.beside != nil {
		attached, items := t.beside.render(b)
		if b.Left() < 0 {
			return out, 0, tooLong
		}
		input = attached + input
		out.Included = append(out.Included, items...)
	}
	msgs, took := fill(b, t.turns, t.oldest)
	b.Return(workspace.PartIndex)
	later := v.later
	setting := prompt.Render(w, v.whole, talkPlaces, nil, numbered(set, timed(prompt.Whole, later))) + v.note()
	shown, line := v.whole, prompt.Line(prompt.Whole)
	if !b.Take(workspace.PartSetting, setting) {
		b.Must(workspace.PartIndex, index)
		setting, shown, line, later = index, v.index, prompt.Short, nil
	}
	for _, p := range talkPlaces {
		for _, e := range prompt.Ordered(w, shown.Entries) {
			if e.Place == p.Place {
				item := p.Item(e, line)
				item.Changes = later[e.ID]
				out.Included = append(out.Included, item)
			}
		}
	}
	out.Request.System = rules + "\n\n" + setting + fixed + digest
	out.Request.Messages = append(msgs, model.Message{Role: workspace.RoleUser, Content: input})
	out.Sent, out.Left = b.Sent(told, set), max(b.Left(), 0)
	out.Sent.AsOf, out.Sent.Ahead = v.at, v.ahead
	return out, took, nil
}

// talkIndex is the setting of v as its index, under the note that opens
// it, each entry numbered as in set.
func talkIndex(w workspace.Work, set workspace.Setting, v view) string {
	return indexNote + "\n\n" + prompt.Render(w, v.index, talkPlaces, nil, numbered(set, prompt.Short)) + v.note()
}

// fill takes as many turns as fit in b, from the newest, or from the
// oldest for oldest; then a first turn too long for the room by itself
// goes cut to what fits. It tells how many went.
func fill(b *prompt.Budget, turns []exchange, oldest bool) ([]model.Message, int) {
	var msgs []model.Message
	took := 0
	for ; took < len(turns); took++ {
		n := len(turns) - 1 - took
		if oldest {
			n = took
		}
		x := turns[n]
		if !b.Take(workspace.PartTalk, x.text()) {
			break
		}
		if oldest {
			msgs = append(msgs, x.user, x.reply)
		} else {
			msgs = append([]model.Message{x.user, x.reply}, msgs...)
		}
	}
	if took == 0 && oldest && len(turns) > 0 {
		x, half := turns[0], b.Left()/2
		x.user.Content, x.reply.Content = prompt.ClipTokens(x.user.Content, half), prompt.ClipTokens(x.reply.Content, half)
		if b.Take(workspace.PartTalk, x.text()) {
			msgs, took = []model.Message{x.user, x.reply}, 1
		}
	}
	return msgs, took
}

// told is what the model is told before the setting, every step: the
// rules and the tools.
func withTools(rules string, tools []model.Tool) string {
	for _, t := range tools {
		rules += "\n" + t.Name + " " + t.Description + " " + string(t.Schema)
	}
	return rules
}

// exchange is a complete turn, at the place of its reply among the events.
type exchange struct {
	user, reply model.Message
	at          int
}

func (x exchange) text() string { return x.user.Content + "\n" + x.reply.Content }

// exchanges are the complete turns of events, in the order of their
// replies: an unanswered turn must not masquerade as chat context. What a
// reply looked up on its way is not sent again; what it proposed in the
// prose is, after it.
func exchanges(events []workspace.Event) []exchange {
	said := map[string]workspace.Event{}
	proposed := map[string][]*workspace.Proposal{}
	var out []exchange
	for n, e := range events {
		switch e.Role {
		case workspace.RoleUser:
			said[e.ID] = e
		case workspace.RoleResult:
			if e.Proposal != nil {
				proposed[e.ReplyTo] = append(proposed[e.ReplyTo], e.Proposal)
			}
		case workspace.RoleAssistant:
			if u, ok := said[e.ReplyTo]; ok {
				reply := e.Text
				if ps := proposed[e.ReplyTo]; len(ps) > 0 {
					reply += "\n\n【这一轮在正文里的提议】"
					for _, p := range ps {
						reply += "\n" + proposalLine(*p)
					}
				}
				out = append(out, exchange{model.Message{Role: workspace.RoleUser, Content: u.Said()}, model.Message{Role: workspace.RoleAssistant, Content: reply}, n})
			}
		}
	}
	return out
}

// proposalLine is p as the talk is told it: where, then the prose.
func proposalLine(p workspace.Proposal) string {
	where := "「" + p.Title + "」"
	switch p.At {
	case workspace.ProposeReplace:
		where += "换掉「" + p.Quote + "」"
	case workspace.ProposeAfter:
		where += "接在「" + p.Quote + "」之后"
	default:
		where += atNames[p.At]
	}
	return where + "：\n" + p.Text
}

// uncompacted is the latest digest among events, if any, and the complete
// turns it does not cover: those replied to after the event it runs
// through. A digest through an event not among them covers them all.
func uncompacted(events []workspace.Event) (workspace.Event, []exchange) {
	turns := exchanges(events)
	for n := len(events) - 1; n >= 0; n-- {
		d := events[n]
		if d.Role != workspace.RoleDigest {
			continue
		}
		through := slices.IndexFunc(events, func(e workspace.Event) bool { return e.ID == d.Through })
		if k := slices.IndexFunc(turns, func(x exchange) bool { return x.at > through }); through >= 0 && k >= 0 {
			return d, turns[k:]
		}
		return d, nil
	}
	return workspace.Event{}, turns
}
