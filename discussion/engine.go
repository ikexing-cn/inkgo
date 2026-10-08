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

// Step is the part of a turn a model is working on.
type Step string

const (
	StepReply     Step = "reply"
	StepBookkeep  Step = "bookkeep"
	StepConflicts Step = "conflicts"
)

// Engine runs the discussion of one work. Its owner serializes calls; Talk
// and Book may be unset when no model is configured, which only the calls
// that need a model refuse.
type Engine struct {
	Store *workspace.Store
	Talk  Model
	Book  Model
	// Progress, when set, hears each step start, with an empty delta, and
	// then what its model streams, never empty. It runs on the caller's
	// goroutine.
	Progress func(Step, model.Delta)
}

// ask runs one step on m, reporting its progress.
func (e *Engine) ask(ctx context.Context, m Model, step Step, req model.Request) (model.Reply, error) {
	req.Effort = m.Effort
	var on func(model.Delta)
	if e.Progress != nil {
		e.Progress(step, model.Delta{})
		on = func(d model.Delta) { e.Progress(step, d) }
	}
	return m.Complete(ctx, req, on)
}

// Turn is one exchange: the author's words, the reply, and the ideas
// bookkeeping drew from them.
type Turn struct {
	User, Reply workspace.Event
	Ideas       []workspace.Idea
}

var errNoModel = errors.New("请先配置讨论模型")

// Send saves the author's words, then asks for a reply and bookkeeps it.
// Whatever fails after the words are saved can be retried without them
// being sent twice.
func (e *Engine) Send(ctx context.Context, session, input string) (Turn, error) {
	var turn Turn
	if strings.TrimSpace(input) == "" {
		return turn, errors.New("请输入要讨论的内容")
	}
	if !e.Talk.ready() || !e.Book.ready() {
		return turn, errNoModel
	}
	ideas, err := e.Store.Ideas()
	if err != nil {
		return turn, err
	}
	events, err := e.Store.Events(session)
	if err != nil {
		return turn, err
	}
	assembled, err := Assemble(ideas, events, input, e.Store.Work().TokenBudget)
	if err != nil {
		return turn, err
	}
	turn.User, err = e.Store.Append(session, workspace.Event{Kind: workspace.KindMessage, Role: workspace.RoleUser, Text: input})
	if err != nil {
		return turn, err
	}
	return e.respond(ctx, session, turn, assembled)
}

// Retry resumes an unanswered saved turn without appending a duplicate user message.
func (e *Engine) Retry(ctx context.Context, session, userID string) (Turn, error) {
	var turn Turn
	if !e.Talk.ready() || !e.Book.ready() {
		return turn, errNoModel
	}
	events, err := e.Store.Events(session)
	if err != nil {
		return turn, err
	}
	pos := -1
	for n, m := range events {
		if m.ID == userID && m.Kind == workspace.KindMessage && m.Role == workspace.RoleUser {
			turn.User, pos = m, n
		}
		if m.Kind == workspace.KindMessage && m.Role == workspace.RoleAssistant && m.ReplyTo == userID {
			return turn, errors.New("这条发言已有回复，请使用重试记账")
		}
	}
	if pos < 0 {
		return turn, errors.New("找不到待重试的发言")
	}
	ideas, err := e.Store.Ideas()
	if err != nil {
		return turn, err
	}
	assembled, err := Assemble(ideas, events[:pos], turn.User.Text, e.Store.Work().TokenBudget)
	if err != nil {
		return turn, err
	}
	return e.respond(ctx, session, turn, assembled)
}

func (e *Engine) respond(ctx context.Context, session string, turn Turn, assembled Context) (Turn, error) {
	reply, err := e.ask(ctx, e.Talk, StepReply, assembled.Request)
	if err != nil {
		return turn, fmt.Errorf("讨论失败，原文已保存：%w", err)
	}
	if strings.TrimSpace(reply.Text) == "" {
		return turn, errors.New("模型返回空回复，原文已保存")
	}
	turn.Reply, err = e.Store.Append(session, workspace.Event{
		Kind: workspace.KindMessage, Role: workspace.RoleAssistant, Text: reply.Text, ReplyTo: turn.User.ID,
		Context: assembled.Included, Omitted: assembled.Omitted,
		Model: e.Talk.Name, Effort: string(e.Talk.Effort), Reasoning: reply.Reasoning,
	})
	if err != nil {
		return turn, err
	}
	turn.Ideas, err = e.Bookkeep(ctx, session, turn.Reply.ID)
	if err != nil {
		return turn, fmt.Errorf("回复已保存，记账未完成，可重试：%w", err)
	}
	return turn, nil
}

// Mark persists the author's decision even if the advisory conflict check
// fails. Errors explicitly distinguish a saved decision from a failed check.
func (e *Engine) Mark(ctx context.Context, id string, status workspace.Status, reason string) error {
	if _, err := e.Store.ChangeStatus(id, status, reason); err != nil {
		return err
	}
	if status == workspace.Kept {
		if err := e.CheckConflicts(ctx, id); err != nil {
			return fmt.Errorf("保留已保存，冲突检查失败（可重试）：%w", err)
		}
	}
	return nil
}

// Manual extracts precisely the selected rune range; it does not ask a model.
func (e *Engine) Manual(session, message string, start, end int, category string) (workspace.Idea, error) {
	events, err := e.Store.Events(session)
	if err != nil {
		return workspace.Idea{}, err
	}
	for _, m := range events {
		if m.ID != message || m.Kind != workspace.KindMessage {
			continue
		}
		runes := []rune(m.Text)
		if start < 0 || end <= start || end > len(runes) {
			return workspace.Idea{}, errors.New("原文选区无效")
		}
		source := workspace.SourceAI
		if m.Role == workspace.RoleUser {
			source = workspace.SourceHuman
		}
		return e.Store.NewIdea(workspace.Idea{
			Text: string(runes[start:end]), Source: source, Category: category, Topic: "general",
			Origin: workspace.Origin{Session: session, Message: message, Span: workspace.Span{Start: start, End: end}},
		})
	}
	return workspace.Idea{}, errors.New("找不到来源消息")
}
