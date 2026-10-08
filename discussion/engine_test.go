package discussion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"writer/model"
	"writer/workspace"
)

type fake struct {
	mu        sync.Mutex
	replies   []string
	reasoning string
	calls     []model.Request
	err       error
}

func (f *fake) Complete(ctx context.Context, r model.Request, on func(model.Delta)) (model.Reply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r)
	if f.err != nil {
		return model.Reply{}, f.err
	}
	if len(f.replies) == 0 {
		return model.Reply{}, errors.New("unexpected model call")
	}
	s := f.replies[0]
	f.replies = f.replies[1:]
	if on != nil {
		on(model.Delta{Reasoning: f.reasoning})
		on(model.Delta{Text: s})
	}
	return model.Reply{Text: s, Reasoning: f.reasoning}, nil
}

func setup(t *testing.T) (*Engine, string) {
	t.Helper()
	s, err := workspace.Create(t.TempDir(), "雨夜", "网文")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	session, err := s.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	return &Engine{Store: s}, session
}

// turn appends an exchange without a model.
func turn(t *testing.T, e *Engine, session, user, reply string) (workspace.Event, workspace.Event) {
	t.Helper()
	u, err := e.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: user})
	if err != nil {
		t.Fatal(err)
	}
	a, err := e.Store.Append(session, workspace.Event{Role: workspace.RoleAssistant, Text: reply, ReplyTo: u.ID})
	if err != nil {
		t.Fatal(err)
	}
	return u, a
}

