package desk

import "github.com/egoist/mygo/ui"

// sideFoot is the bottom of the sidebar: how the backups go, the trash
// on the writing page, and the gear to the settings.
func (a *App) sideFoot(c *ui.Context, slim bool) {
	if a.Store == nil {
		return
	}
	if !slim {
		a.backupLine(c)
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(4).Padding(0, 4, 0, 0).Shrink(0).Children(func() {
		if !slim {
			a.trashRow(c)
		}
		color := ui.Color{}
		if a.page == 4 {
			color = a.pal.ink
		}
		b := iconButton(c, icGear, "设置", color).Tooltip("设置")
		if a.page == 4 {
			b.Background(a.pal.theme.SurfacePressed)
		}
		if b.Clicked() {
			a.page = 4
		}
	})
}

// prefTabs are the settings page's parts, with the labels that pick them.
var prefTabs = [][2]string{{"模型", "模型设置"}, {"外观", "设置-外观"}, {"备份", "设置-备份"}}

// preferences is the settings page: the models, the appearance, and the
// backups.
func (a *App) preferences(c *ui.Context) {
	t := a.pal.theme
	if a.bk.tab == "" {
		a.bk.tab = "模型"
	}
	ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(232).FillHeight().Padding(titleHeight-20, 10, 20, 10).Gap(2).Background(a.pal.side).Shrink(0).Children(func() {
			ui.Text(c, "设置").FontSize(12).TextColor(t.TextMuted).Padding(0, 10, 8, 10)
			for _, tl := range prefTabs {
				tab := tl[0]
				b := ui.ButtonBase(c).Row().FillWidth().Height(32).Justify(ui.Start).AlignItems(ui.Center).Padding(0, 10).Radius(6).Label(tl[1])
				on := a.bk.tab == tab
				fg, weight := t.TextMuted, 400
				if on {
					b.Background(t.SurfacePressed)
					fg, weight = t.Text, 600
				} else if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				b.Children(func() { ui.Text(c, tab).FontSize(13.5).FontWeight(weight).TextColor(fg) })
				if b.Clicked() {
					a.bk.tab = tab
				}
			}
			if a.bk.tab == "模型" {
				ui.Box(c).Height(18)
				a.modelNav(c)
			}
		})
		ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
		if a.bk.tab == "模型" {
			a.modelPane(c)
			return
		}
		ui.Scroll(c).Key("preferences").Grow(1).Shrink(1).FillHeight().Children(func() {
			ui.Column(c).FillWidth().MaxWidth(820).Padding(titleHeight+20, 56, 40, 56).Gap(28).Children(func() {
				if a.bk.tab == "外观" {
					ui.Column(c).FillWidth().Gap(6).Children(func() {
						ui.Text(c, "外观").FontSize(26).FontWeight(700)
						ui.Text(c, "跟随系统，或者一直用浅色、深色。只在这台电脑上生效，所有作品都一样。").FontSize(13.5).TextColor(t.TextMuted)
					})
					ui.Column(c).FillWidth().Children(func() {
						a.pal.hairline(c)
						ui.Row(c).FillWidth().Gap(24).Padding(16, 0).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "颜色").FontSize(14).FontWeight(500).Width(180).Shrink(0)
							ui.Column(c).Width(320).Children(func() { a.themeSegments(c) })
						})
						a.pal.hairline(c)
					})
					return
				}
				a.backupSettingsView(c)
			})
		})
	})
}
