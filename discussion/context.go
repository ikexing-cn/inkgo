// Package discussion runs the talk with the author and the bookkeeper that
// keeps the setting document up with it.
package discussion

import (
	"errors"

	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

// Context is what one talk request sends, the entries of the setting and
// the prose it showed, and what each part took.
type Context struct {
	Request  model.Request
	Included []workspace.ContextItem
	Sent     *workspace.Sent
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
const indexNote = "设定较多，这里只列目录：每条的分类、名字和第一句。"

// Assemble builds a talk request of at most room tokens, in the order a
// provider's cache can reuse: the rules, the setting, the last few
// complete exchanges, which include every one the bookkeeper has not yet
// taken in (done is the number of events it covered), and the author's
// words. The setting goes whole when it fits after the talk, and as an
// index when it does not; the index must fit.
func Assemble(w workspace.Work, room int, set workspace.Setting, events []workspace.Event, done int, input string, beside *Writing) (Context, error) {
	out := Context{Request: model.Request{MaxTokens: talkOutput}}
	rules := talkRules
	if beside != nil {
		rules += "\n\n" + besideRules
	}
	tooLong := errors.New("当前发言超出模型的上下文上限；请缩短，或在设置中「AI」配置里调大上下文上限")
	b := prompt.NewBudget(room)
	if !b.Must(workspace.PartRules, rules) || !b.Must(workspace.PartInput, input) {
		return out, tooLong
	}
	index := indexNote + "\n\n" + prompt.Render(w, set, talkPlaces, nil, prompt.Short)
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
	keep := recentTurns
	for n := len(pairs) - 1; n >= 0 && pairs[n].at >= done; n-- {
		keep = max(keep, len(pairs)-n)
	}
	var msgs []model.Message
	for n := len(pairs) - 1; n >= 0 && n >= len(pairs)-keep; n-- {
		if !b.Take(workspace.PartTalk, pairs[n].user.Content+"\n"+pairs[n].reply.Content) {
			break
		}
		msgs = append([]model.Message{pairs[n].user, pairs[n].reply}, msgs...)
	}
	b.Return(workspace.PartIndex)
	setting, line := prompt.Render(w, set, talkPlaces, nil, prompt.Whole), prompt.Line(prompt.Whole)
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
	out.Sent = b.Sent(rules, set)
	return out, nil
}

type exchange struct {
	user, reply model.Message
	at          int // the reply's index in the events
}

// exchanges are the complete turns of events: an unanswered turn must not
// masquerade as chat context.
func exchanges(events []workspace.Event) []exchange {
	var out []exchange
	for n := 1; n < len(events); n++ {
		a, b := events[n-1], events[n]
		if a.Role == workspace.RoleUser && b.Role == workspace.RoleAssistant && b.ReplyTo == a.ID {
			out = append(out, exchange{model.Message{Role: workspace.RoleUser, Content: a.Said()}, model.Message{Role: workspace.RoleAssistant, Content: b.Text}, n})
		}
	}
	return out
}
