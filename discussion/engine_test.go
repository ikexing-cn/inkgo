package discussion

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

type fake struct {
	mu        sync.Mutex
	steps     []model.Reply // answered before replies
	replies   []string
	reasoning string
	calls     []model.Request
	purposes  []workspace.Purpose // of each request, as its context tells
	err       error
	usage     model.Usage
}

func (f *fake) Complete(ctx context.Context, r model.Request, on func(model.Delta)) (model.Reply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r)
	f.purposes = append(f.purposes, workspace.PurposeOf(ctx))
	if f.err != nil {
		return model.Reply{}, f.err
	}
	if len(f.steps) > 0 {
		r := f.steps[0]
		f.steps = f.steps[1:]
		return r, nil
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
	return model.Reply{Text: s, Reasoning: f.reasoning, Usage: f.usage}, nil
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
	e.Talk = Model{Model: limits, Completer: &fake{err: context.Canceled}}
	turn, err := e.Say(context.Background(), session, workspace.Event{Text: "先讨论城市的天气"})
	if !errors.Is(err, context.Canceled) || turn.User.ID == "" {
		t.Fatal(turn, err)
	}
	talk := &fake{replies: []string{"可以从长期下雨对城市生活的影响谈起。"}}
	e.Talk = Model{Model: limits, Completer: talk}
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

// A turn waits for the reply only: it streams, is recorded with its model,
// its reasoning and what the provider says it used, and no other model is
// asked. The reply is asked for all the model gives; cut off there, the
// author is told the most it gives and where to give more.
func TestTurnWaitsForTheReplyOnly(t *testing.T) {
	e, session := setup(t)
	talk := &fake{replies: []string{"可以让她修灯。"}, reasoning: "她怕黑，职业要和光有关", usage: model.Usage{Input: 900, CacheRead: 600, Output: 40, Reasoning: 12}}
	e.Talk = Model{Model: limits, Completer: talk, Name: "deepseek/deepseek-flash", Effort: model.EffortMax}
	var streamed string
	e.Progress = func(d model.Delta) { streamed += d.Reasoning + d.Text }
	got, err := e.Say(context.Background(), session, workspace.Event{Text: "她做什么工作？"})
	if err != nil {
		t.Fatal(err)
	}
	if streamed != "她怕黑，职业要和光有关可以让她修灯。" || len(talk.calls) != 1 || talk.calls[0].Effort != model.EffortMax || talk.calls[0].MaxTokens != limits.Output {
		t.Fatalf("streamed %q calls %d", streamed, len(talk.calls))
	}
	events, err := e.Store.Events(session)
	if err != nil || len(events) != 2 {
		t.Fatalf("events %+v %v", events, err)
	}
	if r := events[1]; r.ID != got.Reply.ID || r.Reasoning != "她怕黑，职业要和光有关" || r.Model != "deepseek/deepseek-flash" || r.Effort != "max" ||
		r.Usage == nil || *r.Usage != (workspace.Usage{Input: 900, CacheRead: 600, Output: 40, Reasoning: 12}) {
		t.Fatalf("reply record %+v", r)
	}

	cut := Model{Model: limits, Completer: &fake{err: fmt.Errorf("deepseek: %w", model.ErrTruncated)}}
	_, err = cut.Complete(context.Background(), model.Request{}, nil)
	if err == nil || err.Error() != "回复超出模型的最大输出 4K（含思考）；请在设置中「AI」配置里调大" {
		t.Fatalf("cut off at the most: %v", err)
	}
	if _, err = cut.Complete(context.Background(), model.Request{MaxTokens: 400}, nil); !errors.Is(err, model.ErrTruncated) || strings.Contains(err.Error(), "设置") {
		t.Fatalf("cut off at what was asked: %v", err)
	}
}

// The talk sees the setting, each entry under its number and never its
// ID, every exchange not compacted, and the tools.
func TestTalkSeesTheSettingAndTheTurns(t *testing.T) {
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
	e.Talk = Model{Model: limits, Completer: talk}
	if _, err := e.Say(context.Background(), session, workspace.Event{Text: "继续"}); err != nil {
		t.Fatal(err)
	}
	req := talk.calls[0]
	for _, want := range []string{"设定正文（作者定下的，必须遵守）：\n【人物】\n- [1] 主角怕黑", "已放弃（不要再推荐，换个说法也不行）：\n- [2] 主角是转生者", "悬而未决（作者还没表态，可以讨论）：\n- [3] 雨每年停一天"} {
		if !strings.Contains(req.System, want) {
			t.Fatalf("system lacks %q:\n%s", want, req.System)
		}
	}
	for _, op := range r.Ops {
		if strings.Contains(req.System, op.Entry) {
			t.Fatal("an entry ID reached the model")
		}
	}
	if len(req.Messages) != 13 || req.Messages[0].Content != users[0] || len(req.Tools) != 6 || req.NoCalls {
		t.Fatalf("messages %+v", req.Messages)
	}
	events, err := e.Store.Events(session)
	if err != nil {
		t.Fatal(err)
	}
	if reply := events[len(events)-1]; len(reply.Context) != 3 || reply.Context[0].Text != "主角怕黑" {
		t.Fatalf("snapshot %+v", reply.Context)
	}
}

// The setting goes whole while it fits after the talk, and as an index
// when it does not; the index must fit, and so must the author's words.
func TestContextBudget(t *testing.T) {
	w := workspace.Work{Categories: workspace.Categories("网文")}
	set := workspace.Setting{Entries: []workspace.Entry{
		{ID: "old", Place: workspace.PlaceBody, Category: "人物", Names: []string{"林晚"}, Text: "林晚怕黑。" + strings.Repeat("旧", 300), Rev: 1},
		{ID: "new", Place: workspace.PlaceOpen, Category: "人物", Text: "她有个妹妹？" + strings.Repeat("新", 300), Rev: 2},
	}, Revisions: make([]workspace.Revision, 2)}
	whole := prompt.Render(w, set, talkPlaces, nil, numbered(set, prompt.Whole))
	rules := prompt.Estimate(withTools(talkRules, tools(w)))
	room := 512 + rules + prompt.Estimate("问") + prompt.Estimate(whole)
	got, err := Assemble(w, room, set, workspace.Tree{}, nil, "问", Along{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got.Request.System, whole) || len(got.Included) != 2 || !reflect.DeepEqual(got.Included[0], workspace.ContextItem{ID: "old", Text: "林晚：" + set.Entries[0].Text}) || got.Included[1].Text != "悬而未决："+set.Entries[1].Text {
		t.Fatalf("whole: %+v", got.Included)
	}
	if s := got.Sent; s.Setting != 2 || s.Part(workspace.PartSetting) != prompt.Estimate(whole) || s.Part(workspace.PartIndex) != 0 || s.Part(workspace.PartRules) != rules || s.Part(workspace.PartInput) == 0 || got.Left != 0 {
		t.Fatalf("sent %+v", s)
	}

	got, err = Assemble(w, room-1, set, workspace.Tree{}, nil, "问", Along{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Request.System, indexNote+"\n\n设定正文（作者定下的，必须遵守）：\n【人物】\n- [1] 人物 · 林晚：林晚怕黑\n\n悬而未决（作者还没表态，可以讨论）：\n- [2] 人物：她有个妹妹？") ||
		strings.Contains(got.Request.System, "旧旧") || got.Sent.Part(workspace.PartIndex) == 0 || got.Sent.Part(workspace.PartSetting) != 0 ||
		len(got.Included) != 2 || got.Included[0].Text != "人物 · 林晚：林晚怕黑" {
		t.Fatalf("index:\n%s\n%+v", got.Request.System, got.Included)
	}

	// The talk comes before the whole setting: with it, the index goes.
	// What a reply looked up on its way is not sent again.
	events := []workspace.Event{{ID: "u", Role: "user", Text: "问"}, {ID: "c", Role: "call", ReplyTo: "u", Calls: []workspace.Call{{ID: "1", Name: toolLookUp}}},
		{ID: "r", Role: "result", ReplyTo: "u", Call: "1", Text: "查到的"}, {ID: "a", Role: "assistant", Text: "答", ReplyTo: "u"}, {Role: "user", Text: "失败的请求"}}
	got, err = Assemble(w, room, set, workspace.Tree{}, events, "问", Along{})
	if err != nil || len(got.Request.Messages) != 3 || got.Sent.Part(workspace.PartTalk) == 0 || got.Sent.Part(workspace.PartIndex) == 0 {
		t.Fatalf("talk %v %+v", err, got.Sent)
	}

	if _, err = Assemble(w, 8192, set, workspace.Tree{}, nil, strings.Repeat("大", 9000), Along{}); err == nil || !strings.Contains(err.Error(), "设置中「AI」配置里调大上下文上限") {
		t.Fatalf("oversized current input: %v", err)
	}
	index := indexNote + "\n\n" + prompt.Render(w, set, talkPlaces, nil, numbered(set, prompt.Short))
	if _, err = Assemble(w, 512+rules+prompt.Estimate("问")+prompt.Estimate(index)-1, set, workspace.Tree{}, nil, "问", Along{}); err == nil || !strings.Contains(err.Error(), "设定的目录超出模型的上下文上限") {
		t.Fatalf("an index too big: %v", err)
	}
	got, err = Assemble(w, 16000, workspace.Setting{}, workspace.Tree{}, events, "本轮", Along{})
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

// limits are a test model's: room for a work of some size.
var limits = model.Model{ID: "test", Window: 20000, Output: 4000, CompactAt: 80}
