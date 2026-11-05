package draft

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"strings"
	"writer/desk/kit"
	"writer/ext/draft/drafttext"
	"writer/locales"
	"writer/prefs"
	"writer/workspace"
)

// OpenDraft opens the chapter drafting dialog on the frame thread.
func (a *View) OpenDraft() {
	if _, present := a.outlineValue(a.node()); !present {
		return
	}
	w := a
	a.editing.Get().Flush()
	w.Open, w.Words, w.Mode = true, "", draftAppend
	w.Cards = 0
	if err := a.book.Get().Content().ReloadPlan(); err == nil {
		for _, card := range a.book.Get().Content().Plan().Cards {
			if card.Chapter == a.node() {
				w.Cards++
			}
		}
	}
	w.Note = a.DraftNote()
}

const (
	draftAppend  = "append"
	draftReplace = "replace"
)

// Draft opens a talk to draft the chapter shown from its outline, saying
// for the author what they asked in the dialog. The AI proposes the draft
// in the prose.
func (a *View) Draft(c *ui.Context) {
	w := a
	words, ok := prefs.CountOrNone(w.Words)
	if !ok {
		return
	}
	text := "按章纲起草本章"
	if words > 0 {
		if a.book.Get().Store().Work().Language == "en" {
			text += fmt.Sprintf(", about %d words", words)
		} else {
			text += fmt.Sprintf("，约 %d 字", words)
		}
	}
	if strings.TrimSpace(a.ed().Doc.String()) != "" {
		if w.Mode == draftReplace {
			text += "，覆盖已有内容"
		} else {
			text += "，接在已有内容之后"
		}
	}
	if a.beginTalk(c, workspace.Event{Origin: workspace.AIDraft, Text: text}) {
		w.Open = false
		a.surface.Get().CloseSelectionTools()
	}
}

func (a *View) draftView(c *ui.Context) {
	t := a.palette.Get().Colors().Theme
	w := a
	a.palette.Get().Colors().Modal(c, &w.Open, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			head := drafttext.DraftAboutOutline()
			if w.Cards > 0 {
				head = drafttext.DraftAboutCards()
			}
			a.palette.Get().Colors().DialogHead(c, drafttext.CmdDraft(), drafttext.DraftAbout(head, drafttext.DraftAboutFlow()))
			ui.Box(c).FillWidth().Padding(10, 12).Radius(kit.RControl).Background(a.palette.Get().Colors().Side).Children(func() {
				ui.Text(c, strings.TrimSpace(a.chapterOutline(a.node()))).FontSize(kit.FsControl).TextColor(t.TextMuted).LineHeight(kit.LhLoose).FillWidth()
			})
			_, fits := prefs.CountOrNone(w.Words)
			kit.Labeled(c, drafttext.DraftGoal(a.countUnit()), func() {
				ui.TextInput(c, &w.Words).Label(drafttext.DraftGoalLabel(a.countUnit())).Placeholder(drafttext.DraftGoalHint()).FillWidth()
				if !fits {
					ui.Text(c, drafttext.DraftGoalInvalid(a.countUnit())).FontSize(kit.FsNote).TextColor(t.Danger)
				}
			})
			if strings.TrimSpace(a.ed().Doc.String()) != "" {
				kit.Labeled(c, drafttext.DraftExisting(), func() {
					modes := []string{draftAppend, draftReplace}
					kit.Dots(c, &w.Mode, drafttext.DraftExistingHow(), modes, []string{drafttext.DraftAppend(), drafttext.DraftOverwrite()})
				})
			}
			if w.Note != "" {
				ui.Text(c, w.Note).FontSize(kit.FsNote).TextColor(t.TextMuted).FillWidth()
			}
			if work := a.book.Get().Store().Work(); work.Discussion.Model != "" {
				ui.Text(c, drafttext.DraftUsing(work.Discussion.Model, a.ai.Get().EffortLabel(work.Discussion.Effort))).FontSize(kit.FsNote).TextColor(t.TextMuted).FillWidth()
			}
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if a.palette.Get().Colors().Secondary(c, locales.CommonCancel()).Clicked() {
					w.Open = false
				}
				if a.palette.Get().Colors().Primary(c, drafttext.DraftStart()).Disabled(!fits).Clicked() {
					a.Draft(c)
				}
			})
		})
	})
}
