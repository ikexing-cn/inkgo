// Package discussion runs the talk with the author and the bookkeeper that
// keeps the setting document up with it.
package discussion

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"writer/model"
	"writer/workspace"
)

// Context is what one talk request sends, and which entries it had to
// leave out.
type Context struct {
	Request  model.Request
	Included []workspace.ContextItem
	Omitted  []workspace.ContextItem
}

const (
	// talkReserve is the reply's share of the context budget. The request
	// allows far more output, talkOutput, as reasoning comes out of it too.
	talkReserve = 2048
	talkOutput  = 32768
	docSlack    = 512 // headings and per-message overhead
	lineCost    = 8   // a "- " and a newline
	recentTurns = 4
)

// Estimate bounds the tokens of s without the provider's tokenizer: one for
// every three ASCII bytes, and one and a half for every other character,
// above what the tokenizers we know of spend on Chinese. It is never
// reported as an exact count.
func Estimate(s string) int {
	ascii, other := 0, 0
	for _, r := range s {
		if r < utf8.RuneSelf {
			ascii++
		} else {
			other++
		}
	}
	return (ascii+2)/3 + (other*3+1)/2 + 16
}

// places order the setting in the prompt and decide what goes first when
// it does not all fit: the body binds the reply, then what the author
// dropped, then what is open.
var places = []struct {
	place, label, heading string
}{
	{workspace.PlaceBody, "正文", "设定正文（作者定下的，必须遵守）："},
	{workspace.PlaceDropped, "已放弃", "已放弃（不要再推荐，换个说法也不行）："},
	{workspace.PlaceOpen, "悬而未决", "悬而未决（作者还没表态，可以讨论）："},
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// Assemble builds a talk request: the rules, the setting as far as the
// budget allows, newest entries first within each place, and the last few
// complete exchanges, which include every one the bookkeeper has not yet
// taken in: done is the number of events it covered.
func Assemble(w workspace.Work, set workspace.Setting, events []workspace.Event, done int, input string, beside *Writing) (Context, error) {
	out := Context{Request: model.Request{MaxTokens: talkOutput}}
	rules := talkRules
	if beside != nil {
		rules += "\n\n" + besideRules
	}
	remaining := w.TokenBudget - talkReserve - Estimate(rules) - Estimate(input) - docSlack
	if remaining < 0 {
		return out, fmt.Errorf("当前发言超出上下文预算 %d；请缩短发言或提高作品预算", w.TokenBudget)
	}
	// What the author is writing comes first, before the talk and the
	// setting: they asked beside it.
	if beside != nil {
		attached, items, cost := beside.render(remaining)
		remaining -= cost
		input = attached + input
		out.Included = append(out.Included, items...)
	}
	// The exchanges the setting lags behind come before the setting itself.
	pairs := exchanges(events)
	keep := recentTurns
	for n := len(pairs) - 1; n >= 0 && pairs[n].at >= done; n-- {
		keep = max(keep, len(pairs)-n)
	}
	var msgs []model.Message
	for n := len(pairs) - 1; n >= 0 && n >= len(pairs)-keep; n-- {
		cost := Estimate(pairs[n].user.Content) + Estimate(pairs[n].reply.Content)
		if cost > remaining {
			break
		}
		remaining -= cost
		msgs = append([]model.Message{pairs[n].user, pairs[n].reply}, msgs...)
	}
	shown := map[string]bool{}
	for _, p := range places {
		entries := slices.Clone(set.Entries)
		slices.SortStableFunc(entries, func(a, b workspace.Entry) int { return cmp.Compare(b.Rev, a.Rev) })
		for _, e := range entries {
			if e.Place != p.place {
				continue
			}
			text := oneLine(e.Text)
			item := workspace.ContextItem{ID: e.ID, Text: p.label + "：" + text}
			if cost := Estimate(text) + lineCost; cost <= remaining {
				remaining -= cost
				shown[e.ID] = true
				out.Included = append(out.Included, item)
			} else {
				item.Reason = "超出上下文预算"
				out.Omitted = append(out.Omitted, item)
			}
		}
	}
	out.Request.System = rules + "\n\n" + renderSetting(w, set, shown)
	out.Request.Messages = append(msgs, model.Message{Role: workspace.RoleUser, Content: input})
	return out, nil
}

// renderSetting lists the entries shown, the body by category in the
// work's order, in the order they were added.
func renderSetting(w workspace.Work, set workspace.Setting, shown map[string]bool) string {
	if len(shown) == 0 {
		return "设定文档还是空的。"
	}
	var b strings.Builder
	for _, p := range places {
		var lines []string
		if p.place == workspace.PlaceBody {
			for _, c := range w.Categories {
				first := true
				for _, e := range set.Entries {
					if shown[e.ID] && e.Place == p.place && e.Category == c.Name {
						if first {
							lines = append(lines, "【"+c.Name+"】")
							first = false
						}
						lines = append(lines, "- "+oneLine(e.Text))
					}
				}
			}
		} else {
			for _, e := range set.Entries {
				if shown[e.ID] && e.Place == p.place {
					lines = append(lines, "- "+oneLine(e.Text))
				}
			}
		}
		if len(lines) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(p.heading + "\n" + strings.Join(lines, "\n"))
	}
	return b.String()
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
