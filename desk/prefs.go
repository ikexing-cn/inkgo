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
		if a.bk.prefs {
			color = a.pal.ink
		}
		b := iconButton(c, icGear, "设置", color).Tooltip("设置")
		if a.bk.prefs {
			b.Background(a.pal.theme.SurfacePressed)
		}
		if b.Clicked() {
			a.openPrefs(a.bk.tab)
		}
	})
}

// prefTabs are the settings' parts, with the labels that pick them.
var prefTabs = [][2]string{{"模型", "模型设置"}, {"外观", "设置-外观"}, {"备份", "设置-备份"}}

// The settings dialog keeps one size whichever part is shown.
const prefsWidth, prefsHeight = 880, 660

// openPrefs opens the settings at the part named, or the models.
func (a *App) openPrefs(tab string) {
	if tab == "" {
		tab = "模型"
	}
	a.bk.prefs, a.bk.tab = true, tab
}

// preferences is the settings dialog: the models, the appearance, and
// the backups, over whatever page it was opened on.
func (a *App) preferences(c *ui.Context) {
	if !a.bk.prefs {
		return
	}
	t := a.pal.theme
	if a.bk.tab == "" || a.bk.tab == "备份" && a.Store == nil {
		a.bk.tab = "模型"
	}
	w, h := c.Size()
	ui.DialogBase(c, &a.bk.prefs, func(back, panel *ui.Element) {
		back.Background(a.pal.scrim)
		panel.Row().AlignItems(ui.Stretch).Width(min(prefsWidth, w-40)).Height(min(prefsHeight, h-40)).Radius(rDialog).Background(t.Background)
		a.pal.dialog.on(panel)
		ui.Column(c).Width(176).FillHeight().Shrink(0).Padding(20, 10).Gap(2).Radius(rDialog, 0, 0, rDialog).Background(a.pal.side).Children(func() {
			ui.Text(c, "设置").FontSize(fsSmall).FontWeight(600).TextColor(a.pal.faint).Padding(0, 10, 8, 10)
			for _, tl := range prefTabs {
				tab := tl[0]
				if tab == "备份" && a.Store == nil {
					continue
				}
				b := ui.ButtonBase(c).Row().FillWidth().Height(hControl).Justify(ui.Start).AlignItems(ui.Center).Padding(0, 10).Radius(rControl).Label(tl[1])
				on := a.bk.tab == tab
				fg, weight := t.TextMuted, 400
				if on {
					b.Background(t.SurfacePressed)
					fg, weight = t.Text, 600
				} else if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				b.Children(func() { ui.Text(c, tab).FontSize(fsBody).FontWeight(weight).TextColor(fg) })
				if b.Clicked() {
					a.bk.tab = tab
				}
			}
		})
		ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
		ui.Scroll(c).Key("prefs-" + a.bk.tab).Grow(1).Shrink(1).FillHeight().Children(func() {
			ui.Column(c).FillWidth().Padding(32, 40, 32, 40).Gap(24).Children(func() {
				switch a.bk.tab {
				case "外观":
					a.looksView(c)
				case "备份":
					a.backupSettingsView(c)
				default:
					a.modelsView(c)
				}
			})
		})
		if iconButton(c, icX, "关闭设置", ui.Color{}).Absolute().Top(12).Right(12).Clicked() {
			a.bk.prefs = false
		}
	})
}

// prefsHead is the title over a part of the settings.
func (a *App) prefsHead(c *ui.Context, title, caption string) {
	ui.Column(c).FillWidth().Gap(6).Padding(0, 32, 0, 0).Children(func() {
		ui.Text(c, title).FontSize(fsTitle).FontWeight(700)
		ui.Text(c, caption).FontSize(fsBody).TextColor(a.pal.theme.TextMuted).LineHeight(lhBody)
	})
}

// prefsRow is a labeled row of the settings, between hairlines.
func (a *App) prefsRow(c *ui.Context, label string, fn func()) {
	ui.Row(c).FillWidth().Gap(24).Padding(14, 0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(fsBody).FontWeight(500).Width(150).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).Children(fn)
	})
	a.pal.hairline(c)
}

// looksView is the settings' 外观.
func (a *App) looksView(c *ui.Context) {
	a.prefsHead(c, "外观", "只在这台电脑上生效，所有作品都一样。")
	ui.Column(c).FillWidth().Children(func() {
		a.pal.hairline(c)
		a.prefsRow(c, "颜色", func() {
			ui.Column(c).Width(320).Children(func() { a.themeSegments(c) })
		})
	})
}
