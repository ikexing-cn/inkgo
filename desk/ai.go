package desk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/desk/kit"
	"writer/locales"
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
type noPenError struct{}

func (noPenError) Error() string { return locales.SettingsNoPen() }

var errNoPen error = noPenError{}

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
func (a *App) modelsChanged() { a.write.penKey = nil }

type penKey struct {
	writing, discussion workspace.Selection
	providers           int
}

// keepPen rebuilds the writer the editors predict with when the model
// settings change; with the AI off, there is none.
func (a *App) keepPen() {
	w := &a.write
	if !a.aiOn() {
		w.pen.Store(nil)
		w.penKey, w.penErr = nil, nil
		return
	}
	work := a.Store.Work()
	key := penKey{writing: work.Writing, discussion: work.Discussion, providers: len(a.ai.Providers())}
	if w.penKey != nil && key == *w.penKey {
		return
	}
	w.penKey = &key
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
	w.cur().ed.RequestPrediction()
	w.cur().ed.Focus()
}

// openRewrite asks how to rewrite the words selected.
func (a *App) openRewrite() {
	w := &a.write
	if s, e := w.cur().ed.Selection(); s == e {
		return
	}
	a.closeFind()
	w.rewriting, w.instruction = true, ""
}

// rewrite opens a talk to rewrite the words selected as the author asked:
// its first words quote them. The AI proposes the change in the prose.
func (a *App) rewrite(c *ui.Context) {
	w := &a.write
	s, e := w.cur().ed.Selection()
	wish, quote := strings.TrimSpace(w.instruction), strings.TrimSpace(w.cur().ed.Doc.Slice(s, e))
	if wish == "" || quote == "" {
		return
	}
	q := workspace.Quote{Chapter: w.node, Title: a.shownTitle(), Text: quote}
	if a.startTalk(c, workspace.Event{Origin: workspace.AIRewrite, Text: wish, Quotes: []workspace.Quote{q}}) {
		w.rewriting, w.instruction = false, ""
		w.cur().ed.MoveTo(e, false) // the words went to the talk: the caret after them
	}
}

func (a *App) openDraft() {
	w := &a.write
	a.flush()
	w.draftOpen, w.words, w.draftMode = true, "", draftAppend
	w.draftCards = 0
	if err := a.book.ReloadPlan(); err == nil {
		for _, card := range a.book.Plan().Cards {
			if card.Chapter == w.node {
				w.draftCards++
			}
		}
	}
	w.draftNote = a.draftNote()
}

const (
	draftAppend  = "append"
	draftReplace = "replace"
)

