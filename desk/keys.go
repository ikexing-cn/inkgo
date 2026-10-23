package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
)

// keysHelp lists the commands of the help's groups, in the table's order;
// a row that joins commands, such as Ctrl+PgDn / PgUp, shows them
// together. The AI's are not listed while it is off.
func (a *App) keysHelp(c *ui.Context) {
	t := a.pal.Theme
	a.pal.Modal(c, &a.keysOpen, func() {
		ui.Column(c).Width(kit.DialogKeys).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.pal.DialogHead(c, "快捷键", "")
			ui.Scroll(c.Key("keys")).FillWidth().MaxHeight(440).Shrink(1).Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					group := ""
					for _, row := range commandHelp {
						if row.ai && !a.aiOn() {
							continue
						}
						if row.group != group {
							group = row.group
							ui.Text(c, group).FontSize(kit.FsSmall).TextColor(a.pal.Faint).Padding(14, 0, 4, 0)
						}
						ui.Row(c).FillWidth().Gap(12).Padding(6, 0).Children(func() {
							ui.Text(c, row.keys).FontSize(kit.FsControl).TextColor(t.TextMuted).Width(250).Shrink(0)
							ui.Text(c, row.does).FontSize(kit.FsControl).Grow(1).Shrink(1)
						})
					}
				})
			})
			ui.Row(c).FillWidth().Children(func() {
				ui.Spacer(c)
				if a.pal.Secondary(c, "关闭").Clicked() {
					a.keysOpen = false
				}
			})
		})
	})
}
