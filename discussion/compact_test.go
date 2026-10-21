package discussion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

func TestDigestRequestBytes(t *testing.T) {
	e, session := setup(t)
	long(t, e, session, 12, 700, "井边的灯为什么不亮")
	f := &digester{}
	e.Talk = Model{Model: limits, Completer: f, Effort: model.EffortHigh}
	if _, err := e.Say(context.Background(), session, workspace.Event{Text: "接着说"}); err != nil {
		t.Fatal(err)
	}
	requestBytes(t, f.calls, "b663cd8551745a528dd6577cb5ef4461da5accaa2106ac302e1c9a2ae959a29e")
}

// digester writes the next digest when one is asked for, and replies,
// counted as used, otherwise; it keeps what it was asked.
type digester struct {
	calls    []model.Request
	purposes []workspace.Purpose
	digests  int
	err      error
	used     model.Usage
}

func (d *digester) Complete(ctx context.Context, r model.Request, on func(model.Delta)) (model.Reply, error) {
	d.calls = append(d.calls, r)
	d.purposes = append(d.purposes, workspace.PurposeOf(ctx))
	if d.err != nil {
		return model.Reply{}, d.err
	}
	if r.Messages[len(r.Messages)-1].Content == digestAsk {
		d.digests++
		return model.Reply{Text: fmt.Sprintf("## 在聊什么\n第%d份摘要", d.digests), Reasoning: "先理一理", Usage: model.Usage{Input: 9000, CacheRead: 8000, Output: 300}}, nil
	}
	return model.Reply{Text: "好", Usage: d.used}, nil
}

// long appends n exchanges whose replies are size characters, about one
// and a half tokens each, the first said with first, and returns their
// replies. The rules and the tools take about 3K tokens; a test model holds
// 16K, compacts above it and keeps 4K.
func long(t *testing.T, e *Engine, session string, n, size int, first string) []workspace.Event {
	t.Helper()
	var replies []workspace.Event
	for k := 0; k < n; k++ {
		said := fmt.Sprintf("第%d轮", k)
		if k == 0 {
			said = first
		}
		_, a := turn(t, e, session, said, strings.Repeat("雨", size))
		replies = append(replies, a)
	}
	return replies
}

func digests(t *testing.T, e *Engine, session string) []workspace.Event {
	t.Helper()
	events, err := e.Store.Events(session)
	if err != nil {
		t.Fatal(err)
	}
	var out []workspace.Event
	for _, ev := range events {
		if ev.Role == workspace.RoleDigest {
			out = append(out, ev)
		}
	}
	return out
}

