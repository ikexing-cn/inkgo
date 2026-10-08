package discussion

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"writer/model"
	"writer/workspace"
)

// Model is a model client with the settings it runs at.
type Model struct {
	model.Completer
	Name   string // provider/model, recorded with what it writes
	Effort model.Effort
}

func (m Model) ready() bool { return m.Completer != nil }

// ask runs a request on m, streaming to on when it is set.
func ask(ctx context.Context, m Model, req model.Request, on func(model.Delta)) (model.Reply, error) {
	req.Effort = m.Effort
	return m.Complete(ctx, req, on)
}

// Engine runs the talk of one work: a turn waits for the reply only, and
// the bookkeeper brings the setting up to date in the background. Its
// owner serializes calls; Talk may be unset when no model is configured,
// which only the calls that need a model refuse.
type Engine struct {
	Store *workspace.Store
	Talk  Model
	// Progress, when set, hears what the reply streams, never empty. It
	// runs on the caller's goroutine.
	Progress func(model.Delta)
}

// Turn is one exchange: the author's words and the reply.
type Turn struct {
	User, Reply workspace.Event
}

var errNoModel = errors.New("请先配置讨论模型")

// Send saves the author's words, then asks for a reply. If the reply fails,
// the words stay saved and Retry asks again without sending them twice.
func (e *Engine) Send(ctx context.Context, session, input string) (Turn, error) {
	var turn Turn
	if strings.TrimSpace(input) == "" {
		return turn, errors.New("请输入要讨论的内容")
	}
	if !e.Talk.ready() {
		return turn, errNoModel
	}
	assembled, err := e.assemble(session, -1, input)
	if err != nil {
		return turn, err
	}
	turn.User, err = e.Store.Append(session, workspace.Event{Role: workspace.RoleUser, Text: input})
	if err != nil {
		return turn, err
	}
	return e.respond(ctx, session, turn, assembled)
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
	assembled, err := e.assemble(session, pos, turn.User.Text)
	if err != nil {
		return turn, err
	}
	return e.respond(ctx, session, turn, assembled)
}

// assemble builds the request for input from the setting and the talk
// before event before, or all of it for -1.
func (e *Engine) assemble(session string, before int, input string) (Context, error) {
	set, err := e.Store.Setting()
	if err != nil {
		return Context{}, err
	}
	events, err := e.Store.Events(session)
	if err != nil {
		return Context{}, err
	}
	if before >= 0 {
		events = events[:before]
	}
	return Assemble(e.Store.Work(), set, events, set.Processed(session, events), input)
}

func (e *Engine) respond(ctx context.Context, session string, turn Turn, assembled Context) (Turn, error) {
	reply, err := ask(ctx, e.Talk, assembled.Request, e.Progress)
	if err != nil {
		return turn, fmt.Errorf("讨论失败，原文已保存：%w", err)
	}
	if strings.TrimSpace(reply.Text) == "" {
		return turn, errors.New("模型返回空回复，原文已保存")
	}
	turn.Reply, err = e.Store.Append(session, workspace.Event{
		Role: workspace.RoleAssistant, Text: reply.Text, ReplyTo: turn.User.ID,
		Context: assembled.Included, Omitted: assembled.Omitted,
		Model: e.Talk.Name, Effort: string(e.Talk.Effort), Reasoning: reply.Reasoning,
	})
	return turn, err
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
