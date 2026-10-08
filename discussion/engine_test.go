package discussion

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"writer/model"
	"writer/workspace"
)

type fake struct {
	replies []string
	calls   []model.Request
	err     error
}

func (f *fake) Complete(ctx context.Context, r model.Request) (string, error) {
	f.calls = append(f.calls, r)
	if f.err != nil {
		return "", f.err
	}
	if len(f.replies) == 0 {
		return "", errors.New("unexpected model call")
	}
	s := f.replies[0]
	f.replies = f.replies[1:]
	return s, nil
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
func mustIdea(t *testing.T, e *Engine, text string, status workspace.Status) workspace.Idea {
	t.Helper()
	i, err := e.Store.NewIdea(workspace.Idea{Text: text, Source: "human", Category: "人物"})
	if err != nil {
		t.Fatal(err)
	}
	if status != workspace.Unprocessed {
		i, err = e.Store.ChangeStatus(i.ID, status, "作者决定")
		if err != nil {
			t.Fatal(err)
		}
	}
	return i
}

func TestRetryUnansweredTurnDoesNotDuplicateInput(t *testing.T) {
	e, session := setup(t)
	e.Talk = &fake{err: context.Canceled}
	e.Book = &fake{replies: []string{`{"ideas":[]}`}}
	turn, err := e.Send(context.Background(), session, "先讨论城市的天气")
	if !errors.Is(err, context.Canceled) || turn.User.ID == "" {
		t.Fatal(turn, err)
	}
	talk := &fake{replies: []string{"可以从长期下雨对城市生活的影响谈起。"}}
	e.Talk = talk
	retried, err := e.Retry(context.Background(), session, turn.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	events, err := e.Store.Events(session)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || retried.User.ID != turn.User.ID || events[1].ReplyTo != turn.User.ID {
		t.Fatalf("retry duplicated or lost a turn: %+v", events)
	}
	if _, err = e.Retry(context.Background(), session, turn.User.ID); err == nil || len(talk.calls) != 1 {
		t.Fatal("answered turn was retried")
	}
}

func TestTurnJournalAndReopen(t *testing.T) {
	e, session := setup(t)
	kept := mustIdea(t, e, "故事发生在雨夜", workspace.Kept)
	reject := mustIdea(t, e, "主角是捕快", workspace.Rejected)
	mustIdea(t, e, "未处理的海盗方向", workspace.Unprocessed)
	talk := &fake{replies: []string{"可以让她成为一个修灯的人。"}}
	book := &fake{replies: []string{`{"ideas":[{"source":"ai","quote":"修灯的人","occurrence":0,"text":"主角修灯","category":"人物","fact":false,"similar":[]},{"source":"human","quote":"她怕黑","occurrence":0,"text":"她怕黑","category":"人物","fact":false,"similar":[]}]}`}}
	e.Talk = talk
	e.Book = book
	turn, err := e.Send(context.Background(), session, "她怕黑，讨论一下职业。")
	if err != nil {
		t.Fatal(err)
	}
	if len(turn.Ideas) != 2 || turn.Ideas[0].Source != "human" {
		t.Fatalf("ideas %#v", turn.Ideas)
	}
	if turn.Ideas[0].Origin.Span != (workspace.Span{Start: 0, End: 3}) {
		t.Fatal(turn.Ideas[0].Origin)
	}
	// The model sees what the author decided, never an internal ID it could
	// repeat back.
	system := talk.calls[0].System
	if !strings.Contains(system, kept.Text) || !strings.Contains(system, reject.Text) || strings.Contains(system, "海盗") {
		t.Fatal(system)
	}
	for _, call := range append(talk.calls, book.calls...) {
		for _, text := range append([]string{call.System}, call.Messages[len(call.Messages)-1].Content) {
			if strings.Contains(text, kept.ID) || strings.Contains(text, reject.ID) {
				t.Fatalf("model saw an idea ID: %s", text)
			}
		}
	}
	if len(turn.Reply.Context) != 2 || strings.Contains(turn.Reply.Context[0].Text, kept.ID) {
		t.Fatal(turn.Reply.Context)
	}
	_, err = e.Store.ChangeStatus(turn.Ideas[0].ID, workspace.Deferred, "再想想")
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := e.Bookkeep(context.Background(), session, turn.Reply.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(book.calls) != 1 || repeated[0].Status != workspace.Deferred {
		t.Fatal("retry invoked model or overwrote decision")
	}
	root := e.Store.Root()
	if err = e.Store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	events, err := reopened.Events(session)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[1].Context[0].ID == "" {
		t.Fatalf("events %+v", events)
	}
	all, err := reopened.Ideas()
	if err != nil || len(all) != 5 {
		t.Fatalf("ideas %d %v", len(all), err)
	}
	if all[3].Source != workspace.SourceHuman || all[4].Source != workspace.SourceAI {
		t.Fatal("a turn's ideas lost the author-first order")
	}
}

func TestInvalidExtractionDoesNotWritePartialIdeas(t *testing.T) {
	for _, bad := range []string{
		`{"ideas":[{"source":"human","quote":"杜撰的原文","text":"x","category":"人物"}]}`,
		`{"ideas":[{"source":"human","quote":"念头","text":"x","category":"不存在"}]}`,
		`{"ideas":[{"source":"human","quote":"念头","text":"x","category":"人物","similar":[{"ref":"r9","difference":"相近"}]}]}`,
		`{"ideas":[],"ignored":true}`,
		`{} `,
		`{"ideas":[]} {"ideas":[]}`,
	} {
		t.Run(bad, func(t *testing.T) {
			e, session := setup(t)
			e.Talk = &fake{replies: []string{"好的"}}
			e.Book = &fake{replies: []string{bad}}
			turn, err := e.Send(context.Background(), session, "念头")
			if err == nil {
				t.Fatal("invalid extraction accepted")
			}
			if turn.Reply.ID == "" {
				t.Fatal("lost reply")
			}
			ideas, _ := e.Store.Ideas()
			events, _ := e.Store.Events(session)
			if len(ideas) != 0 || len(events) != 2 {
				t.Fatalf("partial extraction persisted: %d %d", len(ideas), len(events))
			}
			e.Book = &fake{replies: []string{`{"ideas":[]}`}}
			if _, err = e.Bookkeep(context.Background(), session, turn.Reply.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBookkeepingPlanReplaysMissingFiles(t *testing.T) {
	e, session := setup(t)
	user, err := e.Store.Append(session, workspace.Event{Kind: "message", Role: "user", Text: "林晚怕黑"})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := e.Store.Append(session, workspace.Event{Kind: "message", Role: "assistant", Text: "好的", ReplyTo: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	planned := []workspace.Idea{{Schema: 1, ID: workspace.NewID(), Text: "林晚怕黑", Status: workspace.Unprocessed, Category: "人物", Source: "human", Created: now, Updated: now, Origin: workspace.Origin{Session: session, Message: user.ID, Span: workspace.Span{Start: 0, End: 4}}}}
	data, _ := json.Marshal(planned)
	_, err = e.Store.Append(session, workspace.Event{Kind: "bookkeeping", ReplyTo: reply.ID, Text: string(data), IdeaIDs: []string{planned[0].ID}})
	if err != nil {
		t.Fatal(err)
	}
	// There are no model clients: recovery must use the durable plan alone.
	ideas, err := e.Bookkeep(context.Background(), session, reply.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ideas) != 1 || ideas[0].ID != planned[0].ID {
		t.Fatal(ideas)
	}
	if _, err = e.Bookkeep(context.Background(), session, reply.ID); err != nil {
		t.Fatal(err)
	}
	all, _ := e.Store.Ideas()
	if len(all) != 1 {
		t.Fatal("duplicate replay")
	}
}

func TestConflictsDoNotDecideForAuthor(t *testing.T) {
	e, _ := setup(t)
	a := mustIdea(t, e, "她怕黑", workspace.Kept)
	b := mustIdea(t, e, "她从不怕黑", workspace.Unprocessed)
	e.Book = &fake{replies: []string{`{"conflicts":["k1"]}`}}
	if err := e.Mark(context.Background(), b.ID, workspace.Kept, "方向不错"); err != nil {
		t.Fatal(err)
	}
	ideas, _ := e.Store.Ideas()
	if ConflictCount(ideas) != 1 {
		t.Fatal(ideas)
	}
	for _, i := range ideas {
		if i.Status != workspace.Kept || len(i.Conflicts) != 1 {
			t.Fatal(i)
		}
	}
	if err := e.Mark(context.Background(), a.ID, workspace.Rejected, "取另一条"); err != nil {
		t.Fatal(err)
	}
	ideas, _ = e.Store.Ideas()
	if ConflictCount(ideas) != 0 {
		t.Fatal("inactive conflict counted")
	}
	if err := e.Mark(context.Background(), a.ID, workspace.Kept, ""); err == nil {
		t.Fatal("rejected must restore to deferred first")
	}
}

func TestConflictFailurePreservesHumanDecision(t *testing.T) {
	e, _ := setup(t)
	mustIdea(t, e, "怕黑", workspace.Kept)
	b := mustIdea(t, e, "不怕黑", workspace.Unprocessed)
	e.Book = &fake{err: errors.New("offline")}
	err := e.Mark(context.Background(), b.ID, workspace.Kept, "保留")
	if err == nil {
		t.Fatal("expected check warning")
	}
	saved, _ := e.Store.Idea(b.ID)
	if saved.Status != workspace.Kept || len(saved.History) != 1 {
		t.Fatal(saved)
	}
}

func TestContextBudgetAndRecentPairs(t *testing.T) {
	now := time.Now()
	ideas := []workspace.Idea{{ID: "new", Status: workspace.Kept, Text: strings.Repeat("新", 30), Updated: now}, {ID: "old", Status: workspace.Deferred, Text: strings.Repeat("旧", 30), Updated: now.Add(-time.Hour)}, {ID: "pending", Status: workspace.Unprocessed, Text: "未定"}, {ID: "superseded", Status: workspace.Superseded, Text: "过时"}}
	budget := talkReserve + Estimate(talkRules) + Estimate("问") + poolSlack + Estimate(describe(ideas[0])) + refCost
	got, err := Assemble(ideas, nil, "问", budget)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Included) != 1 || got.Included[0].ID != "new" || len(got.Omitted) != 1 || got.Omitted[0].ID != "old" {
		t.Fatalf("%+v", got)
	}
	if _, err = Assemble(nil, nil, strings.Repeat("大", 9000), 8192); err == nil {
		t.Fatal("oversized current input accepted")
	}
	events := []workspace.Event{{ID: "u", Kind: "message", Role: "user", Text: "问"}, {ID: "a", Kind: "message", Role: "assistant", Text: "答", ReplyTo: "u"}, {Kind: "bookkeeping", Text: "不能发给讨论模型的计划"}, {Kind: "message", Role: "user", Text: "失败的请求"}}
	got, err = Assemble(nil, events, "本轮", 16000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Request.Messages) != 3 || got.Request.Messages[0].Content != "问" || got.Request.Messages[2].Content != "本轮" {
		t.Fatal(got.Request.Messages)
	}
}

func TestManualRuneSelection(t *testing.T) {
	e, session := setup(t)
	m, err := e.Store.Append(session, workspace.Event{Kind: "message", Role: "user", Text: "雨夜🌙，林晚怕黑。"})
	if err != nil {
		t.Fatal(err)
	}
	idea, err := e.Manual(session, m.ID, 4, 8, "人物")
	if err != nil {
		t.Fatal(err)
	}
	if idea.Text != "林晚怕黑" || idea.Source != "human" {
		t.Fatal(idea)
	}
	if _, err = e.Manual(session, m.ID, 0, 100, "人物"); err == nil {
		t.Fatal("out of bounds accepted")
	}
}

func TestSimilarRefsMapToRejectedIdeas(t *testing.T) {
	e, session := setup(t)
	rejected := mustIdea(t, e, "主角是捕快", workspace.Rejected)
	e.Talk = &fake{replies: []string{"也可以让她在衙门当差。"}}
	book := &fake{replies: []string{`{"ideas":[{"source":"ai","quote":"在衙门当差","occurrence":0,"text":"主角在衙门当差","category":"人物","fact":false,"similar":[{"ref":"r1","difference":"同属官差，只换了说法"}]}]}`}}
	e.Book = book
	turn, err := e.Send(context.Background(), session, "她做什么工作？")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(book.calls[0].Messages[0].Content, `"ref":"r1"`) {
		t.Fatal(book.calls[0].Messages[0].Content)
	}
	if len(turn.Ideas) != 1 || len(turn.Ideas[0].Similar) != 1 || turn.Ideas[0].Similar[0].ID != rejected.ID {
		t.Fatalf("similar ref not mapped: %+v", turn.Ideas)
	}
}
