package writing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"writer/model"
	"writer/prompt"
	"writer/workspace"
)

const (
	// MinSummary is the shortest prose worth a summary.
	MinSummary    = 200
	summaryOutput = 1200
	recapCap      = 4000 // the most of the recap a draft shows
)

// finished tells whether a summary ends as a sentence does. A model now and
// then trails off into a fragment or stray code, and a summary kept as it
// is would be read before every draft after it.
func finished(sum string) bool {
	r := []rune(sum)
	return strings.ContainsRune("。！？…”」』）", r[len(r)-1])
}

// Summarize has the model sum up the chapter as it is on disk.
func (w *Writer) Summarize(ctx context.Context, chapter string) (workspace.Summary, error) {
	if w.Model.Completer == nil {
		return workspace.Summary{}, ErrNoModel
	}
	tree, err := w.Store.Chapters()
	if err != nil {
		return workspace.Summary{}, err
	}
	n, ok := tree.Node(chapter)
	if !ok || n.Folder {
		return workspace.Summary{}, errors.New("找不到这一章")
	}
	ch, err := w.Store.ReadChapter(chapter)
	if err != nil {
		return workspace.Summary{}, err
	}
	if workspace.Count(ch.Text) < MinSummary {
		return workspace.Summary{}, fmt.Errorf("这一章还不到 %d 字，不用摘要", MinSummary)
	}
	b := prompt.NewBudget(w.Model.Input())
	b.Must(workspace.PartRules, summaryRules)
	text := fit(ch.Text, b.Left(), false)
	if strings.TrimSpace(text) == "" {
		return workspace.Summary{}, errors.New("模型的上下文上限放不下摘要的要求：请在设置中「AI」配置里调大上下文上限")
	}
	note, read := "", 0
	if len(text) < len(ch.Text) {
		note, read = "（正文太长，只给了前面一部分）", workspace.Count(text)
	}
	b.Must(workspace.PartProse, text)
	req := model.Request{System: summaryRules, MaxTokens: summaryOutput, Effort: model.EffortNone,
		Messages: []model.Message{{Role: workspace.RoleUser, Content: "【章名】" + n.Title + "\n\n【正文】" + note + "\n" + text}}}
	var out strings.Builder
	if _, err = w.Model.Complete(ctx, req, func(d model.Delta) { out.WriteString(d.Text) }); err != nil {
		return workspace.Summary{}, err
	}
	sum := strings.TrimSpace(out.String())
	if sum == "" {
		return workspace.Summary{}, errors.New("模型没有给出摘要")
	}
	if !finished(sum) {
		return workspace.Summary{}, errors.New("模型给的摘要没有写完，可以重新生成")
	}
	return workspace.Summary{Text: sum, Of: workspace.ProseHash(ch.Text), Words: workspace.Count(ch.Text), Read: read, By: w.Model.Name, At: time.Now(), Sent: b.Sent(summaryRules, workspace.Setting{})}, nil
}

// earlier is the documents before n in the order of the tree.
func earlier(t workspace.Tree, n *workspace.Node) []*workspace.Node {
	var out []*workspace.Node
	done := false
	t.Walk(func(m *workspace.Node, _ int) {
		if m == n {
			done = true
		}
		if !done && !m.Folder {
			out = append(out, m)
		}
	})
	return out
}

// Unsummarized is the documents before chapter long enough for a summary
// that have none, in the order of the tree, and how many before it have one.
func Unsummarized(s *workspace.Store, chapter string) (missing []string, have int, err error) {
	tree, err := s.Chapters()
	if err != nil {
		return nil, 0, err
	}
	n, ok := tree.Node(chapter)
	if !ok {
		return nil, 0, errors.New("找不到这一章")
	}
	sums, err := s.Summaries()
	if err != nil {
		return nil, 0, err
	}
	counts, err := s.Counts()
	if err != nil {
		return nil, 0, err
	}
	for _, m := range earlier(tree, n) {
		if _, ok := sums[m.ID]; ok {
			have++
		} else if counts[m.ID] >= MinSummary {
			missing = append(missing, m.ID)
		}
	}
	return missing, have, nil
}

// recap is the summaries of the chapters before n, the nearest kept when
// they do not all fit, in the order of the tree; stale counts those made
// from prose since changed.
func (w *Writer) recap(t workspace.Tree, n *workspace.Node, budget int) (text string, chapters, stale int, err error) {
	sums, err := w.Store.Summaries()
	if err != nil {
		return "", 0, 0, err
	}
	before := earlier(t, n)
	var lines []string
	size := 0
	for i := len(before) - 1; i >= 0; i-- {
		m := before[i]
		sum, ok := sums[m.ID]
		if !ok {
			continue
		}
		line := "《" + m.Title + "》" + prompt.OneLine(sum.Text)
		cost := prompt.Estimate(line)
		if size+len([]rune(line)) > recapCap || cost > budget {
			break
		}
		size += len([]rune(line))
		budget -= cost
		lines = append([]string{line}, lines...)
		if ch, err := w.Store.ReadChapter(m.ID); err == nil && sum.Stale(ch.Text) {
			stale++
		}
	}
	return strings.Join(lines, "\n"), len(lines), stale, nil
}
