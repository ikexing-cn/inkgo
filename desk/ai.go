package desk

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/egoist/mygo/ui"
	"writer/discussion"
	"writer/model"
	"writer/workspace"
	"writer/writing"
)

// newWriter builds the writer of s with its writing model, or the
// discussion's when the work leaves it empty.
func (a *App) newWriter(s *workspace.Store) (*writing.Writer, error) {
	work := s.Work()
	sel := work.Writing
	if sel.Provider == "" {
		sel.Provider, sel.Model = work.Discussion.Provider, work.Discussion.Model
	}
	if sel.Provider == "" || sel.Model == "" {
		return nil, writing.ErrNoModel
	}
	client, err := model.NewClient(a.Config, sel.Provider, sel.Model)
	if err != nil {
		return nil, err
	}
	return &writing.Writer{Store: s, Model: discussion.Model{Completer: client, Name: sel.Provider + "/" + sel.Model, Effort: model.Effort(sel.Effort)}}, nil
}

// pen continues the text of a chapter's editor with the writer the page
// keeps up to date; it runs off the UI thread.
type pen struct {
	writer  *atomic.Pointer[writing.Writer]
	chapter string
}

func (p pen) Predict(ctx context.Context, before string) (string, error) {
	w := p.writer.Load()
	if w == nil {
		return "", writing.ErrNoModel
	}
	return w.Predict(ctx, writing.Scene{Chapter: p.chapter, Before: before})
}

// keepPen rebuilds the writer the editors predict with when the model
// settings change.
func (a *App) keepPen() {
	w := &a.write
	work := a.Store.Work()
	key := fmt.Sprint(work.Writing, work.Discussion, len(a.Config.Providers))
	if key == w.penKey {
		return
	}
	w.penKey = key
	pw, err := a.Pen(a.Store)
	w.pen.Store(pw)
	w.penErr = err
}

// aiRun is a request of the writing model in flight: what it streams
// waits here for the UI thread.
type aiRun struct {
	op     string
	node   string
	job    writing.Job
	cancel context.CancelFunc
	a, b   int    // the words a rewrite replaces
	lead   string // what goes before a draft's first words
	mu     sync.Mutex
	chunks []string
	done   bool
	text   string
	err    error
	wrote  int // the characters a draft wrote so far
}

func (a *App) start(c *ui.Context, pw *writing.Writer, r *aiRun) {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	a.write.run = r
	go func() {
		text, err := pw.Run(ctx, r.job, func(s string) {
			if r.op != workspace.AIDraft {
				return
			}
			r.mu.Lock()
			r.chunks = append(r.chunks, s)
			r.mu.Unlock()
			c.Invalidate()
		})
		r.mu.Lock()
		r.done, r.text, r.err = true, text, err
		if ctx.Err() != nil {
			r.err = context.Canceled
		}
		r.mu.Unlock()
		cancel()
		c.Invalidate()
	}()
}

// pollAI takes in what the request in flight streamed, and its end.
func (a *App) pollAI() {
	w := &a.write
	r := w.run
	if r == nil {
		return
	}
	r.mu.Lock()
	chunks, done := r.chunks, r.done
	r.chunks = nil
	r.mu.Unlock()
	if r.op == workspace.AIDraft {
		for _, s := range chunks {
			r.wrote += workspace.Count(s)
			w.ed.DraftAppend(r.lead + s)
			r.lead = ""
		}
	}
	if done {
		a.finishAI(r)
	}
}

// finishAI ends the request r, whether it finished, failed or was stopped.
func (a *App) finishAI(r *aiRun) {
	w := &a.write
	w.run = nil
	stopped := errors.Is(r.err, context.Canceled)
	switch r.op {
	case workspace.AIDraft:
		w.ed.EndDraft(r.job.Record.Model)
		a.warn(a.saveText())
		switch {
		case stopped:
			a.notice = "已停止起草，写出的部分保留了；撤销可以整段撤回"
		case r.err != nil:
			a.Error = "起草" + r.err.Error() + "；写出的部分保留了"
		default:
			a.notice = fmt.Sprintf("起草完成，写了 %d 字；撤销可以整段撤回，起草前的版本在快照里", workspace.Count(r.text))
		}
	case workspace.AIRewrite:
		switch {
		case stopped:
			w.ed.SetReadOnly(false)
		case r.err != nil || strings.TrimSpace(r.text) == "":
			w.ed.SetReadOnly(false)
			if r.err == nil {
				r.err = errors.New("模型没有给出改写")
			}
			a.Error = "改写失败：" + r.err.Error()
		default:
			w.ed.Propose(r.a, r.b, r.text)
			w.proposed = &r.job
		}
	}
}

// stopAI stops the request in flight and what it shows, keeping what a
// draft wrote, as leaving the chapter does.
func (a *App) stopAI() {
	w := &a.write
	if r := w.run; r != nil {
		r.cancel()
		a.pollAI()
		if w.run != nil {
			r.err = context.Canceled
			a.finishAI(r)
		}
	}
	if w.proposed != nil {
		a.dropRewrite()
	}
	w.rewriting = false
}

