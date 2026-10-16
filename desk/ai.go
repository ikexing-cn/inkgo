package desk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/aiport"
	"writer/workspace"
)

// pen continues the text of a chapter's editor with the writer the page
// keeps up to date; it runs off the UI thread.
type pen struct {
	writer  *atomic.Pointer[aiport.Pen]
	chapter string
}

// errNoPen is what predicting says without a writer; the editor shows
// nothing of it, and the page says why there is none.
var errNoPen = errors.New("还没有写作模型")

func (p pen) Predict(ctx context.Context, before string) (string, error) {
	w := p.writer.Load()
	if w == nil {
		return "", errNoPen
	}
	return (*w).Predict(ctx, aiport.Scene{Chapter: p.chapter, Before: before})
}

// modelsChanged tells that a service's address or key may be another: the
// writer the editors predict with holds the ones it was made with, and is
// made anew.
func (a *App) modelsChanged() { a.write.penKey = "" }

// keepPen rebuilds the writer the editors predict with when the model
// settings change; with the AI off, there is none.
func (a *App) keepPen() {
	w := &a.write
	if !a.aiOn() {
		w.pen.Store(nil)
		w.penKey, w.penErr = "", nil
		return
	}
	work := a.Store.Work()
	key := fmt.Sprint(work.Writing, work.Discussion, len(a.ai.Providers()))
	if key == w.penKey {
		return
	}
	w.penKey = key
	pw, err := a.ai.Pen(a.Store)
	if err != nil {
		w.pen.Store(nil)
	} else {
		w.pen.Store(&pw)
	}
	w.penErr = err
}

// aiRun is a request of the writing model, in flight, and then what it
// proposed until the author takes it or not: what it streams waits here
// for the UI thread.
type aiRun struct {
	op      string
	node    string
	job     aiport.Job
	cancel  context.CancelFunc
	a, b    int    // the words a rewrite or a draft replaces
	lead    string // what goes before a draft's first words
	mu      sync.Mutex
	chunks  []string
	done    bool
	text    string
	err     error
	wrote   int       // the characters a draft wrote so far
	stopped bool      // a draft ended before it was done
	shown   time.Time // when what it proposed came, with the buttons to take it
}

// settleFor is how long the buttons of what was proposed take no clicks: one
// meant for 停止 or 取消 as the request ended would land on 采纳.
const settleFor = 500 * time.Millisecond

// settled tells whether r was proposed long enough ago to be clicked on.
func (r *aiRun) settled() bool { return time.Since(r.shown) >= settleFor }

// propose shows what r proposed until the author takes it or not.
func (a *App) propose(r *aiRun) {
	r.shown = time.Now()
	a.write.proposed = r
}

