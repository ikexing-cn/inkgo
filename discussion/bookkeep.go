package discussion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"writer/model"
	"writer/workspace"
)

const (
	bookReserve = 4096 // tokens kept for the ledger
	maxIdeas    = 100  // per turn
)

// extracted is one idea as the bookkeeping model reports it.
type extracted struct {
	Source     string       `json:"source"`
	Quote      string       `json:"quote"`
	Occurrence int          `json:"occurrence"`
	Text       string       `json:"text"`
	Category   string       `json:"category"`
	Fact       bool         `json:"fact"`
	Similar    []similarRef `json:"similar"`
}

type similarRef struct {
	Ref        string `json:"ref"`
	Difference string `json:"difference"`
}

type ledger struct {
	Ideas []extracted `json:"ideas"`
}

// decode reads exactly one strict JSON value, allowing a ```json fence.
func decode(s string, v any) error {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json\n") && strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(strings.TrimPrefix(s, "```json\n"), "```")
	}
	d := json.NewDecoder(strings.NewReader(s))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("记账 JSON 不符合约定：%w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("记账返回了多个 JSON 值或尾随内容")
	}
	return nil
}

// Bookkeep journals a validated plan before creating files. Retrying after a
// crash replays those same IDs and never overwrites a card the author edited.
func (e *Engine) Bookkeep(ctx context.Context, session, replyID string) ([]workspace.Idea, error) {
	events, err := e.Store.Events(session)
	if err != nil {
		return nil, err
	}
	user, reply, err := findTurn(events, replyID)
	if err != nil {
		return nil, err
	}
	for _, ev := range events {
		if ev.Kind == workspace.KindBookkeeping && ev.ReplyTo == replyID {
			plan, err := readPlan(ev, session, user, reply)
			if err != nil {
				return nil, err
			}
			return e.apply(plan)
		}
	}
	plan, err := e.draftPlan(ctx, session, user, reply)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, i := range plan {
		ids = append(ids, i.ID)
	}
	if _, err = e.Store.Append(session, workspace.Event{Kind: workspace.KindBookkeeping, ReplyTo: replyID, Text: string(data), IdeaIDs: ids}); err != nil {
		return nil, err
	}
	return e.apply(plan)
}

// findTurn returns the reply replyID and the words it answered.
func findTurn(events []workspace.Event, replyID string) (user, reply workspace.Event, err error) {
	for _, ev := range events {
		if ev.ID == replyID && ev.Kind == workspace.KindMessage && ev.Role == workspace.RoleAssistant {
			reply = ev
		}
	}
	for _, ev := range events {
		if reply.ID != "" && ev.ID == reply.ReplyTo && ev.Kind == workspace.KindMessage && ev.Role == workspace.RoleUser {
			user = ev
		}
	}
	if user.ID == "" {
		return user, reply, errors.New("找不到本轮完整对话")
	}
	return user, reply, nil
}

// readPlan checks a journaled plan still describes this turn.
func readPlan(ev workspace.Event, session string, user, reply workspace.Event) ([]workspace.Idea, error) {
	var plan []workspace.Idea
	if err := decode(ev.Text, &plan); err != nil {
		return nil, fmt.Errorf("已保存的记账计划损坏：%w", err)
	}
	if plan == nil || len(plan) != len(ev.IdeaIDs) {
		return nil, errors.New("已保存的记账计划缺少条目或 ID 不匹配")
	}
	seen := map[string]bool{}
	for n, i := range plan {
		if i.ID != ev.IdeaIDs[n] || seen[i.ID] || i.Status != workspace.Unprocessed ||
			i.Origin.Session != session || (i.Origin.Message != user.ID && i.Origin.Message != reply.ID) {
			return nil, errors.New("已保存的记账计划包含重复 ID、无效状态或来源")
		}
		seen[i.ID] = true
	}
	return plan, nil
}