// A talk over the threshold is compacted before the reply: its oldest
// turns go into a digest, asked for as the talk is, so the cache holds,
// and the reply reads the digest and the turns after it. The talk stays,
// and search_talks still finds what was compacted.
func TestALongTalkIsCompacted(t *testing.T) {
	e, session := setup(t)
	replies := long(t, e, session, 12, 700, "井边的灯为什么不亮")
	talk := &digester{}
	e.Talk = Model{Model: limits, Completer: talk, Name: "deepseek/deepseek-flash", Effort: model.EffortHigh}
	if _, err := e.Say(context.Background(), session, workspace.Event{Text: "接着说"}); err != nil {
		t.Fatal(err)
	}
	if len(talk.calls) != 2 {
		t.Fatalf("%d requests", len(talk.calls))
	}
	for _, p := range talk.purposes {
		if p.Feature != workspace.FeatureTalk || p.Talk != session || len(p.Parts) == 0 {
			t.Fatalf("the record would say %+v", p)
		}
	}
	asked, replied := talk.calls[0], talk.calls[1]
	if !asked.NoCalls || len(asked.Tools) != 7 || !strings.HasPrefix(asked.System, talkRules) || asked.Effort != model.EffortHigh ||
		len(asked.Messages) != 19 || asked.Messages[0].Content != "井边的灯为什么不亮" || asked.Messages[17].Content != replies[8].Text {
		t.Fatalf("the digest was asked for with %d messages:\n%s", len(asked.Messages), asked.System)
	}
	got := digests(t, e, session)
	if len(got) != 1 {
		t.Fatalf("digests %+v", got)
	}
	d := got[0]
	if d.Through != replies[8].ID || d.Text != "## 在聊什么\n第1份摘要" || d.Model != "deepseek/deepseek-flash" || d.Effort != "high" || d.Reasoning != "先理一理" ||
		d.Usage == nil || d.Usage.CacheRead != 8000 || d.Sent == nil || d.Sent.Part(workspace.PartTalk) == 0 || d.Sent.Part(workspace.PartInput) == 0 {
		t.Fatalf("digest %+v", d)
	}
	if !strings.HasSuffix(replied.System, digestNote+"\n\n"+d.Text) || len(replied.Messages) != 7 || replied.Messages[0].Content != "第9轮" {
		t.Fatalf("the reply read %d messages:\n%s", len(replied.Messages), replied.System)
	}
	events, err := e.Store.Events(session)
	if err != nil || len(events) != 27 || events[0].Text != "井边的灯为什么不亮" {
		t.Fatalf("the talk did not stay: %d %v", len(events), err)
	}
	if r := events[len(events)-1]; r.Role != workspace.RoleAssistant || r.Sent.Part(workspace.PartDigest) == 0 {
		t.Fatalf("the reply did not record the digest: %+v", r.Sent)
	}
	found := (&toolbox{store: e.Store, session: session}).searchTalks(input{Query: "井边的灯"})
	if found.failed || !strings.Contains(found.text, "作者：井边的灯为什么不亮") {
		t.Fatalf("search_talks lost what was compacted: %s", found.text)
	}

	// Within the threshold, nothing more is compacted.
	if _, err = e.Say(context.Background(), session, workspace.Event{Text: "再说说"}); err != nil {
		t.Fatal(err)
	}
	if len(talk.calls) != 3 || len(digests(t, e, session)) != 1 {
		t.Fatalf("compacted within the threshold: %d requests", len(talk.calls))
	}

	// Over it again, the digest before goes into the next, which alone
	// is read after.
	long(t, e, session, 9, 700, "又一轮")
	if _, err = e.Say(context.Background(), session, workspace.Event{Text: "然后呢"}); err != nil {
		t.Fatal(err)
	}
	got = digests(t, e, session)
	asked, replied = talk.calls[3], talk.calls[4]
	if len(got) != 2 || !strings.Contains(asked.System, digestNote+"\n\n"+d.Text) || asked.Messages[0].Content != "第9轮" ||
		!strings.HasSuffix(replied.System, got[1].Text) || strings.Contains(replied.System, d.Text) {
		t.Fatalf("the digests did not merge: %+v", got)
	}
}

// A talk too long for one request is digested a request at a time, each
// taking in the digest before, until the turns left are few enough.
func TestCompactionTakesWhatOneRequestHolds(t *testing.T) {
	e, session := setup(t)
	replies := long(t, e, session, 20, 1000, "第0轮")
	talk := &digester{}
	e.Talk = Model{Model: limits, Completer: talk}
	if _, err := e.Say(context.Background(), session, workspace.Event{Text: "接着说"}); err != nil {
		t.Fatal(err)
	}
	got := digests(t, e, session)
	if len(got) != 3 || len(talk.calls) != 4 {
		t.Fatalf("%d digests in %d requests", len(got), len(talk.calls))
	}
	at := map[string]int{}
	for n, r := range replies {
		at[r.ID] = n
	}
	for n, d := range got {
		if n > 0 && (at[d.Through] <= at[got[n-1].Through] || !strings.Contains(talk.calls[n].System, got[n-1].Text)) {
			t.Fatalf("digest %d did not go on from the one before", n)
		}
	}
	if last := talk.calls[len(talk.calls)-1]; at[got[len(got)-1].Through] != 17 || len(last.Messages) != 5 {
		t.Fatalf("the reply read %d messages, after turn %d", len(last.Messages), at[got[len(got)-1].Through])
	}
}