func revise(t *testing.T, e *Engine, ops ...workspace.Op) workspace.Revision {
	t.Helper()
	r, err := e.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func setting(t *testing.T, e *Engine) workspace.Setting {
	t.Helper()
	set, err := e.Store.Setting()
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestRetryUnansweredTurnDoesNotDuplicateInput(t *testing.T) {
	e, session := setup(t)
	e.Talk = Model{Completer: &fake{err: context.Canceled}}
	turn, err := e.Send(context.Background(), session, "先讨论城市的天气")
	if !errors.Is(err, context.Canceled) || turn.User.ID == "" {
		t.Fatal(turn, err)
	}
	talk := &fake{replies: []string{"可以从长期下雨对城市生活的影响谈起。"}}
	e.Talk = Model{Completer: talk}
	retried, err := e.Retry(context.Background(), session, turn.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	events, err := e.Store.Events(session)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || retried.User.ID != turn.User.ID || events[1].ReplyTo != turn.User.ID {
		t.Fatalf("retry duplicated or lost a turn: %+v", events)
	}
	if _, err = e.Retry(context.Background(), session, turn.User.ID); err == nil || len(talk.calls) != 1 {
		t.Fatal("answered turn was retried")
	}
}

// A turn waits for the reply only: it streams, is recorded with its model
// and reasoning, and no other model is asked.
func TestTurnWaitsForTheReplyOnly(t *testing.T) {
	e, session := setup(t)
	talk := &fake{replies: []string{"可以让她修灯。"}, reasoning: "她怕黑，职业要和光有关"}
	e.Talk = Model{Completer: talk, Name: "deepseek/deepseek-flash", Effort: model.EffortMax}
	var streamed string
	e.Progress = func(d model.Delta) { streamed += d.Reasoning + d.Text }
	got, err := e.Send(context.Background(), session, "她做什么工作？")
	if err != nil {
		t.Fatal(err)
	}
	if streamed != "她怕黑，职业要和光有关可以让她修灯。" || len(talk.calls) != 1 || talk.calls[0].Effort != model.EffortMax {
		t.Fatalf("streamed %q calls %d", streamed, len(talk.calls))
	}
	events, err := e.Store.Events(session)
	if err != nil || len(events) != 2 {
		t.Fatalf("events %+v %v", events, err)
	}
	if r := events[1]; r.ID != got.Reply.ID || r.Reasoning != "她怕黑，职业要和光有关" || r.Model != "deepseek/deepseek-flash" || r.Effort != "max" {
		t.Fatalf("reply record %+v", r)
	}
}

func TestTalkSeesTheSettingAndTheTurnsItLags(t *testing.T) {
	e, session := setup(t)
	r := revise(t, e,
		workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角怕黑"},
		workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceDropped, Category: "人物", Text: "主角是转生者"},
		workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "规则", Text: "雨每年停一天"})
	var users []string
	for n := 0; n < 6; n++ {
		u := fmt.Sprintf("第%d轮", n)
		users = append(users, u)
		turn(t, e, session, u, "好")
	}
	talk := &fake{replies: []string{"嗯"}}
	e.Talk = Model{Completer: talk}
	if _, err := e.Send(context.Background(), session, "继续"); err != nil {
		t.Fatal(err)
	}
	req := talk.calls[0]
	for _, want := range []string{"设定正文（作者定下的，必须遵守）：\n【人物】\n- 主角怕黑", "已放弃（不要再推荐，换个说法也不行）：\n- 主角是转生者", "悬而未决（作者还没表态，可以讨论）：\n- 雨每年停一天"} {
		if !strings.Contains(req.System, want) {
			t.Fatalf("system lacks %q:\n%s", want, req.System)
		}
	}
	for _, op := range r.Ops {
		if strings.Contains(req.System, op.Entry) {
			t.Fatal("an entry ID reached the model")
		}
	}
	// None of the six turns is in the setting yet, so all of them are sent,
	// past the usual four.
	if len(req.Messages) != 13 || req.Messages[0].Content != users[0] {
		t.Fatalf("messages %+v", req.Messages)
	}
	events, err := e.Store.Events(session)
	if err != nil {
		t.Fatal(err)
	}
	if reply := events[len(events)-1]; len(reply.Context) != 3 || reply.Context[0].Text != "正文：主角怕黑" {
		t.Fatalf("snapshot %+v", reply.Context)
	}
}

func TestContextBudget(t *testing.T) {
	w := workspace.Work{Categories: workspace.Categories("网文")}
	set := workspace.Setting{Entries: []workspace.Entry{
		{ID: "old", Place: workspace.PlaceBody, Category: "人物", Text: strings.Repeat("旧", 30), Rev: 1},
		{ID: "new", Place: workspace.PlaceBody, Category: "人物", Text: strings.Repeat("新", 30), Rev: 2},
	}}
	w.TokenBudget = talkReserve + Estimate(talkRules) + Estimate("问") + docSlack + Estimate(strings.Repeat("新", 30)) + lineCost
	got, err := Assemble(w, set, nil, 0, "问")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Included) != 1 || got.Included[0].ID != "new" || len(got.Omitted) != 1 || got.Omitted[0].ID != "old" {
		t.Fatalf("%+v", got)
	}
	w.TokenBudget = 8192
	if _, err = Assemble(w, set, nil, 0, strings.Repeat("大", 9000)); err == nil {
		t.Fatal("oversized current input accepted")
	}
	w.TokenBudget = 16000
	events := []workspace.Event{{ID: "u", Role: "user", Text: "问"}, {ID: "a", Role: "assistant", Text: "答", ReplyTo: "u"}, {Role: "user", Text: "失败的请求"}}
	got, err = Assemble(w, workspace.Setting{}, events, 0, "本轮")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Request.Messages) != 3 || got.Request.Messages[0].Content != "问" || got.Request.Messages[2].Content != "本轮" || !strings.Contains(got.Request.System, "设定文档还是空的") {
		t.Fatal(got.Request)
	}
}

func TestExtractQuotesTheSelectedRunes(t *testing.T) {
	e, session := setup(t)
	m, err := e.Store.Append(session, workspace.Event{Role: "user", Text: "雨夜🌙，林晚怕黑。"})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := e.Extract(session, m.ID, 4, 8, "人物")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Text != "林晚怕黑" || entry.Place != workspace.PlaceBody || !entry.Author || entry.Sources[0].Quote != "林晚怕黑" {
		t.Fatal(entry)
	}
	if _, err = e.Extract(session, m.ID, 0, 100, "人物"); err == nil {
		t.Fatal("out of bounds accepted")
	}
}

