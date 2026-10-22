package desk

import "github.com/egoist/mygo/ui"

// keysHelp lists the commands of the help's groups, in the table's order;
// a row that joins commands, such as Ctrl+PgDn / PgUp, shows them
// together. The AI's are not listed while it is off.
func (a *App) keysHelp(c *ui.Context) {
	t := a.pal.theme
	a.modal(c, &a.keysOpen, func() {
		ui.Column(c).Width(dialogKeys).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "快捷键", "")
			ui.Scroll(c.Key("keys")).FillWidth().MaxHeight(440).Shrink(1).Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					group := ""
					for _, row := range commandHelp {
						if row.ai && !a.aiOn() {
							continue
						}
						if row.group != group {
							group = row.group
							ui.Text(c, group).FontSize(fsSmall).TextColor(a.pal.faint).Padding(14, 0, 4, 0)
						}
						ui.Row(c).FillWidth().Gap(12).Padding(6, 0).Children(func() {
							ui.Text(c, row.keys).FontSize(fsControl).TextColor(t.TextMuted).Width(250).Shrink(0)
							ui.Text(c, row.does).FontSize(fsControl).Grow(1).Shrink(1)
						})
					}
				})
			})
			ui.Row(c).FillWidth().Children(func() {
				ui.Spacer(c)
				if a.pal.secondary(c, "关闭").Clicked() {
					a.keysOpen = false
				}
			})
		})
	})
}
