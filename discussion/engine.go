package discussion

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"writer/model"
	"writer/workspace"
)

// Engine runs the discussion of one work. Its owner serializes calls; Talk
// and Book may be nil when no model is configured, which only the calls that
// need a model refuse.
type Engine struct {
	Store *workspace.Store
	Talk  model.Completer
	Book  model.Completer
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
	if e.Talk == nil || e.Book == nil {
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
	if e.Talk == nil || e.Book == nil {
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
	reply, err := e.Talk.Complete(ctx, assembled.Request)
	if err != nil {
		return turn, fmt.Errorf("讨论失败，原文已保存：%w", err)
	}
	if strings.TrimSpace(reply) == "" {
		return turn, errors.New("模型返回空回复，原文已保存")
	}
	turn.Reply, err = e.Store.Append(session, workspace.Event{
		Kind: workspace.KindMessage, Role: workspace.RoleAssistant, Text: reply, ReplyTo: turn.User.ID,
		Context: assembled.Included, Omitted: assembled.Omitted,
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