// How big the talk is goes by what the provider counted for the last
// reply, which may be more than estimated; once the setting has changed
// since, by the estimate alone. Each digest goes to Step with no ID as it
// is asked for, then as written.
func TestCompactionGoesByWhatWasCounted(t *testing.T) {
	for _, changed := range []bool{false, true} {
		e, session := setup(t)
		long(t, e, session, 6, 700, "第0轮")
		talk := &digester{used: model.Usage{Input: 17000, Output: 100}}
		e.Talk = Model{Model: limits, Completer: talk}
		var stepped []workspace.Event
		e.Step = func(ev workspace.Event) { stepped = append(stepped, ev) }
		if _, err := e.Say(context.Background(), session, workspace.Event{Text: "接着说"}); err != nil {
			t.Fatal(err)
		}
		if len(digests(t, e, session)) != 0 {
			t.Fatal("compacted by the estimate")
		}
		if changed {
			revise(t, e, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "林晚怕黑。"})
		}
		if _, err := e.Say(context.Background(), session, workspace.Event{Text: "再说说"}); err != nil {
			t.Fatal(err)
		}
		got := digests(t, e, session)
		if changed {
			if len(got) != 0 || len(stepped) != 0 {
				t.Fatalf("compacted by a count from before the setting changed: %d", len(got))
			}
			continue
		}
		if len(got) != 1 || len(stepped) != 2 || stepped[0].Role != workspace.RoleDigest || stepped[0].ID != "" || stepped[1].ID != got[0].ID {
			t.Fatalf("%d digests, stepped %+v", len(got), stepped)
		}
	}
}

// When the digest fails, the author's words stay and the turn fails; a
// retry compacts, and reads the digest written after the words.
func TestAFailedCompactionIsRetried(t *testing.T) {
	e, session := setup(t)
	long(t, e, session, 12, 700, "第0轮")
	e.Talk = Model{Model: limits, Completer: &digester{err: errors.New("服务不可用")}}
	turn, err := e.Say(context.Background(), session, workspace.Event{Text: "接着说"})
	if err == nil || !strings.Contains(err.Error(), "整理之前的讨论失败，原文已保存：服务不可用") || turn.User.ID == "" || len(digests(t, e, session)) != 0 {
		t.Fatalf("%v %+v", err, turn)
	}
	talk := &digester{}
	e.Talk = Model{Model: limits, Completer: talk}
	if _, err = e.Retry(context.Background(), session, turn.User.ID); err != nil {
		t.Fatal(err)
	}
	replied := talk.calls[len(talk.calls)-1]
	if len(digests(t, e, session)) != 1 || !strings.Contains(replied.System, digestNote) || len(replied.Messages) != 7 || replied.Messages[6].Content != "接着说" {
		t.Fatalf("the retry read %d messages:\n%s", len(replied.Messages), replied.System)
	}
}

// A turn too long for a digest request by itself goes cut to what fits.
func TestATurnTooLongIsCut(t *testing.T) {
	long := exchange{model.Message{Role: workspace.RoleUser, Content: strings.Repeat("问", 1000)}, model.Message{Role: workspace.RoleAssistant, Content: strings.Repeat("答", 1000)}, 1}
	b := prompt.NewBudget(1512)
	msgs, took := fill(b, []exchange{long, long}, true)
	if took != 1 || len(msgs) != 2 || !strings.HasSuffix(msgs[0].Content, prompt.CutMark) || !strings.HasSuffix(msgs[1].Content, prompt.CutMark) || b.Left() < 0 {
		t.Fatalf("took %d, left %d", took, b.Left())
	}
	if cut := prompt.ClipTokens(long.user.Content, 300); prompt.Estimate(cut) > 300 || prompt.Estimate(cut+"问") <= 300 || prompt.ClipTokens("短", 300) != "短" {
		t.Fatalf("cut to %d runes", len([]rune(cut)))
	}
	if _, took = fill(prompt.NewBudget(1512), []exchange{long}, false); took != 0 {
		t.Fatal("the talk cut a turn")
	}
}