func (a *App) start(c *ui.Context, r *aiRun) {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	a.write.run = r
	sv := c.Services()
	go func() {
		text, err := r.job.Run(ctx, func(s string) {
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
			w.ed.ProposeMore(r.lead + s)
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
		// What it wrote waits for the author, though it stopped short; with
		// nothing written, it is as if there was no draft.
		switch {
		case r.wrote == 0:
			a.dropProposal()
			if !stopped {
				if r.err == nil {
					r.err = errors.New("模型未返回内容")
				}
				a.Error = "起草失败：" + r.err.Error()
			}
		case r.err != nil:
			r.stopped = true
			a.propose(r)
			if !stopped {
				a.Error = "起草" + r.err.Error() + "；已写的部分可以采纳"
			}
		default:
			a.propose(r)
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
			a.propose(r)
		}
	}
}

// stopAI stops the request in flight and drops what the AI proposed, a
// draft as a rewrite: not taken, it is not the author's.
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
		a.dropProposal()
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
	a.closeFind()
	w.rewriting = true
}

func (a *App) startRewrite(c *ui.Context) {
	w := &a.write
	s, e := w.ed.Selection()
	if s == e {
		return
	}
	a.warn(a.saveText())
	pw, err := a.ai.Pen(a.Store)
	if err != nil {
		a.warn(err)
		return
	}
	r := []rune(w.ed.Doc.String())
	job, err := pw.Rewrite(aiport.Scene{Chapter: w.node, Before: string(r[:s]), After: string(r[e:])}, string(r[s:e]), w.instruction)
	if err != nil {
		a.warn(err)
		return
	}
	w.ed.SetReadOnly(true)
	a.start(c, &aiRun{op: workspace.AIRewrite, node: w.node, job: job, a: s, b: e})
}

// acceptProposal keeps the chapter as a snapshot with what the AI did,
// then puts in what it proposed, a rewrite or a draft.
func (a *App) acceptProposal() {
	w := &a.write
	p := w.proposed
	if p == nil {
		return
	}
	rec := p.job.Record
	if err := a.Store.SnapshotForAI(w.node, w.ed.Doc.String(), storedMarks(w.ed.Doc.Marks()), rec); err != nil {
		a.warn(err)
		return
	}
	w.ed.SetReadOnly(false)
	w.ed.AcceptProposal(rec.Model)
	w.proposed, w.rewriting = nil, false
	a.warn(a.saveText())
	what := "改写"
	if p.op == workspace.AIDraft {
		what = "起草"
	}
	a.notice = "已采纳" + what + "；可撤销恢复原文，原版本已存入快照"
}

func (a *App) dropProposal() {
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
}

const (
	draftAppend  = "接在已有内容之后"
	draftReplace = "覆盖已有内容"
)

// startDraft has the AI write the chapter from its outline, shown beside
// the text there until the author takes it.
func (a *App) startDraft(c *ui.Context) {
	w := &a.write
	words, ok := countOrNone(w.words)
	if !ok {
		return
	}
	a.flush()
	pw, err := a.ai.Pen(a.Store)
	if err != nil {
		a.warn(err)
		return
	}
	text := w.ed.Doc.String()
	appending := w.draftMode == draftAppend && strings.TrimSpace(text) != ""
	scene := aiport.Scene{Chapter: w.node}
	if appending {
		scene.Before = text
	}
	job, err := pw.Draft(scene, words)
	if err != nil {
		a.warn(err)
		return
	}
	r := &aiRun{op: workspace.AIDraft, node: w.node, job: job, b: w.ed.Doc.Len()}
	if appending {
		r.a = r.b
		if !strings.HasSuffix(text, "\n") {
			r.lead = "\n"
		}
	}
	w.ed.ProposeDraft(r.a, r.b)
	w.ed.SetReadOnly(true)
	a.closeFind()
	w.draftOpen = false
	a.start(c, r)
}

// aiBar shows what the AI is doing in the chapter, and asks what the
// author makes of a rewrite or a draft, once it settled. Its buttons give
// the keyboard back to the prose as the bar changes under them: taken, an
// undo takes it back.
func (a *App) aiBar(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	// A card over the prose and as wide.
	bar := func(fn func()) {
		overProse(c, 10, 0, func() {
			ui.Row(c).FillWidth().Padding(6, 8, 6, 12).Gap(8).AlignItems(ui.Center).Radius(rCard).Border(1, t.Border).Background(a.pal.side).Children(fn)
		})
	}
	switch {
	case w.run != nil && w.run.op == workspace.AIDraft:
		bar(func() {
			glyph(c, icFeather, iconMd, a.pal.ink)
			ui.Textf(c, "AI 正在按章纲起草 · 已写 %d 字 · %s", w.run.wrote, a.modelLabel(w.run.job.Record)).FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			if action(c, "停止", t.Danger, false).Clicked() {
				w.run.cancel()
				w.ed.Focus()
			}
		})
	case w.run != nil:
		bar(func() {
			glyph(c, icWand, iconMd, a.pal.ink)
			ui.Textf(c, "AI 正在改写选中内容 · %s", a.modelLabel(w.run.job.Record)).FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			if a.pal.secondary(c, "取消").Clicked() {
				w.run.cancel()
				w.ed.Focus()
			}
		})
	case w.proposed != nil && w.proposed.op == workspace.AIDraft:
		p := w.proposed
		bar(func() {
			glyph(c, icFeather, iconMd, t.Success)
			say := fmt.Sprintf("起草 %d 字，绿色为起草", p.wrote)
			if p.stopped {
				say = "已停止，" + say
			}
			if p.b > p.a {
				say += "，红色删除线为将被覆盖的原文"
			}
			ui.Text(c, say+"。").FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
			if a.pal.secondary(c, "放弃").Clicked() && p.settled() {
				a.dropProposal()
				w.ed.Focus()
			}
			if a.pal.ghost(c, "重新起草", ui.Color{}).Clicked() && p.settled() {
				a.startDraft(c)
				w.ed.Focus()
			}
			if a.pal.primary(c, "采纳").Clicked() && p.settled() {
				a.acceptProposal()
				w.ed.Focus()
			}
		})
	case w.proposed != nil:
		p := w.proposed
		bar(func() {
			glyph(c, icWand, iconMd, t.Success)
			ui.Text(c, "红色删除线为原文，绿色为改写。").FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1)
			if a.pal.secondary(c, "放弃").Clicked() && p.settled() {
				a.dropProposal()
				w.rewriting = false
				w.ed.Focus()
			}
			if a.pal.ghost(c, "重新改写", ui.Color{}).Clicked() && p.settled() { // the same words, as asked before
				a.dropProposal()
				w.ed.Select(p.a, p.b)
				a.startRewrite(c)
				w.ed.Focus()
			}
			if a.pal.primary(c, "采纳").Clicked() && p.settled() {
				a.acceptProposal()
				w.ed.Focus()
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
				w.ed.Focus()
			}
			if iconButton(c, icX, "关闭改写", ui.Color{}).Clicked() || in.Shortcut(0, ui.KeyEscape) {
				w.rewriting = false
				w.ed.Focus()
			}
		})
	}
}