// book returns a bookkeeper answering with replies.
func book(e *Engine, replies ...string) (*Bookkeeper, *fake) {
	f := &fake{replies: replies, reasoning: "想一想"}
	return &Bookkeeper{Store: e.Store, Model: Model{Completer: f, Name: "deepseek/deepseek-flash", Effort: model.EffortLow}}, f
}

func ops(v ...map[string]any) string {
	b, _ := json.Marshal(map[string]any{"ops": v})
	return string(b)
}

func TestBookkeeperWritesWhatTheRulesAllow(t *testing.T) {
	e, session := setup(t)
	kept := revise(t, e, workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角怕黑"}).Ops[0].Entry
	u, a := turn(t, e, session, "主角修路灯。她其实不是怕黑，是怕安静。", "也许可以让城市的雨从不停。")
	b, f := book(e, ops(
		map[string]any{"op": "add", "place": "body", "category": "人物", "text": "主角修路灯", "sources": []map[string]string{{"msg": "m1", "quote": "主角修路灯"}}},
		map[string]any{"op": "edit", "entry": "e1", "text": "主角怕安静", "because": map[string]string{"msg": "m1", "quote": "她其实不是怕黑，是怕安静"}},
		map[string]any{"op": "add", "place": "open", "category": "规则", "text": "雨从不停", "sources": []map[string]string{{"msg": "m2", "quote": "城市的雨从不停"}}},
	))
	did, err := b.Run(context.Background(), session)
	if err != nil || !did {
		t.Fatal(did, err)
	}
	payload := f.calls[0].Messages[0].Content
	if strings.Contains(payload, kept) || strings.Contains(payload, u.ID) || !strings.Contains(payload, `"who":"author"`) {
		t.Fatalf("payload %s", payload)
	}
	set := setting(t, e)
	if len(set.Entries) != 3 || set.Entries[0].Text != "主角怕安静" || set.Entries[1].Place != workspace.PlaceBody || set.Entries[2].Place != workspace.PlaceOpen {
		t.Fatalf("entries %+v", set.Entries)
	}
	if src := set.Entries[0].Sources; len(src) != 1 || src[0].Message != u.ID {
		t.Fatalf("the author's words did not join the sources: %+v", src)
	}
	r := set.Revisions[len(set.Revisions)-1]
	if r.By != workspace.ByBookkeeper || r.From != u.ID || r.Through != a.ID || r.Base != 1 || r.Reasoning != "想一想" || r.Effort != "low" {
		t.Fatalf("revision %+v", r)
	}
	if did, err = b.Run(context.Background(), session); did || err != nil {
		t.Fatal("covered turns were taken in again")
	}
}

func TestBookkeeperRules(t *testing.T) {
	src := func(msg, q string) []map[string]string { return []map[string]string{{"msg": msg, "quote": q}} }
	because := func(msg, q string) map[string]string { return map[string]string{"msg": msg, "quote": q} }
	for _, tc := range []struct {
		name, want string
		op         map[string]any
	}{
		{"quote not verbatim", "逐字", map[string]any{"op": "add", "place": "body", "category": "人物", "text": "主角修灯", "sources": src("m1", "主角修灯")}},
		{"unknown message", "不存在", map[string]any{"op": "add", "place": "open", "category": "人物", "text": "x", "sources": src("m9", "主角")}},
		{"AI's idea into the body", "accepted", map[string]any{"op": "add", "place": "body", "category": "规则", "text": "雨从不停", "sources": src("m2", "雨从不停")}},
		{"AI accepting itself", "作者", map[string]any{"op": "add", "place": "body", "category": "规则", "text": "雨从不停", "sources": src("m2", "雨从不停"), "accepted": because("m2", "雨从不停")}},
		{"inferred change to the body", "because", map[string]any{"op": "edit", "entry": "e1", "text": "主角不怕黑"}},
		{"inferred drop", "because", map[string]any{"op": "move", "entry": "e1", "place": "dropped"}},
		{"back from dropped", "because", map[string]any{"op": "move", "entry": "e2", "place": "open"}},
		{"the author's entry", "because", map[string]any{"op": "edit", "entry": "e3", "text": "改一改"}},
		{"unknown category", "未知分类", map[string]any{"op": "add", "place": "open", "category": "魔法", "text": "x", "sources": src("m1", "主角")}},
		{"unknown entry", "不在 setting", map[string]any{"op": "edit", "entry": "e9", "text": "x"}},
		{"ID instead of ref", "不在 setting", map[string]any{"op": "move", "entry": "20261008-1200-abcd", "place": "open"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, session := setup(t)
			revise(t, e,
				workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: "人物", Text: "主角怕黑"},
				workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceDropped, Category: "人物", Text: "主角是转生者"},
				workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceOpen, Category: "人物", Text: "主角有个妹妹"})
			turn(t, e, session, "主角修路灯。", "也许城市的雨从不停。")
			before := len(setting(t, e).Revisions)
			// The same breach twice: the correction fails too.
			b, f := book(e, ops(tc.op), ops(tc.op))
			_, err := b.Run(context.Background(), session)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v", err)
			}
			if len(f.calls) != 2 || !strings.Contains(f.calls[1].Messages[2].Content, tc.want) {
				t.Fatal("the model was not told what to correct")
			}
			if len(setting(t, e).Revisions) != before {
				t.Fatal("a breach was written")
			}
		})
	}
}

