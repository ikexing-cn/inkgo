package desk

import (
	"github.com/egoist/mygo/ui"
)

func (a *App) welcome(c *ui.Context) {
	t := a.pal.theme
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(56, 24).Gap(28).Children(func() {
			ui.Column(c).Width(560).Gap(10).Children(func() {
				ui.Text(c, "从一个念头开始").FontSize(30).Bold()
				ui.Text(c, "和 AI 慢慢聊，讨论会在后台整理成设定文档。你定下的会被遵守，放下的 AI 不再提起。").FontSize(15).TextColor(t.TextMuted).LineHeight(1.6)
			})
			a.pal.cardBox(c).Width(560).Padding(24).Gap(16).Children(func() {
				ui.Segmented(c, &a.welcomeMode, "新建作品", "打开已有作品").Label("开始方式")
				label := "存放作品的空文件夹"
				if a.welcomeMode == 1 {
					label = "已有作品的文件夹"
				}
				ui.Column(c).FillWidth().Gap(6).Children(func() {
					ui.Text(c, label).FontSize(13).TextColor(t.TextMuted)
					ui.Row(c).FillWidth().Gap(8).Children(func() {
						ui.TextInput(c, &a.path).Label("作品文件夹").Placeholder(`例如 D:\写作\雨城来信`).Grow(1)
						if a.PickFolder != nil && ui.Button(c, "浏览…").Clicked() {
							a.browse(c, label, &a.path)
						}
					})
				})
				if a.welcomeMode == 0 {
					ui.Row(c).FillWidth().Gap(12).Children(func() {
						ui.Column(c).Grow(1).Gap(6).Children(func() {
							ui.Text(c, "作品名称").FontSize(13).TextColor(t.TextMuted)
							ui.TextInput(c, &a.name).Label("作品名称").Placeholder("可以之后再改").FillWidth()
						})
						ui.Column(c).Width(150).Gap(6).Children(func() {
							ui.Text(c, "体裁").FontSize(13).TextColor(t.TextMuted)
							ui.Select(c, &a.genre, []string{"网文", "出版小说", "剧本", "非虚构"}).Label("体裁").FillWidth()
						})
					})
				}
				ui.Row(c).FillWidth().Children(func() {
					ui.Spacer(c)
					if a.welcomeMode == 0 {
						if ui.PrimaryButton(c, "新建作品").Disabled(trim(a.path) == "").Clicked() {
							a.setError(a.create())
						}
					} else if ui.PrimaryButton(c, "打开作品").Disabled(trim(a.path) == "").Clicked() {
						a.setError(a.Open(trim(a.path)))
					}
				})
			})
			ui.Column(c).Width(560).Gap(6).Children(func() {
				for _, line := range []string{
					"作品、对话和设定文档都是你文件夹里的普通文件，可以备份、同步或用别的编辑器打开。",
					"AI 只在你发送时回应，不会自己往下写。",
					"API key 存在系统钥匙串里，不会写进作品文件夹。",
				} {
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, "·").TextColor(t.Accent).Bold()
						ui.Text(c, line).FontSize(13).TextColor(t.TextMuted).Grow(1)
					})
				}
			})
		})
	})
}
