package desk

import "github.com/egoist/mygo/ui"

// keyGroups are the shortcuts the help lists, by where they work.
var keyGroups = []struct {
	Where string
	Keys  [][2]string
}{
	{"正文", [][2]string{
		{modKey + "S", "立即保存（停笔片刻也会自动保存）"},
		{modKey + "F", "查找替换，可切到全书"},
		{modKey + "N", "在这一章后面新建章节"},
		{modKey + "PgDn / PgUp", "下一章 / 上一章"},
		{"F11", "专注写作：只留正文"},
		{modKey + "J", "在光标处续写一句"},
		{"Tab / Esc", "采纳 / 放弃续写"},
		{modKey + "L", "把选中的原文拿去讨论"},
		{modKey + "Z / " + modKey + "Y", "撤销 / 重做"},
		{modKey + "Backspace / " + modKey + "Delete", "向前 / 向后删到词边"},
		{modKey + "滚轮 · " + modKey + "= / " + modKey + "- / " + modKey + "0", "正文字号大一号 / 小一号 / 复原"},
	}},
	{"讨论", [][2]string{
		{modKey + "Enter", "发送"},
	}},
	{"任何地方", [][2]string{
		{modKey + "/", "打开这张表"},
	}},
}

// keysHelp lists the shortcuts.
func (a *App) keysHelp(c *ui.Context) {
	t := a.pal.theme
	ui.Modal(c, &a.keysOpen, func() {
		ui.Column(c).Width(440).Gap(14).Children(func() {
			ui.Text(c, "快捷键").FontSize(fsDialog).FontWeight(600)
			for _, g := range keyGroups {
				ui.Column(c).FillWidth().Gap(6).Children(func() {
					ui.Text(c, g.Where).FontSize(fsNote).FontWeight(600).TextColor(t.TextMuted)
					for _, k := range g.Keys {
						ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
							ui.Text(c, k[0]).FontSize(fsNote).FontWeight(600).Width(170).Shrink(0)
							ui.Text(c, k[1]).FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1)
						})
					}
				})
			}
			ui.Row(c).FillWidth().Children(func() {
				ui.Spacer(c)
				if ghost(c, "关闭", ui.Color{}).Clicked() {
					a.keysOpen = false
				}
			})
		})
	})
}
