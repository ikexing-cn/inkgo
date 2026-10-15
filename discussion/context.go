// Package discussion runs the talk with the author: a reply looks things
// up in the book with tools and keeps the setting document up with the
// talk.
package discussion

import (
	"errors"
	"fmt"

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

const (
	talkOutput  = 32768 // what the request allows, reasoning included
	recentTurns = 4
)

// talkPlaces are the places of the setting as the talk shows them, in the
// order it does.
var talkPlaces = []prompt.Place{
	{Place: workspace.PlaceBody, Heading: "设定正文（作者定下的，必须遵守）："},
	{Place: workspace.PlaceDropped, Label: "已放弃", Heading: "已放弃（不要再推荐，换个说法也不行）："},
	{Place: workspace.PlaceOpen, Label: "悬而未决", Heading: "悬而未决（作者还没表态，可以讨论）："},
}

// indexNote opens the setting when it goes as an index.
const indexNote = "设定较多，这里只列目录：每条的编号、分类、名字和第一句。要看全文和出处，用 look_up_setting 查。"

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
// provider's cache can reuse: the rules, the setting, the last few
// complete exchanges, and the author's words. The setting goes whole when
// it fits after the talk, and as an index when it does not; the index must
// fit.
func Assemble(w workspace.Work, room int, set workspace.Setting, events []workspace.Event, input string, beside *Writing) (Context, error) {
	out := Context{Request: model.Request{MaxTokens: talkOutput}}
	rules := talkRules
	if beside != nil {
		rules += "\n\n" + besideRules
	}
	tooLong := errors.New("当前发言超出模型的上下文上限；请缩短，或在设置中「AI」配置里调大上下文上限")
	out.Request.Tools = tools(w)
	told := withTools(rules, out.Request.Tools)
	b := prompt.NewBudget(room)
	if !b.Must(workspace.PartRules, told) || !b.Must(workspace.PartInput, input) {
		return out, tooLong
	}
	index := indexNote + "\n\n" + prompt.Render(w, set, talkPlaces, nil, numbered(set, prompt.Short))
	if !b.Must(workspace.PartIndex, index) {
		return out, errors.New("设定的目录超出模型的上下文上限，请在设置中「AI」配置里调大上下文上限")
	}
	// What the author is writing goes with their words: they asked beside it.
	if beside != nil {
		attached, items := beside.render(b)
		if b.Left() < 0 {
			return out, tooLong
		}
		input = attached + input
		out.Included = append(out.Included, items...)
	}
	pairs := exchanges(events)
	var msgs []model.Message
	for n := len(pairs) - 1; n >= 0 && n >= len(pairs)-recentTurns; n-- {
		if !b.Take(workspace.PartTalk, pairs[n].user.Content+"\n"+pairs[n].reply.Content) {
			break
		}
		msgs = append([]model.Message{pairs[n].user, pairs[n].reply}, msgs...)
	}
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
	out.Request.System = rules + "\n\n" + setting
	out.Request.Messages = append(msgs, model.Message{Role: workspace.RoleUser, Content: input})
	out.Sent, out.Left = b.Sent(told, set), max(b.Left(), 0)
	return out, nil
}

// told is what the model is told before the setting, every step: the
// rules and the tools.
func withTools(rules string, tools []model.Tool) string {
	for _, t := range tools {
		rules += "\n" + t.Name + " " + t.Description + " " + string(t.Schema)
	}
	return rules
}

type exchange struct {
	user, reply model.Message
}

// exchanges are the complete turns of events, in the order of their
// replies: an unanswered turn must not masquerade as chat context. What a
// reply looked up on its way is not sent again.
func exchanges(events []workspace.Event) []exchange {
	said := map[string]workspace.Event{}
	var out []exchange
	for _, e := range events {
		switch e.Role {
		case workspace.RoleUser:
			said[e.ID] = e
		case workspace.RoleAssistant:
			if u, ok := said[e.ReplyTo]; ok {
				out = append(out, exchange{model.Message{Role: workspace.RoleUser, Content: u.Said()}, model.Message{Role: workspace.RoleAssistant, Content: e.Text}})
			}
		}
	}
	return out
}