// draftNote says what a talk opened to draft the chapter shown carries
// before the AI looks anything up, and about how much.
func (a *App) draftNote() string {
	w := &a.write
	p, err := a.ai.Preloaded(a.Store, workspace.AIDraft, w.node)
	if err != nil {
		return ""
	}
	what := locales.DraftItemBooks()
	if p.Outlines == aiport.VolumeChapters {
		what = locales.DraftItemVolume()
	}
	if p.Previous {
		what = locales.DraftItemPrev() + locales.DraftListSep() + what
	}
	if p.Cards > 0 {
		what = locales.DraftItemCards(p.Cards) + locales.DraftListSep() + what
	}
	return locales.DraftSends(what, tokens.Round(p.Tokens))
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
	if strings.TrimSpace(w.cur().ed.Doc.String()) != "" {
		if w.draftMode == draftReplace {
			text += "，覆盖已有内容"
		} else {
			text += "，接在已有内容之后"
		}
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
		a.SetError(&aiport.SetupError{Err: errors.New(locales.ChatNeedModel())})
		return false
	}
	what := map[string]string{workspace.AIDraft: locales.SettingsFeatureDraft(), workspace.AIRewrite: locales.SettingsFeatureRewrite()}[words.Origin]
	if a.busy != "" {
		a.status.Hint(locales.DraftBusyOther(what))
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
	a.titles[id], a.blank[id] = a.talkTitle([]workspace.Event{words}), false
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
	if ref, ok := workspace.ParseModelRef(name); ok {
		for _, p := range a.ai.Providers() {
			if p.ID == ref.Provider && p.Name != "" {
				ref.Provider = p.Name
				return ref.String()
			}
		}
	}
	return name
}

func (a *App) draftView(c *ui.Context) {
	t := a.pal.Theme
	w := &a.write
	a.pal.Modal(c, &w.draftOpen, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			head := locales.DraftAboutOutline()
			if w.draftCards > 0 {
				head = locales.DraftAboutCards()
			}
			a.pal.DialogHead(c, locales.CmdDraft(), locales.DraftAbout(head, locales.DraftAboutFlow()))
			ui.Box(c).FillWidth().Padding(10, 12).Radius(kit.RControl).Background(a.pal.Side).Children(func() {
				ui.Text(c, strings.TrimSpace(w.cur().outline.Text)).FontSize(kit.FsControl).TextColor(t.TextMuted).LineHeight(kit.LhLoose).FillWidth()
			})
			_, fits := prefs.CountOrNone(w.words)
			kit.Labeled(c, locales.DraftGoal(), func() {
				ui.TextInput(c, &w.words).Label(locales.DraftGoalLabel()).Placeholder(locales.DraftGoalHint()).FillWidth()
				if !fits {
					ui.Text(c, locales.DraftGoalInvalid()).FontSize(kit.FsNote).TextColor(t.Danger)
				}
			})
			if strings.TrimSpace(w.cur().ed.Doc.String()) != "" {
				kit.Labeled(c, locales.DraftExisting(), func() {
					modes := []string{draftAppend, draftReplace}
					kit.Dots(c, &w.draftMode, locales.DraftExistingHow(), modes, []string{locales.DraftAppend(), locales.DraftOverwrite()})
				})
			}
			if w.draftNote != "" {
				ui.Text(c, w.draftNote).FontSize(kit.FsNote).TextColor(t.TextMuted).FillWidth()
			}
			if work := a.Store.Work(); work.Discussion.Model != "" {
				ui.Text(c, locales.DraftUsing(work.Discussion.Model, a.ai.EffortLabel(work.Discussion.Effort))).FontSize(kit.FsNote).TextColor(t.TextMuted).FillWidth()
			}
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if a.pal.Secondary(c, locales.CommonCancel()).Clicked() {
					w.draftOpen = false
				}
				if a.pal.Primary(c, locales.DraftStart()).Disabled(!fits).Clicked() {
					a.draft(c)
				}
			})
		})
	})
}

// aiRecord shows what an AI did, with a snapshot taken before it, as
// snapshots kept it while a draft or a rewrite asked the model once.
func (a *App) aiRecord(c *ui.Context, r *workspace.AIRecord) {
	t := a.pal.Theme
	op := map[string]string{workspace.AIDraft: locales.DraftFromOutline(), workspace.AIRewrite: locales.SettingsFeatureRewrite()}[r.Op]
	ui.Column(c).FillWidth().Gap(4).Padding(10, 12).Radius(kit.RControl).Border(1, t.Border).Children(func() {
		line := op + " · " + a.modelLabel(*r)
		if r.Instruction != "" {
			line += " · " + r.Instruction
		}
		ui.Text(c, line).FontSize(kit.FsNote).FontWeight(600).FillWidth()
		shown := append([]string{locales.RecordsLoreCount(len(r.Setting))}, r.Shown...)
		ui.Text(c, locales.DraftReferenced(strings.Join(shown, locales.DraftListSep()))).FontSize(kit.FsNote).TextColor(t.TextMuted).FillWidth().LineHeight(kit.LhBody)
		if len(r.Omitted) > 0 {
			ui.Text(c, locales.DraftOmittedLore(len(r.Omitted))).FontSize(kit.FsNote).TextColor(t.Warning)
		}
	})
}

// talkRecord shows, with a snapshot taken before what an AI proposed in
// talks went in, which talks they were.
func (a *App) talkRecord(c *ui.Context, sessions []string) {
	t := a.pal.Theme
	var b strings.Builder
	for _, s := range sessions {
		title, ok := a.talkName(s)
		if !ok {
			title = locales.SnapAITalkGone()
		}
		b.WriteString(locales.SnapAITalk(title))
	}
	ui.Column(c).FillWidth().Padding(10, 12).Radius(kit.RControl).Border(1, t.Border).Children(func() {
		ui.Text(c, locales.SnapAIFrom(b.String())).FontSize(kit.FsNote).FontWeight(600).FillWidth()
	})
}