// continueText asks for a sentence after the caret.
func (a *App) continueText() {
	w := &a.write
	if w.pen.Load() == nil {
		a.warn(w.penErr)
		return
	}
	w.ed.RequestPrediction()
	w.ed.Focus()
}

// openRewrite asks how to rewrite the words selected.
func (a *App) openRewrite() {
	w := &a.write
	if s, e := w.ed.Selection(); s == e || w.run != nil || w.proposed != nil {
		return
	}
	w.rewriting, w.finding = true, false
}

func (a *App) startRewrite(c *ui.Context) {
	w := &a.write
	s, e := w.ed.Selection()
	if s == e {
		return
	}
	a.warn(a.saveText())
	pw, err := a.Pen(a.Store)
	if err != nil {
		a.warn(err)
		return
	}
	r := []rune(w.ed.Doc.String())
	job, err := pw.Rewrite(writing.Scene{Chapter: w.node, Before: string(r[:s]), After: string(r[e:])}, string(r[s:e]), w.instruction)
	if err != nil {
		a.warn(err)
		return
	}
	w.ed.SetReadOnly(true)
	a.start(c, pw, &aiRun{op: workspace.AIRewrite, node: w.node, job: job, a: s, b: e})
}

// acceptRewrite keeps the text replaced as a snapshot with what the AI
// did, then puts the rewrite in.
func (a *App) acceptRewrite() {
	w := &a.write
	if w.proposed == nil {
		return
	}
	rec := w.proposed.Record
	if err := a.Store.SnapshotForAI(w.node, w.ed.Doc.String(), storedMarks(w.ed.Doc.Marks()), rec); err != nil {
		a.warn(err)
		return
	}
	w.ed.SetReadOnly(false)
	w.ed.AcceptProposal(rec.Model)
	w.proposed, w.rewriting = nil, false
	a.warn(a.saveText())
	a.notice = "已采纳改写；撤销可以换回原文，原来的版本也在快照里"
}

func (a *App) dropRewrite() {
	w := &a.write
	w.ed.DropProposal()
	w.ed.SetReadOnly(false)
	w.proposed = nil
}

func (a *App) openDraft() {
	w := &a.write
	if w.run != nil || w.proposed != nil {
		return
	}
	a.flush()
	w.draftOpen, w.words = true, ""
	w.draftMode = draftAppend
	missing, have, err := writing.Unsummarized(a.Store, w.node)
	a.warn(err)
	w.unsummarized, w.recapped = missing, have
}

const (
	draftAppend  = "接在已写的后面"
	draftReplace = "覆盖已写的内容"
)

// startDraft keeps the chapter as a snapshot with what the AI is to do,
// then has it write.
func (a *App) startDraft(c *ui.Context) {
	w := &a.write
	words := 0
	if s := strings.TrimSpace(w.words); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			a.Error = "目标字数需要是正整数，或者留空"
			return
		}
		words = n
	}
	a.flush()
	pw, err := a.Pen(a.Store)
	if err != nil {
		a.warn(err)
		return
	}
	text := w.ed.Doc.String()
	appending := w.draftMode == draftAppend && strings.TrimSpace(text) != ""
	scene := writing.Scene{Chapter: w.node}
	if appending {
		scene.Before = text
	}
	job, err := pw.Draft(scene, words)
	if err == nil {
		err = a.Store.SnapshotForAI(w.node, text, storedMarks(w.ed.Doc.Marks()), job.Record)
	}
	if err != nil {
		a.warn(err)
		return
	}
	r := &aiRun{op: workspace.AIDraft, node: w.node, job: job}
	n := w.ed.Doc.Len()
	if appending {
		w.ed.BeginDraft(n, n)
		if !strings.HasSuffix(text, "\n") {
			r.lead = "\n"
		}
	} else {
		w.ed.BeginDraft(0, n)
	}
	w.draftOpen, w.finding = false, false
	a.start(c, pw, r)
}

