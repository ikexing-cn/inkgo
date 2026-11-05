package check

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"

	"writer/ext/check/checktext"
)

// footCheck is where the check of the chapter stands, a way to it, and
// when chapters are checked.
func (a *Panel) footCheck(c *ui.Context) {
	t := a.pal().Theme
	w := a.writing()
	k := &a.State
	line, found, can, tip := a.checkState()
	short := checkWhens[0].label
	for _, o := range checkWhens {
		if o.When == k.When {
			short = o.label
		}
	}
	if found > 0 {
		b := a.pal().Ghost(c, line, ui.Color{}).Padding(0).Label("查看检查结果").Tooltip(checktext.StatusGoCheck()).Shrink(1).MinWidth(0)
		if b.Clicked() {
			for _, it := range a.checkItems(false) {
				if it.Here {
					a.openCheck(it.F.ID)
					break
				}
			}
		}
	} else {
		said := ui.Text(c, line).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		if tip != "" {
			said.Tooltip(tip)
		}
	}
	if can {
		b := ui.ButtonBase(c).Label(checktext.StatusCheckNow()).Tooltip(checktext.StatusCheckTip()).Shrink(0)
		color := t.TextMuted
		if b.Hovered() {
			color = t.Text
		}
		b.Children(func() {
			ui.Text(c, checktext.StatusCheckNow()).FontSize(kit.FsNote).TextColor(color).Underline().SingleLine()
		})
		if b.Clicked() {
			if err := a.editingHost.SaveText(); err != nil {
				a.warn(err)
			} else {
				a.State.Queue.Enqueue(w.Node)
			}
		}
	}
	ui.Text(c, "·").FontSize(kit.FsNote).TextColor(a.pal().Faint).Shrink(0)
	b := ui.ButtonBase(c).Label(checktext.StatusWhenTitle()).Shrink(0)
	color := t.TextMuted
	if b.Hovered() {
		color = t.Text
	}
	b.Children(func() { ui.Text(c, short()+" ▴").FontSize(kit.FsNote).TextColor(color).SingleLine() })
	if b.Clicked() {
		k.WhenOpen = !k.WhenOpen
	}
	kit.Popover(c, b, &k.WhenOpen, 0, 1, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorTopLeft, ui.AnchorBottomLeft).Margin(0, 0, 4, 0).Width(270).Radius(kit.RCard).Padding(10, 6, 6, 6).Gap(4).
			Background(a.pal().Raised).Border(1, t.Border)
		a.pal().Pop.On(panel)
		ui.Text(c, checktext.StatusWhenTitle()).FontSize(kit.FsBody).FontWeight(600).Padding(0, 8, 4, 8)
		ui.Column(c).FillWidth().Gap(2).Children(func() { a.whenRows(c) })
	})
}
