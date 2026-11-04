package discussion

import (
	"context"
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
// How big that request is goes by the last reply's as the provider counted
// it, where it can, and by estimate where it cannot.
// A digest is asked for in a request laid out as the talk's, so a
// provider's cache reuses it, and takes in the digest before and as many
// turns as the request holds; it takes as many requests as it needs. The
// talk stays as it was said.
func (e *Engine) compact(ctx context.Context, session string, before int, user workspace.Event) error {
	w := e.Store.Work()
	set, events, err := e.read(session, before)
	if err != nil {
		return err
	}
	tree, err := e.Store.Chapters()
	if err != nil {
		return err
	}
	along, err := e.along(tree, events, user)
	if err != nil {
		return err
	}
	probe, err := Assemble(w, math.MaxInt32, set, tree, events, user.Said(), along)
	if err != nil {
		return err
	}
	_, turns := uncompacted(events)
	n := due(e.Talk.Model, probe.Sent, measured(events, probe.Sent), prompt.Estimate(talkIndex(w, set, viewOf(set, tree, along.Fixed))), turns)
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
		asked, took, err := lay(w, e.Talk.Input(), set, tree, talk{digest: digest.Text, turns: turns[:n], input: digestAsk, near: along.Beside != nil, fixed: along.Fixed, oldest: true})
		if err != nil {
			return err
		}
		if took == 0 {
			return &model.SetupError{Err: &model.ContextLimitError{Part: "turn"}}
		}
		asked.Request.NoCalls = true
		if e.Step != nil {
			e.Step(workspace.Event{Role: workspace.RoleDigest})
		}
		reply, err := ask(e.purpose(ctx, session, asked.Sent, along.Fixed), e.Talk, asked.Request, nil)
		if err != nil {
			return err
		}
		if strings.TrimSpace(reply.Text) == "" {
			return ErrEmptyDigest
		}
		d, err := e.Store.Append(session, workspace.Event{
			Role: workspace.RoleDigest, Text: strings.TrimSpace(reply.Text), Through: events[turns[took-1].at].ID, Sent: asked.Sent,
			Model: e.Talk.Name, Effort: string(e.Talk.Effort), Reasoning: strings.TrimSpace(reply.Reasoning), Usage: (*workspace.Usage)(&reply.Usage),
		})
		if err != nil {
			return err
		}
		if e.Step != nil {
			e.Step(d)
		}
		if set, events, err = e.read(session, before); err != nil {
			return err
		}
	}
}

// due is how many of turns, the oldest not compacted yet, to digest before
// a request that would send all of them as sent records, shift tokens more
// than estimated: none while it stays within the model's threshold and
// what it takes in, or else the oldest until the rest take a fifth of the
// window, all but the last. The setting counts as it would go with the
// talk that short: whole, or as its index of index tokens, which shift
// does not measure.
func due(m model.Model, sent *workspace.Sent, shift, index int, turns []exchange) int {
	limit := min(m.Window*m.CompactAt/100, m.Input())
	keep := m.Window * keepPercent / 100
	setting, talk := sent.Part(workspace.PartSetting), sent.Part(workspace.PartTalk)
	rest := prompt.Size(sent) - setting - talk
	if rest+setting+keep > limit {
		setting, shift = index, 0
	}
	if rest+setting+talk+shift <= limit {
		return 0
	}
	n := 0
	for ; n < len(turns)-1 && talk > keep; n++ {
		talk -= prompt.Estimate(turns[n].text())
	}
	return n
}

// measured is how many tokens more than estimated, or fewer, a request
// that would send as sent records takes: as many as the last reply's
// request did, as the provider counted it against its estimate. The two
// share all but what each adds, and what each adds is small next to it.
// It is none where there is nothing to measure by: the last reply has no
// count, or the talk was compacted, or the setting changed, since.
func measured(events []workspace.Event, sent *workspace.Sent) int {
	for n := len(events) - 1; n >= 0; n-- {
		switch ev := events[n]; ev.Role {
		case workspace.RoleDigest:
			return 0
		case workspace.RoleAssistant:
			if ev.Usage == nil || ev.Usage.Input == 0 || ev.Sent == nil || ev.Sent.Setting != sent.Setting ||
				(ev.Sent.Part(workspace.PartIndex) > 0) != (sent.Part(workspace.PartIndex) > 0) {
				return 0
			}
			return ev.Usage.Input - prompt.Size(ev.Sent)
		}
	}
	return 0
}