// aiBar shows what the AI is doing in the chapter, and asks what the
// author makes of a rewrite.
func (a *App) aiBar(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	bar := func(fn func()) {
		ui.Row(c).FillWidth().Padding(8, 16, 4, 24).Gap(8).AlignItems(ui.Center).Shrink(0).Children(fn)
	}
	switch {
	case w.run != nil && w.run.op == workspace.AIDraft:
		bar(func() {
			glyph(c, icFeather, 15, a.pal.ink)
			ui.Textf(c, "AI 正在按章纲起草 · 已写 %d 字 · %s", w.run.wrote, a.modelLabel(w.run.job.Record)).FontSize(13).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			if action(c, "停止", t.Danger, false).Clicked() {
				w.run.cancel()
			}
		})
	case w.run != nil:
		bar(func() {
			glyph(c, icWand, 15, a.pal.ink)
			ui.Textf(c, "AI 正在改写选中的部分 · %s", a.modelLabel(w.run.job.Record)).FontSize(13).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			if ghost(c, "取消", ui.Color{}).Clicked() {
				w.run.cancel()
			}
		})
	case w.proposed != nil:
		bar(func() {
			glyph(c, icWand, 15, t.Success)
			ui.Text(c, "红色划掉的是原文，绿色是改写。").FontSize(13).TextColor(t.TextMuted).Grow(1).Shrink(1)
			if ghost(c, "放弃", ui.Color{}).Clicked() {
				a.dropRewrite()
			}
			if ghost(c, "重新改写", ui.Color{}).Clicked() {
				a.dropRewrite()
				w.rewriting = true
			}
			if a.pal.primary(c, "采纳").Clicked() {
				a.acceptRewrite()
			}
		})
	case w.rewriting:
		bar(func() {
			glyph(c, icWand, 15, t.TextMuted)
			in := ui.TextInput(c, &w.instruction).Label("改写要求").Placeholder("怎么改？比如：节奏放慢，多写她的犹豫").Grow(1).Shrink(1).AutoFocus()
			s, e := w.ed.Selection()
			go_ := a.pal.primary(c, "改写").Disabled(s == e || strings.TrimSpace(w.instruction) == "")
			if (in.Submitted() || go_.Clicked()) && s != e && strings.TrimSpace(w.instruction) != "" {
				a.startRewrite(c)
			}
			if iconButton(c, icX, "关闭改写", ui.Color{}).Clicked() || in.Shortcut(0, ui.KeyEscape) {
				w.rewriting = false
			}
		})
	}
}

func (a *App) modelLabel(r workspace.AIRecord) string {
	return r.Model + " · " + model.Effort(r.Effort).Label()
}

func (a *App) draftView(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	ui.Modal(c, &w.draftOpen, func() {
		ui.Column(c).Width(520).Gap(14).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "按章纲起草").FontSize(17).FontWeight(600)
				ui.Text(c, "AI 只把章纲写成文字，不加章纲里没有的大事件。起草前存一份快照，写出的文字带“AI 起草”标记，你改过的地方会脱离标记。").FontSize(13).TextColor(t.TextMuted).LineHeight(1.6)
			})
			ui.Box(c).FillWidth().Padding(10, 12).Radius(8).Background(a.pal.side).Children(func() {
				ui.Text(c, strings.TrimSpace(w.outline)).FontSize(13).TextColor(t.TextMuted).LineHeight(1.7).FillWidth()
			})
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.End).Children(func() {
				labeled(c, "目标字数（可以不填）", func() {
					ui.TextInput(c, &w.words).Label("目标字数").Placeholder("由 AI 按章纲判断").FillWidth()
				}).Width(180)
				if strings.TrimSpace(w.ed.Doc.String()) != "" {
					labeled(c, "这一章已经有内容", func() {
						ui.Select(c, &w.draftMode, []string{draftAppend, draftReplace}).Label("已有内容").FillWidth()
					}).Grow(1)
				}
			})
			a.recapNote(c)
			if pw := w.pen.Load(); pw != nil {
				ui.Textf(c, "用 %s · 推理 %s；可以在“模型设置”里换。", pw.Model.Name, pw.Model.Effort.Label()).FontSize(12).TextColor(a.pal.faint)
			}
			ui.Row(c).FillWidth().Gap(10).Children(func() {
				ui.Spacer(c)
				if ghost(c, "取消", ui.Color{}).Clicked() {
					w.draftOpen = false
				}
				if a.pal.primary(c, "开始起草").Clicked() {
					a.startDraft(c)
				}
			})
		})
	})
}

// aiRecord shows what an AI did, with a snapshot taken before it.
func (a *App) aiRecord(c *ui.Context, r *workspace.AIRecord) {
	t := a.pal.theme
	op := map[string]string{workspace.AIDraft: "按章纲起草", workspace.AIRewrite: "改写"}[r.Op]
	ui.Column(c).FillWidth().Gap(4).Padding(10, 12).Radius(8).Border(1, t.Border).Children(func() {
		line := op + " · " + a.modelLabel(*r)
		if r.Instruction != "" {
			line += " · " + r.Instruction
		}
		ui.Text(c, line).FontSize(12.5).FontWeight(600).FillWidth()
		shown := append([]string{fmt.Sprintf("设定 %d 条", len(r.Setting))}, r.Shown...)
		ui.Text(c, "AI 看到了："+strings.Join(shown, "、")).FontSize(12).TextColor(t.TextMuted).FillWidth().LineHeight(1.5)
		if len(r.Omitted) > 0 {
			ui.Textf(c, "超出上下文预算没放进去的设定 %d 条", len(r.Omitted)).FontSize(12).TextColor(t.Warning)
		}
	})
}