// draftPlan asks the bookkeeping model for this turn's ideas and turns its
// answer into new idea records, rejecting anything it cannot verify.
func (e *Engine) draftPlan(ctx context.Context, session string, user, reply workspace.Event) ([]workspace.Idea, error) {
	if e.Book == nil {
		return nil, errors.New("请先配置记账模型")
	}
	ideas, err := e.Store.Ideas()
	if err != nil {
		return nil, err
	}
	// Rejected ideas go out under local refs; the model never sees an ID.
	rejected := map[string]string{}
	var avoid []map[string]string
	for _, i := range ideas {
		if i.Status == workspace.Rejected {
			ref := fmt.Sprintf("r%d", len(avoid)+1)
			rejected[ref] = i.ID
			avoid = append(avoid, map[string]string{"ref": ref, "text": i.Text})
		}
	}
	var categories []string
	for _, c := range e.Store.Work().Categories {
		categories = append(categories, c.Name)
	}
	payload, err := json.Marshal(map[string]any{"human": user.Text, "ai": reply.Text, "categories": categories, "rejected": avoid})
	if err != nil {
		return nil, err
	}
	req := model.Request{System: bookRules, Messages: []model.Message{{Role: workspace.RoleUser, Content: string(payload)}}, MaxTokens: bookReserve}
	if Estimate(bookRules)+Estimate(string(payload))+bookReserve > e.Store.Work().TokenBudget {
		return nil, errors.New("本轮原文和放弃清单超出记账预算，请提高预算后重试；未静默省略相近检测")
	}
	raw, err := e.Book.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	var l ledger
	if err = decode(raw, &l); err != nil {
		return nil, err
	}
	if l.Ideas == nil {
		return nil, errors.New("记账缺少 ideas 数组")
	}
	if len(l.Ideas) > maxIdeas {
		return nil, fmt.Errorf("单轮思路超过 %d 条，请缩短讨论", maxIdeas)
	}
	// The author's words come first, whatever order the model used.
	slices.SortStableFunc(l.Ideas, func(a, b extracted) int {
		return boolOrder(a.Source != workspace.SourceHuman) - boolOrder(b.Source != workspace.SourceHuman)
	})
	used := map[string]bool{}
	for _, i := range ideas {
		used[i.ID] = true
	}
	seen := map[string]bool{}
	// Offsets keep the plan's order through sorting by creation time.
	base := time.Now().UTC()
	var plan []workspace.Idea
	for _, item := range l.Ideas {
		msg := reply
		switch item.Source {
		case workspace.SourceHuman:
			msg = user
		case workspace.SourceAI:
		default:
			return nil, errors.New("记账 source 必须是 human 或 ai")
		}
		if strings.TrimSpace(item.Text) == "" || item.Quote == "" || item.Occurrence < 0 {
			return nil, errors.New("记账思路缺少内容或有效原文")
		}
		start, end, ok := quoteSpan(msg.Text, item.Quote, item.Occurrence)
		if !ok {
			return nil, fmt.Errorf("记账引用无法在 %s 原文中定位", item.Source)
		}
		if !slices.Contains(categories, item.Category) {
			return nil, fmt.Errorf("记账使用未知分类 %q", item.Category)
		}
		var similar []workspace.Similarity
		for _, sim := range item.Similar {
			id, ok := rejected[sim.Ref]
			if !ok || strings.TrimSpace(sim.Difference) == "" {
				return nil, errors.New("记账相近标记指向未知的放弃条目或缺少区别说明")
			}
			similar = append(similar, workspace.Similarity{ID: id, Difference: sim.Difference})
		}
		key := fmt.Sprintf("%s:%d:%d:%s", msg.ID, start, end, item.Text)
		if seen[key] {
			continue
		}
		seen[key] = true
		id := workspace.NewID()
		for used[id] {
			id = workspace.NewID()
		}
		used[id] = true
		at := base.Add(time.Duration(len(plan)) * time.Millisecond)
		plan = append(plan, workspace.Idea{
			Schema: workspace.Schema, ID: id, Status: workspace.Unprocessed, Category: item.Category,
			Source: item.Source, Fact: item.Fact, Topic: "general", Similar: similar, Text: item.Text,
			Origin:  workspace.Origin{Session: session, Message: msg.ID, Span: workspace.Span{Start: start, End: end}},
			Created: at, Updated: at,
		})
	}
	return plan, nil
}

func boolOrder(b bool) int {
	if b {
		return 1
	}
	return 0
}

// apply creates the plan's missing ideas, keeping any already written.
func (e *Engine) apply(plan []workspace.Idea) ([]workspace.Idea, error) {
	result := make([]workspace.Idea, 0, len(plan))
	for _, idea := range plan {
		existing, err := e.Store.Idea(idea.ID)
		if err == nil {
			result = append(result, existing)
			continue
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		created, err := e.Store.NewIdea(idea)
		if err != nil {
			return nil, err
		}
		result = append(result, created)
	}
	return result, nil
}

// quoteSpan finds the occurrence-th copy of quote in text, in runes.
func quoteSpan(text, quote string, occurrence int) (int, int, bool) {
	offset := 0
	for n := 0; ; n++ {
		i := strings.Index(text[offset:], quote)
		if i < 0 {
			return 0, 0, false
		}
		i += offset
		if n == occurrence {
			start := utf8.RuneCountInString(text[:i])
			return start, start + utf8.RuneCountInString(quote), true
		}
		offset = i + len(quote)
	}
}