// modelLabel names the model of r and its effort: DeepSeek/deepseek-flash ·
// Default.
func (a *App) modelLabel(r workspace.AIRecord) string {
	return a.serviceModel(r.Model) + " · " + a.ai.EffortLabel(r.Effort)
}

// serviceModel names a model, written service ID/model, with the service by
// its name as the settings show it: DeepSeek/deepseek-flash.
func (a *App) serviceModel(name string) string {
	if id, m, ok := strings.Cut(name, "/"); ok {
		for _, p := range a.ai.Providers() {
			if p.ID == id && p.Name != "" {
				return p.Name + "/" + m
			}
		}
	}
	return name
}

func (a *App) draftView(c *ui.Context) {
	t := a.pal.theme
	w := &a.write
	a.modal(c, &w.draftOpen, func() {
		ui.Column(c).Width(dialogSmall).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "起草本章", "AI 仅将章纲扩写为正文，不添加章纲之外的重大情节。写完先供你查看，采纳后才写入正文，写入前保存快照；写入的文字带有“AI 起草”标记，修改过的部分不再带标记。")
			ui.Box(c).FillWidth().Padding(10, 12).Radius(rControl).Background(a.pal.side).Children(func() {
				ui.Text(c, strings.TrimSpace(w.outline)).FontSize(fsControl).TextColor(t.TextMuted).LineHeight(lhLoose).FillWidth()
			})
			_, fits := countOrNone(w.words)
			labeled(c, "目标字数（选填）", func() {
				ui.TextInput(c, &w.words).Label("目标字数").Placeholder("由 AI 按章纲判断").FillWidth()
				if !fits {
					ui.Text(c, "目标字数须为正整数，或留空").FontSize(fsNote).TextColor(t.Danger)
				}
			})
			if strings.TrimSpace(w.ed.Doc.String()) != "" {
				labeled(c, "本章已有内容", func() {
					modes := []string{draftAppend, draftReplace}
					dots(c, &w.draftMode, "已有内容的处理方式", modes, modes)
				})
			}
			if pw := w.pen.Load(); pw != nil {
				ui.Textf(c, "用 %s · %s；可在设置中「AI」配置里更换。", a.serviceModel((*pw).Name()), a.ai.EffortLabel((*pw).Effort())).FontSize(fsNote).TextColor(t.TextMuted)
			}
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "取消").Clicked() {
					w.draftOpen = false
				}
				if a.pal.primary(c, "开始起草").Disabled(!fits).Clicked() {
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
			ui.Textf(c, "超出上下文上限而未放入的设定 %d 条", len(r.Omitted)).FontSize(fsNote).TextColor(t.Warning)
		}
	})
}
