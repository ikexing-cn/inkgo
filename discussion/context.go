// Package discussion implements the passive discussion and bookkeeping loop.
package discussion

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"writer/model"
	"writer/workspace"
)

// Embedder is reserved for M2. M1 never calls an embedding service.
// Cache entries will live in .writer/embeddings/<model>/<text-sha256>.bin.
type Embedder interface {
	Embed(context.Context, []string) ([][]float32, error)
}

// Context is what one discussion request sends, and which ideas it had to
// leave out.
type Context struct {
	Request  model.Request
	Included []workspace.ContextItem
	Omitted  []workspace.ContextItem
}

const (
	talkReserve = 2048 // tokens kept for the reply
	poolSlack   = 512  // section headings and per-message overhead
	refCost     = 24   // a "[12] " prefix or a conflict note
	recentTurns = 4
)

// Estimate deliberately uses UTF-8 bytes, a conservative bound for ordinary
// byte-based tokenizers. Providers can have different tokenizers; this is not
// reported as an exact token count.
func Estimate(s string) int { return len(s) + 16 }

// sections order the pool in the prompt, by the status they show.
var sections = []struct {
	status         workspace.Status
	label, heading string
}{
	{workspace.Kept, "留下", "作者留下的方向："},
	{workspace.Deferred, "待定", "作者还没想好的方向："},
	{workspace.Adopted, "已定设定", "已定设定（只能遵守）："},
	{workspace.Rejected, "已放弃", "作者放弃的方向（不要再推荐）："},
}

func label(s workspace.Status) string {
	for _, sec := range sections {
		if sec.status == s {
			return sec.label
		}
	}
	return ""
}

// describe is an idea's line in the prompt, without its number.
func describe(i workspace.Idea) string {
	text := strings.Join(strings.Fields(i.Text), " ")
	if i.Status == workspace.Rejected && len(i.History) > 0 {
		if reason := i.History[len(i.History)-1].Reason; reason != "" {
			text += "（理由：" + reason + "）"
		}
	}
	if i.Fact && !i.Verified {
		text += "（现实事实，未核实）"
	}
	return text
}

// Assemble builds a discussion request: the rules, the ideas the author
// decided on, newest first until the budget runs out, and the last few
// complete exchanges.
func Assemble(ideas []workspace.Idea, events []workspace.Event, input string, budget int) (Context, error) {
	out := Context{Request: model.Request{MaxTokens: talkReserve}}
	remaining := budget - talkReserve - Estimate(talkRules) - Estimate(input) - poolSlack
	if remaining < 0 {
		return out, fmt.Errorf("当前发言超出上下文预算 %d；请缩短发言或提高作品预算", budget)
	}
	kept := map[string]bool{}
	for _, i := range ideas {
		if i.Status == workspace.Kept {
			kept[i.ID] = true
		}
	}
	candidates := slices.Clone(ideas)
	slices.SortStableFunc(candidates, func(a, b workspace.Idea) int { return b.Updated.Compare(a.Updated) })
	var shown []workspace.Idea
	for _, i := range candidates {
		if label(i.Status) == "" {
			continue
		}
		text := describe(i)
		item := workspace.ContextItem{ID: i.ID, Text: label(i.Status) + "：" + text}
		cost := Estimate(text) + refCost*(1+len(activeConflicts(i, kept)))
		if cost > remaining {
			item.Reason = "超出上下文预算"
			out.Omitted = append(out.Omitted, item)
			continue
		}
		remaining -= cost
		shown = append(shown, i)
		out.Included = append(out.Included, item)
	}
	out.Request.System = talkRules + "\n\n" + renderPool(shown, kept)
	out.Request.Messages = append(recent(events, remaining), model.Message{Role: workspace.RoleUser, Content: input})
	return out, nil
}

func activeConflicts(i workspace.Idea, kept map[string]bool) []string {
	if !kept[i.ID] {
		return nil
	}
	var ids []string
	for _, id := range i.Conflicts {
		if kept[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

// renderPool lists ideas by section under numbers local to this request.
func renderPool(shown []workspace.Idea, kept map[string]bool) string {
	if len(shown) == 0 {
		return "作者还没有对任何方向表态。"
	}
	ref := map[string]int{}
	for n, i := range shown {
		ref[i.ID] = n + 1
	}
	var b strings.Builder
	for _, sec := range sections {
		first := true
		for _, i := range shown {
			if i.Status != sec.status {
				continue
			}
			if first {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(sec.heading + "\n")
				first = false
			}
			fmt.Fprintf(&b, "[%d] %s", ref[i.ID], describe(i))
			for _, id := range activeConflicts(i, kept) {
				if r, ok := ref[id]; ok {
					fmt.Fprintf(&b, "（与 [%d] 矛盾，作者尚未解决）", r)
				}
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// recent keeps the last complete exchanges that fit. Unanswered turns and
// bookkeeping records must not masquerade as chat context.
func recent(events []workspace.Event, remaining int) []model.Message {
	var pairs [][2]model.Message
	for n := 1; n < len(events); n++ {
		a, b := events[n-1], events[n]
		if a.Kind == workspace.KindMessage && a.Role == workspace.RoleUser &&
			b.Kind == workspace.KindMessage && b.Role == workspace.RoleAssistant && b.ReplyTo == a.ID {
			pairs = append(pairs, [2]model.Message{{Role: workspace.RoleUser, Content: a.Text}, {Role: workspace.RoleAssistant, Content: b.Text}})
		}
	}
	var msgs []model.Message
	for n := len(pairs) - 1; n >= 0 && n >= len(pairs)-recentTurns; n-- {
		cost := Estimate(pairs[n][0].Content) + Estimate(pairs[n][1].Content)
		if cost > remaining {
			break
		}
		remaining -= cost
		msgs = append([]model.Message{pairs[n][0], pairs[n][1]}, msgs...)
	}
	return msgs
}
