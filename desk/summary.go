package desk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
	"writer/writing"
)

// summaries is the summaries waiting to be written, one at a time in the
// background, beside whatever the writing model is doing in the editor.
type summaries struct {
	queue   []string
	running string
	store   *workspace.Store // the work the one running is of
	cancel  context.CancelFunc
	done    chan summaryDone
	failed  map[string]string // why a chapter's last summary failed
	total   int               // the summaries asked for since the queue was empty
}

type summaryDone struct {
	chapter string
	store   *workspace.Store
	sum     workspace.Summary
	err     error
}

// queueSummary asks for chapter's summary unless it is waiting or running.
func (a *App) queueSummary(chapter string) {
	q := &a.sums
	if q.running == chapter {
		return
	}
	for _, id := range q.queue {
		if id == chapter {
			return
		}
	}
	if q.running == "" && len(q.queue) == 0 {
		q.total = 0
	}
	q.queue = append(q.queue, chapter)
	q.total++
	delete(q.failed, chapter)
}

// leaveChapter asks for the summary of the chapter shown when its prose
// changed while it was open and the summary does not say what it says now.
func (a *App) leaveChapter() {
	w := &a.write
	if !w.changed || w.node == "" || w.lost || !a.write.autoSummary {
		return
	}
	w.changed = false
	if workspace.Count(w.saved) < writing.MinSummary {
		return
	}
	sum := w.sum
	if sum.Text == "" || sum.By != "" && sum.Stale(w.saved) {
		a.queueSummary(w.node)
	}
}

// pollSummaries takes in a summary written, and starts the next.
func (a *App) pollSummaries(c *ui.Context) {
	q := &a.sums
	if q.done == nil {
		q.done = make(chan summaryDone, 1)
		q.failed = map[string]string{}
	}
	select {
	case d := <-q.done:
		q.running, q.cancel = "", nil
		if d.store == a.Store {
			a.keepSummary(d)
		}
	default:
	}
	if q.running != "" || len(q.queue) == 0 || a.Store == nil {
		return
	}
	id := q.queue[0]
	q.queue = q.queue[1:]
	pw, err := a.Pen(a.Store)
	if err != nil {
		q.failed[id] = err.Error()
		q.queue = nil
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	q.running, q.store, q.cancel = id, a.Store, cancel
	store := a.Store
	go func() {
		sum, err := pw.Summarize(ctx, id)
		cancel()
		q.done <- summaryDone{chapter: id, store: store, sum: sum, err: err}
		c.Invalidate()
	}()
}

// keepSummary saves a summary written, unless the author wrote one for the
// prose as it is meanwhile.
func (a *App) keepSummary(d summaryDone) {
	q := &a.sums
	if d.err != nil {
		q.failed[d.chapter] = d.err.Error()
		return
	}
	delete(q.failed, d.chapter)
	now, err := a.Store.ReadSummary(d.chapter)
	if err != nil {
		q.failed[d.chapter] = err.Error()
		return
	}
	if now.By == "" && now.Text != "" && now.Of == d.sum.Of {
		return
	}
	if err = a.Store.WriteSummary(d.chapter, d.sum); err != nil {
		q.failed[d.chapter] = err.Error()
		return
	}
	w := &a.write
	if w.node == d.chapter && w.summary == w.summarySaved {
		w.sum, w.summary, w.summarySaved = d.sum, d.sum.Text, d.sum.Text
	}
}

// stopSummaries drops the summaries waiting, as the work closes.
func (a *App) stopSummaries() {
	q := &a.sums
	if q.cancel != nil {
		q.cancel()
	}
	q.queue, q.total = nil, 0
}

// loadSummary reads the summary of the chapter opened.
func (a *App) loadSummary(id string) {
	w := &a.write
	sum, err := a.Store.ReadSummary(id)
	a.warn(err)
	w.sum, w.summary, w.summarySaved = sum, sum.Text, sum.Text
}

// saveSummary keeps what the author wrote as the summary of the prose as
// it is now.
func (a *App) saveSummary() error {
	w := &a.write
	if w.node == "" || w.ed == nil || w.lost || w.summary == w.summarySaved {
		return nil
	}
	text := w.ed.Doc.String()
	sum := workspace.Summary{Text: strings.TrimSpace(w.summary), Of: workspace.ProseHash(text), Words: workspace.Count(text), At: time.Now()}
	if err := a.Store.WriteSummary(w.node, sum); err != nil {
		return err
	}
	w.sum, w.summarySaved = sum, w.summary
	return nil
}

// summaryState says where the summary of the chapter shown stands.
func (a *App) summaryState() (label, line string, warn bool) {
	w := &a.write
	q := &a.sums
	switch {
	case q.running == w.node:
		return "本章摘要 · 正在生成", "AI 正在阅读本章并生成摘要…", false
	case w.sum.Text == "" && w.summary == "":
		label = "本章摘要 · 暂无"
		line = fmt.Sprintf("正文满 %d 字后，离开本章时自动生成；也可手动填写。", writing.MinSummary)
		if !w.autoSummary {
			line = "自动生成已关闭，可点击“生成摘要”，也可手动填写。"
		}
	case w.sum.By == "":
		label, line = "本章摘要", "手动填写"
	default:
		label, line = "本章摘要", "AI 生成 · "+shortModel(w.sum.By)+" · "+w.sum.At.Local().Format("01-02 15:04")
	}
	if w.sum.Text != "" && w.ed != nil && w.sum.Stale(w.ed.Doc.String()) && w.summary == w.summarySaved {
		label, warn = "本章摘要 · 可能过时", true
		line += " · 正文已修改"
	}
	if why := q.failed[w.node]; why != "" {
		line, warn = "上次生成失败："+why, true
	}
	return label, line, warn
}

func shortModel(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// summaryShown says whether the chapter shown has a summary to show: not
// over an empty chapter that has none.
func (a *App) summaryShown() bool {
	w := &a.write
	return w.sum.Text != "" || w.summary != "" || a.sums.running == w.node || w.ed == nil || w.ed.Doc.Len() > 0
}

// summaryPanel is the summary of the chapter shown, opened under its toggle.
func (a *App) summaryPanel(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	_, line, warn := a.summaryState()
	ui.Column(c).FillWidth().Gap(6).Padding(2, 0, 4, 0).Children(func() {
		ui.TextAreaBase(c, &w.summary).Label("本章摘要").Placeholder("本章发生的事。起草后续章节时，AI 会参考各章摘要").FillWidth().Height(84).
			FontSize(fsBody).LineHeight(lhLoose).Padding(10, 12).Radius(rControl).Background(a.pal.side).Disabled(w.lost || a.sums.running == w.node)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			color := a.pal.faint
			if warn {
				color = t.Warning
			}
			ui.Text(c, line).FontSize(fsNote).TextColor(color).Grow(1).Shrink(1)
			verb := "重新生成"
			if w.sum.Text == "" {
				verb = "生成摘要"
			}
			short := w.ed == nil || workspace.Count(w.ed.Doc.String()) < writing.MinSummary
			tip := "由 AI 阅读本章并生成摘要"
			if short {
				tip = fmt.Sprintf("正文满 %d 字后才需要摘要", writing.MinSummary)
			}
			if ghost(c, verb, a.pal.ink).Label(verb + "摘要").Tooltip(tip).Disabled(short || a.sums.running == w.node || w.lost).Clicked() {
				a.flush()
				w.summary = w.summarySaved
				a.queueSummary(w.node)
			}
		})
	})
}

