package discussion

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

// Model is a model client with the settings it runs at. A request sends
// at most Input tokens, and its reply is cut at Output.
type Model struct {
	model.Completer
	model.Model
	Name   string // provider/model, recorded with what it writes
	Effort model.Effort
}

func (m Model) ready() bool { return m.Completer != nil }

// Complete asks no more of the reply than the model gives.
func (m Model) Complete(ctx context.Context, req model.Request, on func(model.Delta)) (model.Reply, error) {
	if req.MaxTokens == 0 || req.MaxTokens > m.Output {
		req.MaxTokens = m.Output
	}
	return m.Completer.Complete(ctx, req, on)
}

// ask runs a request on m, streaming to on when it is set.
func ask(ctx context.Context, m Model, req model.Request, on func(model.Delta)) (model.Reply, error) {
	req.Effort = m.Effort
	return m.Complete(ctx, req, on)
}

// Engine runs the talk of one work: a reply may look things up in the book
// and note the setting with tools, a step at a time. Its owner serializes
// calls; Talk may be unset when no model is configured, which only the
// calls that need a model refuse.
type Engine struct {
	Store *workspace.Store
	Talk  Model
	// Progress, when set, hears what the reply streams, never empty, and
	// Step each step's calls and each call's result as they are written.
	// Both run on the caller's goroutine.
	Progress func(model.Delta)
	Step     func(workspace.Event)
	// Beside, when set, is what the author is writing as they talk, sent
	// with the turn.
	Beside *Writing
}

// Turn is one exchange: the author's words and the reply.
type Turn struct {
	User, Reply workspace.Event
}

var errNoModel = errors.New("请先配置讨论模型")

// Say saves the author's words, with the prose they quote and the chapter
// they were writing, then asks for a reply. If the reply fails, the words
// stay saved and Retry asks again without sending them twice.
func (e *Engine) Say(ctx context.Context, session string, words workspace.Event) (Turn, error) {
	var turn Turn
	if strings.TrimSpace(words.Text) == "" {
		return turn, errors.New("请输入要讨论的内容")
	}
	if !e.Talk.ready() {
		return turn, errNoModel
	}
	words = workspace.Event{Role: workspace.RoleUser, Text: words.Text, Quotes: words.Quotes, Chapter: words.Chapter}
	if _, err := e.assemble(session, -1, words.Said()); err != nil {
		return turn, err
	}
	user, err := e.Store.Append(session, words)
	if err != nil {
		return turn, err
	}
	turn.User = user
	return e.answer(ctx, session, turn, -1)
}

// Retry resumes an unanswered saved turn without appending a duplicate user message.
func (e *Engine) Retry(ctx context.Context, session, userID string) (Turn, error) {
	var turn Turn
	if !e.Talk.ready() {
		return turn, errNoModel
	}
	events, err := e.Store.Events(session)
	if err != nil {
		return turn, err
	}
	pos := -1
	for n, m := range events {
		if m.ID == userID && m.Role == workspace.RoleUser {
			turn.User, pos = m, n
		}
		if m.Role == workspace.RoleAssistant && m.ReplyTo == userID {
			return turn, errors.New("这条发言已有回复")
		}
	}
	if pos < 0 {
		return turn, errors.New("找不到待重试的发言")
	}
	return e.answer(ctx, session, turn, pos)
}

// answer brings the talk before event before, or all of it for -1, within
// the model's threshold, then asks for the reply to the turn.
func (e *Engine) answer(ctx context.Context, session string, turn Turn, before int) (Turn, error) {
	input := turn.User.Said()
	if err := e.compact(ctx, session, before, input); err != nil {
		return turn, fmt.Errorf("整理之前的讨论失败，原文已保存：%w", err)
	}
	assembled, err := e.assemble(session, before, input)
	if err != nil {
		return turn, err
	}
	return e.respond(ctx, session, turn, assembled)
}

// assemble builds the request for input from the setting and the talk
// before event before, or all of it for -1.
func (e *Engine) assemble(session string, before int, input string) (Context, error) {
	set, events, err := e.read(session, before)
	if err != nil {
		return Context{}, err
	}
	return Assemble(e.Store.Work(), e.Talk.Input(), set, events, input, e.Beside)
}

// read is the setting and the talk before event before, or all of it for
// -1. The digests written after it come along: a turn retried may have
// been compacted for.
func (e *Engine) read(session string, before int) (workspace.Setting, []workspace.Event, error) {
	set, err := e.Store.Setting()
	if err != nil {
		return set, nil, err
	}
	events, err := e.Store.Events(session)
	if err != nil || before < 0 {
		return set, events, err
	}
	talk := slices.Clip(events[:before])
	for _, ev := range events[before:] {
		if ev.Role == workspace.RoleDigest {
			talk = append(talk, ev)
		}
	}
	return set, talk, nil
}

// maxSteps bounds the requests of one reply: the last one may call no tool
// and answers from what was found.
const maxSteps = 10

// capNote tells the model of the last step it may look nothing more up.
const capNote = "\n\n（已达到本次查阅的上限，不能再查了，请用已经查到的内容回答。）"

