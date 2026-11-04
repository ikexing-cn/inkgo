package snaps

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"writer/desk/kit"
	"writer/ext/snaps/snapstext"
	"writer/locales"
	"writer/workspace"
)

// aiRecord shows what an AI did, with a snapshot taken before it, as
// snapshots kept it while a draft or a rewrite asked the model once.
func (a *View) aiRecord(c *ui.Context, r *workspace.AIRecord) {
	t := a.Palette.Colors().Theme
	op := map[string]string{workspace.AIDraft: snapstext.DraftFromOutline(), workspace.AIRewrite: locales.SettingsFeatureRewrite()}[r.Op]
	ui.Column(c).FillWidth().Gap(4).Padding(10, 12).Radius(kit.RControl).Border(1, t.Border).Children(func() {
		line := op + " · " + a.Desktop.ModelLabel(*r)
		if r.Instruction != "" {
			line += " · " + r.Instruction
		}
		ui.Text(c, line).FontSize(kit.FsNote).FontWeight(600).FillWidth()
		shown := append([]string{snapstext.RecordsLoreCount(len(r.Setting))}, r.Shown...)
		ui.Text(c, snapstext.DraftReferenced(strings.Join(shown, locales.DraftListSep()))).FontSize(kit.FsNote).TextColor(t.TextMuted).FillWidth().LineHeight(kit.LhBody)
		if len(r.Omitted) > 0 {
			ui.Text(c, snapstext.DraftOmittedLore(len(r.Omitted))).FontSize(kit.FsNote).TextColor(t.Warning)
		}
	})
}

// talkRecord shows, with a snapshot taken before what an AI proposed in
// talks went in, which talks they were.
func (a *View) talkRecord(c *ui.Context, sessions []string) {
	t := a.Palette.Colors().Theme
	var b strings.Builder
	// With talk off its titles are unknown, so the line names no talk.
	if talk, on := a.Talk(); on {
		for _, s := range sessions {
			title, ok := talk.TalkTitle(s)
			if !ok {
				title = locales.SnapAITalkGone()
			}
			b.WriteString(snapstext.SnapAITalk(title))
		}
	}
	ui.Column(c).FillWidth().Padding(10, 12).Radius(kit.RControl).Border(1, t.Border).Children(func() {
		ui.Text(c, snapstext.SnapAIFrom(b.String())).FontSize(kit.FsNote).FontWeight(600).FillWidth()
	})
}