func TestBookkeeperCorrectsOnce(t *testing.T) {
	e, session := setup(t)
	turn(t, e, session, "主角修路灯。", "好的。")
	b, _ := book(e, `{"ops":[{"op":"add","place":"body","category":"人物","text":"主角修灯","sources":[{"msg":"m1","quote":"主角修灯"}]}]}`,
		`{"ops":[{"op":"add","place":"body","category":"人物","text":"主角修路灯","sources":[{"msg":"m1","quote":"主角修路灯"}]}]}`)
	if _, err := b.Run(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	set := setting(t, e)
	if len(set.Entries) != 1 || set.Entries[0].Text != "主角修路灯" || !strings.Contains(set.Revisions[0].Reasoning, "（改正时）") {
		t.Fatalf("%+v", set)
	}
}

// A batch failing every attempt is recorded and the turns after it go on;
// redoing it later takes it in.
func TestWorkerRetriesRecordsFailureAndRedoes(t *testing.T) {
	e, session := setup(t)
	u, _ := turn(t, e, session, "主角修路灯。", "好的。")
	failing := &fake{err: errors.New("服务暂时不可用")}
	var bk atomic.Pointer[Bookkeeper]
	changed := make(chan struct{}, 8)
	w := NewWorker(func() (*Bookkeeper, error) { return bk.Load(), nil }, []time.Duration{time.Millisecond, time.Millisecond}, func() { changed <- struct{}{} })
	bk.Store(&Bookkeeper{Store: e.Store, Model: Model{Completer: failing}})
	var mu sync.Mutex
	var logged []string
	w.Log = func(line string) { mu.Lock(); logged = append(logged, line); mu.Unlock() }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	w.Kick()
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("no revision")
	}
	set := setting(t, e)
	if len(failing.calls) != 3 || len(set.Revisions) != 1 || !strings.Contains(set.Revisions[0].Failed, "服务暂时不可用") || set.Revisions[0].From != u.ID {
		t.Fatalf("calls %d revisions %+v", len(failing.calls), set.Revisions)
	}
	mu.Lock()
	if log := strings.Join(logged, "|"); strings.Count(log, "失败，用时") != 3 || !strings.Contains(log, "服务暂时不可用") {
		t.Fatalf("log %q", log)
	}
	mu.Unlock()
	failed := set.Revisions[0].ID
	next, _ := book(e, ops(map[string]any{"op": "add", "place": "body", "category": "规则", "text": "雨从不停", "sources": []map[string]string{{"msg": "m3", "quote": "雨从不停"}}}))
	bk.Store(next)
	turn(t, e, session, "雨从不停。", "好。")
	w.Kick()
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("the turns after a failure did not go on")
	}
	if set = setting(t, e); len(set.Entries) != 1 || set.Entries[0].Text != "雨从不停" {
		t.Fatalf("%+v", set.Entries)
	}
	redo, _ := book(e, ops(map[string]any{"op": "add", "place": "body", "category": "人物", "text": "主角修路灯", "sources": []map[string]string{{"msg": "m1", "quote": "主角修路灯"}}}))
	if err := redo.Redo(context.Background(), failed); err != nil {
		t.Fatal(err)
	}
	if set = setting(t, e); !set.Redone(failed) || len(set.Entries) != 2 {
		t.Fatalf("redo %+v", set)
	}
}

