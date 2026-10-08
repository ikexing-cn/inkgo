package discussion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"writer/model"
	"writer/workspace"
)

const conflictReserve = 1024

// CheckConflicts asks which other kept ideas contradict id, and records the
// answer on both sides. It only points conflicts out; the author resolves
// them by changing either idea.
func (e *Engine) CheckConflicts(ctx context.Context, id string) error {
	target, err := e.Store.Idea(id)
	if err != nil {
		return err
	}
	if target.Status != workspace.Kept {
		return errors.New("只能检查已保留思路")
	}
	ideas, err := e.Store.Ideas()
	if err != nil {
		return err
	}
	refs := map[string]string{}
	var others []map[string]string
	for _, i := range ideas {
		if i.ID != id && i.Status == workspace.Kept {
			ref := fmt.Sprintf("k%d", len(others)+1)
			refs[ref] = i.ID
			others = append(others, map[string]string{"ref": ref, "text": i.Text})
		}
	}
	var found []string
	if len(others) > 0 {
		if found, err = e.askConflicts(ctx, target.Text, others, refs); err != nil {
			return err
		}
	}
	// Both sides retain the relationship. Only kept-kept pairs count as active;
	// rejecting/deferring an idea therefore resolves the visible conflict.
	slices.Sort(found)
	target.Conflicts = slices.Compact(found)
	if err = e.Store.SaveIdea(target); err != nil {
		return err
	}
	for _, other := range ideas {
		if other.ID == id {
			continue
		}
		had := slices.Contains(other.Conflicts, id)
		has := slices.Contains(found, other.ID)
		if had == has {
			continue
		}
		other.Conflicts = slices.DeleteFunc(other.Conflicts, func(s string) bool { return s == id })
		if has {
			other.Conflicts = append(other.Conflicts, id)
		}
		if err = e.Store.SaveIdea(other); err != nil {
			return err
		}
	}
	return nil
}

// askConflicts returns the IDs the model says contradict target.
func (e *Engine) askConflicts(ctx context.Context, target string, others []map[string]string, refs map[string]string) ([]string, error) {
	if e.Book == nil {
		return nil, errors.New("未配置记账模型")
	}
	data, err := json.Marshal(map[string]any{"target": target, "others": others})
	if err != nil {
		return nil, err
	}
	if Estimate(string(data))+Estimate(conflictRules)+conflictReserve > e.Store.Work().TokenBudget {
		return nil, errors.New("保留清单超出冲突检查预算，请提高作品预算")
	}
	raw, err := e.Book.Complete(ctx, model.Request{System: conflictRules, Messages: []model.Message{{Role: workspace.RoleUser, Content: string(data)}}, MaxTokens: conflictReserve})
	if err != nil {
		return nil, err
	}
	var answer struct {
		Conflicts []string `json:"conflicts"`
	}
	if err = decode(raw, &answer); err != nil {
		return nil, err
	}
	if answer.Conflicts == nil {
		return nil, errors.New("冲突检查缺少 conflicts 数组")
	}
	ids := []string{}
	for _, ref := range answer.Conflicts {
		id, ok := refs[ref]
		if !ok {
			return nil, errors.New("冲突检查返回了未知条目")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ConflictCount counts the contradicting pairs of kept ideas.
func ConflictCount(ideas []workspace.Idea) int {
	kept := map[string]bool{}
	for _, i := range ideas {
		if i.Status == workspace.Kept {
			kept[i.ID] = true
		}
	}
	pairs := map[[2]string]bool{}
	for _, i := range ideas {
		for _, id := range activeConflicts(i, kept) {
			if id == i.ID {
				continue
			}
			pair := [2]string{i.ID, id}
			if pair[0] > pair[1] {
				pair[0], pair[1] = pair[1], pair[0]
			}
			pairs[pair] = true
		}
	}
	return len(pairs)
}