// fullNote stands for what a tool returned when there is no room left for it.
const fullNote = "（这次查到的内容太多，放不下了；不能再查了，请用已经查到的内容回答。）"

// respond asks for the reply, one step at a time: each step's calls are
// written before they run and each result after, then the next step sees
// them. The reply records what all its steps used.
func (e *Engine) respond(ctx context.Context, session string, turn Turn, assembled Context) (Turn, error) {
	box := &toolbox{store: e.Store, session: session, turn: turn.User.ID, model: e.Talk}
	req := assembled.Request
	var used model.Usage
	var thought []string
	looked, capped := 0, false
	write := func(ev workspace.Event) error {
		ev.ReplyTo = turn.User.ID
		ev, err := e.Store.Append(session, ev)
		if err == nil && e.Step != nil {
			e.Step(ev)
		}
		return err
	}
	for step := 1; ; step++ {
		capped = step == maxSteps
		req.NoCalls = req.NoCalls || capped
		reply, err := ask(ctx, e.Talk, req, e.Progress)
		if err != nil {
			return turn, fmt.Errorf("讨论失败，原文已保存：%w", err)
		}
		used = used.Add(reply.Usage)
		if r := strings.TrimSpace(reply.Reasoning); r != "" {
			thought = append(thought, r)
		}
		if len(reply.Calls) == 0 || req.NoCalls {
			if strings.TrimSpace(reply.Text) == "" {
				return turn, errors.New("模型返回空回复，原文已保存")
			}
			sent := withLooked(assembled.Sent, looked)
			turn.Reply, err = e.Store.Append(session, workspace.Event{
				Role: workspace.RoleAssistant, Text: reply.Text, ReplyTo: turn.User.ID,
				Context: assembled.Included, Sent: sent, Capped: capped,
				Model: e.Talk.Name, Effort: string(e.Talk.Effort), Reasoning: strings.Join(thought, "\n\n"), Usage: (*workspace.Usage)(&used),
			})
			return turn, err
		}
		calls := make([]workspace.Call, len(reply.Calls))
		for n, c := range reply.Calls {
			calls[n] = workspace.Call{ID: c.ID, Name: c.Name, Input: c.Input, Label: box.label(c)}
		}
		if err = write(workspace.Event{Role: workspace.RoleCall, Text: reply.Text, Calls: calls, Model: e.Talk.Name, Effort: string(e.Talk.Effort)}); err != nil {
			return turn, err
		}
		req.Messages = append(slices.Clip(req.Messages), model.Message{Role: workspace.RoleAssistant, Content: reply.Text, Calls: reply.Calls, Thinking: reply.Thinking})
		for _, c := range reply.Calls {
			if ctx.Err() != nil {
				return turn, fmt.Errorf("讨论失败，原文已保存：%w", ctx.Err())
			}
			out := box.run(c)
			if err = write(workspace.Event{Role: workspace.RoleResult, Call: c.ID, Text: out.text, Failed: out.failed, Revision: out.revision}); err != nil {
				return turn, err
			}
			told := out.text
			if looked+prompt.Estimate(told) > assembled.Left {
				told, req.NoCalls = fullNote, true
			}
			looked += prompt.Estimate(told)
			req.Messages = append(req.Messages, model.Message{Role: "tool", CallID: c.ID, Content: told})
		}
		if step+1 == maxSteps {
			last := &req.Messages[len(req.Messages)-1]
			last.Content += capNote
		}
	}
}

// withLooked is sent with what the tools returned, in its place among the
// parts.
func withLooked(sent *workspace.Sent, looked int) *workspace.Sent {
	if sent == nil || looked == 0 {
		return sent
	}
	out := *sent
	out.Parts = append([]workspace.Part{}, sent.Parts...)
	out.Parts = append(out.Parts, workspace.Part{Name: workspace.PartLooked, Tokens: looked})
	slices.SortStableFunc(out.Parts, func(a, b workspace.Part) int {
		return slices.Index(workspace.Parts, a.Name) - slices.Index(workspace.Parts, b.Name)
	})
	return &out
}

// Extract adds the selected runes of a message to the setting's body as
// the author's entry, quoting them as its source; it asks no model.
func (e *Engine) Extract(session, message string, start, end int, category string) (workspace.Entry, error) {
	events, err := e.Store.Events(session)
	if err != nil {
		return workspace.Entry{}, err
	}
	for _, m := range events {
		if m.ID != message {
			continue
		}
		runes := []rune(m.Text)
		if start < 0 || end <= start || end > len(runes) {
			return workspace.Entry{}, errors.New("原文选区无效")
		}
		text := string(runes[start:end])
		if strings.TrimSpace(text) == "" {
			return workspace.Entry{}, errors.New("选中的只有空白")
		}
		r, err := e.Store.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
			Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: category, Text: strings.TrimSpace(text),
			Sources: []workspace.Source{{Session: session, Message: message, Quote: text}},
		}}})
		if err != nil {
			return workspace.Entry{}, err
		}
		set, err := e.Store.Setting()
		if err != nil {
			return workspace.Entry{}, err
		}
		entry, _ := set.Entry(r.Ops[0].Entry)
		return entry, nil
	}
	return workspace.Entry{}, errors.New("找不到来源消息")
}