func TestLocateForgivesCopyMarks(t *testing.T) {
	text := "**一种是它吃声音。** 那东西一动，周围的“雨声”会先被吸走。"
	for quote, want := range map[string]string{
		"它吃声音": "它吃声音",
		"一种是它吃声音。那东西一动": "一种是它吃声音。** 那东西一动",
		"周围的雨声会先被吸走":    "周围的“雨声”会先被吸走",
	} {
		if got, ok := locate(text, quote); !ok || got != want || !strings.Contains(text, got) {
			t.Errorf("locate(%q) = %q %v, want %q", quote, got, ok, want)
		}
	}
	for _, quote := range []string{"", "  ", "它不吃声音", "**"} {
		if _, ok := locate(text, quote); ok {
			t.Errorf("locate(%q) found", quote)
		}
	}
}

// The exchange before a batch goes with it, marked as covered, so "the
// second one" can be resolved and quoted.
func TestBookkeeperSeesTheExchangeBefore(t *testing.T) {
	e, session := setup(t)
	turn(t, e, session, "给我几个方向", "一是雨从不停；二是雨每年停一天。")
	b, _ := book(e, `{"ops":[]}`)
	if _, err := b.Run(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	u, _ := turn(t, e, session, "第二个不错，就这么定。", "好。")
	b, f := book(e, ops(map[string]any{"op": "add", "place": "body", "category": "规则", "text": "雨每年停一天",
		"sources": []map[string]string{{"msg": "m2", "quote": "雨每年停一天"}}, "accepted": map[string]string{"msg": "m3", "quote": "第二个不错，就这么定"}}))
	if _, err := b.Run(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	payload := f.calls[0].Messages[0].Content
	if !strings.Contains(payload, `{"ref":"m2","who":"ai","earlier":true,"text":"一是雨从不停；二是雨每年停一天。"}`) || !strings.Contains(payload, `{"ref":"m3","who":"author","text":"第二个不错，就这么定。"}`) {
		t.Fatalf("payload %s", payload)
	}
	set := setting(t, e)
	r := set.Revisions[len(set.Revisions)-1]
	if r.From != u.ID || len(set.Entries) != 1 || set.Entries[0].Place != workspace.PlaceBody {
		t.Fatalf("%+v %+v", r, set.Entries)
	}
}

// Chinese counts by the character, not by its three UTF-8 bytes.
func TestEstimateCountsCharacters(t *testing.T) {
	if got := Estimate(strings.Repeat("雨", 1000)); got != 1516 {
		t.Fatalf("1000 汉字 = %d", got)
	}
	if got := Estimate(strings.Repeat("rain ", 300)); got != 516 {
		t.Fatalf("1500 ASCII bytes = %d", got)
	}
}

// A backlog too long for one request is taken in a turn at a time, oldest
// first, instead of failing.
func TestBookkeeperSplitsALongBacklog(t *testing.T) {
	e, session := setup(t)
	long := strings.Repeat("雨", 2000)
	u1, r1 := turn(t, e, session, "第一轮", long)
	u2, r2 := turn(t, e, session, "第二轮", long)
	w := e.Store.Work()
	w.TokenBudget = Estimate(bookRules) + bookReserve + Estimate(long) + 600
	if err := e.Store.SaveWork(w); err != nil {
		t.Fatal(err)
	}
	b, _ := book(e, `{"ops":[]}`, `{"ops":[]}`)
	for range 2 {
		if did, err := b.Run(context.Background(), session); !did || err != nil {
			t.Fatal(did, err)
		}
	}
	set := setting(t, e)
	if len(set.Revisions) != 2 || set.Revisions[0].From != u1.ID || set.Revisions[0].Through != r1.ID ||
		set.Revisions[1].From != u2.ID || set.Revisions[1].Through != r2.ID {
		t.Fatalf("%+v", set.Revisions)
	}
}
