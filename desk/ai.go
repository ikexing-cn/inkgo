package desk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo/ui"
	"writer/desk/aiport"
	"writer/prefs"
	"writer/tokens"
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
var errNoPen = errors.New("还没有续写和检查模型")

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
	if s, e := w.ed.Selection(); s == e {
		return
	}
	a.closeFind()
	w.rewriting, w.instruction = true, ""
}

// rewrite opens a talk to rewrite the words selected as the author asked:
// its first words quote them. The AI proposes the change in the prose.
func (a *App) rewrite(c *ui.Context) {
	w := &a.write
	s, e := w.ed.Selection()
	wish, quote := strings.TrimSpace(w.instruction), strings.TrimSpace(w.ed.Doc.Slice(s, e))
	if wish == "" || quote == "" {
		return
	}
	q := workspace.Quote{Chapter: w.node, Title: a.shownTitle(), Text: quote}
	if a.startTalk(c, workspace.Event{Origin: workspace.AIRewrite, Text: wish, Quotes: []workspace.Quote{q}}) {
		w.rewriting, w.instruction = false, ""
		w.ed.MoveTo(e, false) // the words went to the talk: the caret after them
	}
}

func (a *App) openDraft() {
	w := &a.write
	a.flush()
	w.draftOpen, w.words, w.draftMode = true, "", draftAppend
	w.draftCards = 0
	if p, err := a.Store.Plan(); err == nil {
		for _, card := range p.Cards {
			if card.Chapter == w.node {
				w.draftCards++
			}
		}
	}
	w.draftNote = a.draftNote()
}

const (
	draftAppend  = "接在已有内容之后"
	draftReplace = "覆盖已有内容"
)

// draftNote says what a talk opened to draft the chapter shown carries
// before the AI looks anything up, and about how much.
func (a *App) draftNote() string {
	w := &a.write
	n, ok := w.tree.Node(w.node)
	if !ok {
		return ""
	}
	k, err := a.ai.Preloaded(a.Store, workspace.AIDraft, w.node)
	if err != nil {
		return ""
	}
	what := "各卷卷纲"
	if p := n.Parent; p != nil && p.Parent != nil {
		what = "本卷各章章纲"
	}
	var last *workspace.Node // the chapter before it
	done := false
	w.tree.Walk(func(m *workspace.Node, _ int) {
		switch {
		case done:
		case m == n:
			done = true
		case !m.Folder:
			last = m
		}
	})
	if last != nil && w.counts[last.ID] > 0 {
		what = "上一章全文、" + what
	}
	if w.draftCards > 0 {
		what = fmt.Sprintf("本章的 %d 张卡片、", w.draftCards) + what
	}
	return fmt.Sprintf("将带上%s等，约 %s token；AI 需要时会自己再查。", what, tokens.Round(k))
}

// draft opens a talk to draft the chapter shown from its outline, saying
// for the author what they asked in the dialog. The AI proposes the draft
// in the prose.
func (a *App) draft(c *ui.Context) {
	w := &a.write
	words, ok := prefs.CountOrNone(w.words)
	if !ok {
		return
	}
	text := "按章纲起草本章"
	if words > 0 {
		text += fmt.Sprintf("，约 %d 字", words)
	}
	if strings.TrimSpace(w.ed.Doc.String()) != "" {
		text += "，" + w.draftMode
	}
	if a.startTalk(c, workspace.Event{Origin: workspace.AIDraft, Text: text}) {
		w.draftOpen = false
		a.closeFind()
	}
}

// startTalk opens a new talk that keeps to the chapter shown, to draft or
// rewrite it, with the author's first words; it shows beside the prose.
// It reports whether the talk began: there must be a model to talk with,
// and no other reply in flight.
func (a *App) startTalk(c *ui.Context, words workspace.Event) bool {
	w := &a.write
	if work := a.Store.Work(); work.Discussion.Provider == "" || work.Discussion.Model == "" {
		a.SetError(&aiport.SetupError{Err: errors.New("请先配置讨论模型")})
		return false
	}
	what := map[string]string{workspace.AIDraft: "起草", workspace.AIRewrite: "改写"}[words.Origin]
	if a.busy != "" {
		a.hint = "AI 正在回复别的讨论；等它回完或取消后再" + what
		return false
	}
	a.flush() // the talk reads the chapter from the disk
	id, err := a.blankSession()
	if err != nil {
		a.warn(err)
		return false
	}
	a.switchSession(id)
	words.Chapter, words.Role = w.node, workspace.RoleUser
	a.titles[id], a.blank[id] = workspace.TalkTitle([]workspace.Event{words}, a.nodeTitle), false
	d := &a.doc
	d.hidden, d.tab, d.talks = false, paneTalk, false
	w.focus = false
	a.savePrefs()
	a.say(c, id, words, nil)
	return true
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
			head := "AI 仅将章纲扩写为正文，不添加章纲之外的重大情节。"
			if w.draftCards > 0 {
				head = "AI 仅将章纲和本章的卡片扩写为正文，不添加这些之外的重大情节。"
			}
			a.pal.dialogHead(c, "起草本章", head+"起草在一段新的讨论里进行：写好的先在正文里显示为提议，采纳后才写入，写入前保存快照；想改哪里，在讨论里接着说。")
			ui.Box(c).FillWidth().Padding(10, 12).Radius(rControl).Background(a.pal.side).Children(func() {
				ui.Text(c, strings.TrimSpace(w.outline)).FontSize(fsControl).TextColor(t.TextMuted).LineHeight(lhLoose).FillWidth()
			})
			_, fits := prefs.CountOrNone(w.words)
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
			if w.draftNote != "" {
				ui.Text(c, w.draftNote).FontSize(fsNote).TextColor(t.TextMuted).FillWidth()
			}
			if work := a.Store.Work(); work.Discussion.Model != "" {
				ui.Textf(c, "用讨论模型 %s · %s；可在设置中「AI」配置里更换。", work.Discussion.Model, a.ai.EffortLabel(work.Discussion.Effort)).FontSize(fsNote).TextColor(t.TextMuted).FillWidth()
			}
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "取消").Clicked() {
					w.draftOpen = false
				}
				if a.pal.primary(c, "开始起草").Disabled(!fits).Clicked() {
					a.draft(c)
				}
			})
		})
	})
}

// aiRecord shows what an AI did, with a snapshot taken before it, as
// snapshots kept it while a draft or a rewrite asked the model once.
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

// talkRecord shows, with a snapshot taken before what an AI proposed in
// talks went in, which talks they were.
func (a *App) talkRecord(c *ui.Context, sessions []string) {
	t := a.pal.theme
	var b strings.Builder
	for _, s := range sessions {
		title, ok := a.titles[s]
		if !ok {
			title = "已删除的讨论"
		}
		b.WriteString("「" + title + "」")
	}
	ui.Column(c).FillWidth().Padding(10, 12).Radius(rControl).Border(1, t.Border).Children(func() {
		ui.Text(c, "采纳讨论"+b.String()+"里的提议").FontSize(fsNote).FontWeight(600).FillWidth()
	})
}
