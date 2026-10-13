package desk

import "github.com/egoist/mygo/ui"

// keyGroups are the shortcuts the help lists, by where they work.
var keyGroups = []struct {
	Where string
	Keys  [][2]string
}{
	{"正文", [][2]string{
		{modKey + "S", "立即保存（停笔片刻也会自动保存）"},
		{modKey + "F", "查找替换，可切换到全书"},
		{modKey + "N", "在本章之后新建章节"},
		{"F2", "重命名当前章节"},
		{modKey + "PgDn / PgUp", "下一章 / 上一章"},
		{"Ctrl+Tab / Ctrl+Shift+Tab", "下一个 / 上一个标签页"},
		{modKey + "W", "关闭当前标签页"},
		{"F11", "专注写作：仅显示正文"},
		{modKey + "J", "在光标处续写一句"},
		{"Tab / Esc", "采纳 / 放弃续写"},
		{modKey + "L", "将选中的原文引用到讨论"},
		{modKey + "Z / " + modKey + "Y", "撤销 / 重做"},
		{modKey + "Backspace / " + modKey + "Delete", "向前 / 向后删除一个词"},
		{modKey + "滚轮 · " + modKey + "= / " + modKey + "- / " + modKey + "0", "正文字号增大 / 减小 / 复原"},
	}},
	{"讨论", [][2]string{
		{modKey + "Enter", "发送"},
	}},
	{"全局", [][2]string{
		{modKey + "/", "打开快捷键列表"},
	}},
}

// keysHelp lists the shortcuts.
func (a *App) keysHelp(c *ui.Context) {
	t := a.pal.theme
	a.modal(c, &a.keysOpen, func() {
		ui.Column(c).Width(dialogKeys).Padding(dialogPad).Gap(16).Children(func() {
			a.pal.dialogHead(c, "快捷键", "")
			ui.Scroll(c).Key("keys").FillWidth().MaxHeight(440).Shrink(1).Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					for _, g := range keyGroups {
						ui.Text(c, g.Where).FontSize(fsSmall).TextColor(a.pal.faint).Padding(14, 0, 4, 0)
						for _, k := range g.Keys {
							ui.Row(c).FillWidth().Gap(12).Padding(6, 0).Children(func() {
								ui.Text(c, k[0]).FontSize(fsControl).TextColor(t.TextMuted).Width(250).Shrink(0)
								ui.Text(c, k[1]).FontSize(fsControl).Grow(1).Shrink(1)
							})
						}
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