// recapNote tells, as a draft is set up, which chapters before it the AI
// will not see for want of a summary, and offers to write them.
func (a *App) recapNote(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	q := &a.sums
	waiting := len(q.queue)
	if q.running != "" {
		waiting++
	}
	if waiting > 0 && q.total > 0 {
		ui.Textf(c, "正在补齐摘要：%d / %d。补齐前起草，AI 只能参考已有摘要。", q.total-waiting, q.total).FontSize(fsNote).TextColor(t.TextMuted)
		return
	}
	var missing []string
	for _, id := range w.unsummarized {
		if sum, err := a.Store.ReadSummary(id); err == nil && sum.Text == "" {
			missing = append(missing, id)
		}
	}
	w.unsummarized = missing
	if len(missing) == 0 {
		if w.recapped > 0 {
			ui.Textf(c, "起草时 AI 会参考前 %d 章的摘要，以衔接前文。", w.recapped).FontSize(fsNote).TextColor(t.TextMuted)
		}
		return
	}
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Padding(8, 10).Radius(rCard).Background(t.Warning.Alpha(0.08)).Children(func() {
		ui.Textf(c, "前文有 %d 章尚无摘要，起草时 AI 无法了解这些章节的内容。", len(missing)).FontSize(fsNote).TextColor(t.Warning.Mix(t.Text, 0.35)).Grow(1).Shrink(1)
		if action(c, fmt.Sprintf("补齐摘要（%d 次调用）", len(missing)), t.Warning, false).Label("补齐摘要").Clicked() {
			for _, id := range missing {
				a.queueSummary(id)
			}
		}
	})
}
