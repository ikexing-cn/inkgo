package discussion

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"

	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

// keepPercent is the share of the window the talk not compacted yet is
// brought down to, as estimated.
const keepPercent = 20

// compact digests the oldest turns not compacted yet when a request
// sending them all, with input, would be over the model's threshold, until
// the turns left take about a fifth of the window; the last turn stays.
// A digest is asked for in a request laid out as the talk's, so a
// provider's cache reuses it, and takes in the digest before and as many
// turns as the request holds; it takes as many requests as it needs. The
// talk stays as it was said.
func (e *Engine) compact(ctx context.Context, session string, before int, input string) error {
	w := e.Store.Work()
	set, events, err := e.read(session, before)
	if err != nil {
		return err
	}
	probe, err := Assemble(w, math.MaxInt32, set, events, input, e.Beside)
	if err != nil {
		return err
	}
	_, turns := uncompacted(events)
	n := due(e.Talk.Model, probe.Sent, prompt.Estimate(talkIndex(w, set)), turns)
	if n == 0 {
		return nil
	}
	through := events[turns[n-1].at].ID
	for {
		digest, turns := uncompacted(events)
		n := slices.IndexFunc(turns, func(x exchange) bool { return events[x.at].ID == through }) + 1
		if n == 0 {
			return nil
		}
		asked, took, err := lay(w, e.Talk.Input(), set, talk{digest: digest.Text, turns: turns[:n], input: digestAsk, near: e.Beside != nil, oldest: true})
		if err != nil {
			return err
		}
		if took == 0 {
			return errors.New("有一轮讨论太长，超出模型的上下文上限，请在设置中「AI」配置里调大上下文上限")
		}
		asked.Request.NoCalls = true
		reply, err := ask(ctx, e.Talk, asked.Request, nil)
		if err != nil {
			return err
		}
		if strings.TrimSpace(reply.Text) == "" {
			return errors.New("模型返回空摘要")
		}
		if _, err = e.Store.Append(session, workspace.Event{
			Role: workspace.RoleDigest, Text: strings.TrimSpace(reply.Text), Through: events[turns[took-1].at].ID, Sent: asked.Sent,
			Model: e.Talk.Name, Effort: string(e.Talk.Effort), Reasoning: strings.TrimSpace(reply.Reasoning), Usage: (*workspace.Usage)(&reply.Usage),
		}); err != nil {
			return err
		}
		if set, events, err = e.read(session, before); err != nil {
			return err
		}
	}
}

// due is how many of turns, the oldest not compacted yet, to digest before
// a request that would send all of them as sent records: none while it
// stays within the model's threshold and what it takes in, or else the
// oldest until the rest take a fifth of the window, all but the last. The
// setting counts as it would go with the talk that short: whole, or as its
// index of index tokens.
func due(m model.Model, sent *workspace.Sent, index int, turns []exchange) int {
	limit := min(m.Window*m.CompactAt/100, m.Input())
	keep := m.Window * keepPercent / 100
	setting, talk := sent.Part(workspace.PartSetting), sent.Part(workspace.PartTalk)
	rest := prompt.Size(sent) - setting - talk
	if rest+setting+keep > limit {
		setting = index
	}
	if rest+setting+talk <= limit {
		return 0
	}
	n := 0
	for ; n < len(turns)-1 && talk > keep; n++ {
		talk -= prompt.Estimate(turns[n].text())
	}
	return n
}
