// Package discussion runs the talk with the author: a reply looks things
// up in the book with tools and keeps the setting document up with the
// talk.
package discussion

import (
	"errors"
	"fmt"
	"slices"
	"sort"

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

// Assemble builds a talk request of at most room tokens, in the order a
// provider's cache can reuse: the rules, the setting, the digest of the
// talk compacted, the complete exchanges after it, and the author's words.
// The exchanges go from the newest as far as they fit. The setting goes
// whole when it fits after the talk, and as an index when it does not; the
// index and the digest must fit.
func Assemble(w workspace.Work, room int, set workspace.Setting, events []workspace.Event, input string, beside *Writing) (Context, error) {
	digest, turns := uncompacted(events)
	out, _, err := lay(w, room, set, talk{digest: digest.Text, turns: turns, input: input, beside: beside, near: beside != nil})
	return out, err
}

// talk is what a talk request carries besides the rules and the setting.
type talk struct {
	digest string     // of the talk compacted, if any
	turns  []exchange // the complete exchanges after it
	input  string     // the last words, which must fit
	beside *Writing   // what the author is writing, sent with input
	near   bool       // the author talks beside the prose, as the rules say
	oldest bool       // keep the oldest turns that fit, not the newest
}

// lay builds a request of at most room tokens for t, as Assemble tells,
// and how many of its turns went.
func lay(w workspace.Work, room int, set workspace.Setting, t talk) (Context, int, error) {
	out := Context{} // the reply may take all the model gives: its output is kept out of the room
	rules := talkRules
	if t.near {
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
	index := talkIndex(w, set)
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
	setting, line := prompt.Render(w, set, talkPlaces, nil, numbered(set, prompt.Whole)), prompt.Line(prompt.Whole)
	if !b.Take(workspace.PartSetting, setting) {
		b.Must(workspace.PartIndex, index)
		setting, line = index, prompt.Short
	}
	for _, p := range talkPlaces {
		for _, e := range prompt.Ordered(w, set.Entries) {
			if e.Place == p.Place {
				out.Included = append(out.Included, p.Item(e, line))
			}
		}
	}
	out.Request.System = rules + "\n\n" + setting + digest
	out.Request.Messages = append(msgs, model.Message{Role: workspace.RoleUser, Content: input})
	out.Sent, out.Left = b.Sent(told, set), max(b.Left(), 0)
	return out, took, nil
}

// talkIndex is the setting as its index, under the note that opens it.
func talkIndex(w workspace.Work, set workspace.Setting) string {
	return indexNote + "\n\n" + prompt.Render(w, set, talkPlaces, nil, numbered(set, prompt.Short))
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
		x.user.Content, x.reply.Content = clipTo(x.user.Content, half), clipTo(x.reply.Content, half)
		if b.Take(workspace.PartTalk, x.text()) {
			msgs, took = []model.Message{x.user, x.reply}, 1
		}
	}
	return msgs, took
}

// cutMark ends what was cut to fit.
const cutMark = "……（后面太长，略去）"

// clipTo cuts s from its end to at most tokens, as estimated.
func clipTo(s string, tokens int) string {
	if prompt.Estimate(s) <= tokens {
		return s
	}
	r := []rune(s)
	n := sort.Search(len(r)+1, func(n int) bool { return prompt.Estimate(string(r[:n])+cutMark) > tokens }) - 1
	return string(r[:max(n, 0)]) + cutMark
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
// reply looked up on its way is not sent again.
func exchanges(events []workspace.Event) []exchange {
	said := map[string]workspace.Event{}
	var out []exchange
	for n, e := range events {
		switch e.Role {
		case workspace.RoleUser:
			said[e.ID] = e
		case workspace.RoleAssistant:
			if u, ok := said[e.ReplyTo]; ok {
				out = append(out, exchange{model.Message{Role: workspace.RoleUser, Content: u.Said()}, model.Message{Role: workspace.RoleAssistant, Content: e.Text}, n})
			}
		}
	}
	return out
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
