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

// modelsChanged tells that a service's address or key may be another: the
// writer the editors predict with holds the ones it was made with, and is
// made anew.
func (a *App) modelsChanged() { a.write.penKey = "" }

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
	sv := c.Services()
	go func() {
		text, err := pw.Run(ctx, r.job, func(s string) {
			if r.op != workspace.AIDraft {
				return
			}
			r.mu.Lock()
			r.chunks = append(r.chunks, s)
			r.mu.Unlock()
			sv.Invalidate()
		})
		r.mu.Lock()
		r.done, r.text, r.err = true, text, err
		if ctx.Err() != nil {
			r.err = context.Canceled
		}
		r.mu.Unlock()
		cancel()
		sv.Invalidate()
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
			a.notice = "已停止起草，已写部分保留；可撤销整段撤回"
		case r.err != nil:
			a.Error = "起草" + r.err.Error() + "；已写部分保留"
		default:
			a.notice = fmt.Sprintf("起草完成，共 %d 字；可撤销整段撤回，起草前的版本已存入快照", workspace.Count(r.text))
		}
	case workspace.AIRewrite:
		switch {
		case stopped:
			w.ed.SetReadOnly(false)
		case r.err != nil || strings.TrimSpace(r.text) == "":
			w.ed.SetReadOnly(false)
			if r.err == nil {
				r.err = errors.New("模型未返回改写结果")
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
	a.notice = "已采纳改写；可撤销恢复原文，原版本已存入快照"
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
	w.draftMode, w.draftAt = draftAppend, 0
	missing, have, err := writing.Unsummarized(a.Store, w.node)
	a.warn(err)
	w.unsummarized, w.recapped = missing, have
}

const (
	draftAppend  = "接在已有内容之后"
	draftReplace = "覆盖已有内容"
)

// startDraft keeps the chapter as a snapshot with what the AI is to do,
// then has it write.
func (a *App) startDraft(c *ui.Context) {
	w := &a.write
	words := 0
	if s := strings.TrimSpace(w.words); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			a.Error = "目标字数须为正整数，或留空"
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
			glyph(c, icFeather, iconMd, a.pal.ink)
			ui.Textf(c, "AI 正在按章纲起草 · 已写 %d 字 · %s", w.run.wrote, a.modelLabel(w.run.job.Record)).FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			if action(c, "停止", t.Danger, false).Clicked() {
				w.run.cancel()
			}
		})
	case w.run != nil:
		bar(func() {
			glyph(c, icWand, iconMd, a.pal.ink)
			ui.Textf(c, "AI 正在改写选中内容 · %s", a.modelLabel(w.run.job.Record)).FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			if a.pal.secondary(c, "取消").Clicked() {
				w.run.cancel()
			}
		})
	case w.proposed != nil:
		bar(func() {
			glyph(c, icWand, iconMd, t.Success)
			ui.Text(c, "红色删除线为原文，绿色为改写。").FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1)
			if a.pal.secondary(c, "放弃").Clicked() {
				a.dropRewrite()
			}
			if a.pal.ghost(c, "重新改写", ui.Color{}).Clicked() {
				a.dropRewrite()
				w.rewriting = true
			}
			if a.pal.primary(c, "采纳").Clicked() {
				a.acceptRewrite()
			}
		})
	case w.rewriting:
		bar(func() {
			glyph(c, icWand, iconMd, t.TextMuted)
			in := ui.TextInput(c, &w.instruction).Label("改写要求").Placeholder("改写要求，例如：放慢节奏，多写她的犹豫").Grow(1).Shrink(1).AutoFocus()
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
	a.modal(c, &w.draftOpen, func() {
		ui.Column(c).Width(dialogSmall).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "起草本章", "AI 仅将章纲扩写为正文，不添加章纲之外的重大情节。起草前会保存快照；生成的文字带有“AI 起草”标记，修改过的部分不再带标记。")
			ui.Box(c).FillWidth().Padding(10, 12).Radius(rControl).Background(a.pal.side).Children(func() {
				ui.Text(c, strings.TrimSpace(w.outline)).FontSize(fsControl).TextColor(t.TextMuted).LineHeight(lhLoose).FillWidth()
			})
			labeled(c, "目标字数（选填）", func() {
				ui.TextInput(c, &w.words).Label("目标字数").Placeholder("由 AI 按章纲判断").FillWidth()
			})
			if strings.TrimSpace(w.ed.Doc.String()) != "" {
				labeled(c, "本章已有内容", func() {
					modes := []string{draftAppend, draftReplace}
					w.draftMode = modes[min(w.draftAt, len(modes)-1)]
					a.pal.segments(c, &w.draftAt, "已有内容的处理方式", modes).FillWidth()
				})
			}
			a.recapNote(c)
			if pw := w.pen.Load(); pw != nil {
				ui.Textf(c, "用 %s · 推理 %s；可在“模型设置”中更换。", pw.Model.Name, pw.Model.Effort.Label()).FontSize(fsNote).TextColor(t.TextMuted)
			}
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "取消").Clicked() {
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
	ui.Column(c).FillWidth().Gap(4).Padding(10, 12).Radius(rControl).Border(1, t.Border).Children(func() {
		line := op + " · " + a.modelLabel(*r)
		if r.Instruction != "" {
			line += " · " + r.Instruction
		}
		ui.Text(c, line).FontSize(fsNote).FontWeight(600).FillWidth()
		shown := append([]string{fmt.Sprintf("设定 %d 条", len(r.Setting))}, r.Shown...)
		ui.Text(c, "AI 参考了："+strings.Join(shown, "、")).FontSize(fsNote).TextColor(t.TextMuted).FillWidth().LineHeight(lhBody)
		if len(r.Omitted) > 0 {
			ui.Textf(c, "超出上下文预算而未放入的设定 %d 条", len(r.Omitted)).FontSize(fsNote).TextColor(t.Warning)
		}
	})
}
