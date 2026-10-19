package desk

import "github.com/egoist/mygo/ui"

// sideFoot is the bottom of the sidebar: how the backups go, the trash,
// and the gear to the settings; slim, the trash is an icon over the gear.
func (a *App) sideFoot(c *ui.Context, slim bool) {
	if a.Store == nil {
		return
	}
	gear := func() {
		color := ui.Color{}
		if a.bk.prefs {
			color = a.pal.ink
		}
		b := iconButton(c, icGear, "设置", color).Tooltip("设置").KeepFocus()
		if a.bk.prefs {
			b.Background(a.pal.theme.SurfacePressed)
		}
		if b.Clicked() {
			a.openPrefs(a.bk.tab)
		}
	}
	if slim {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(2).Shrink(0).Children(func() {
			if iconButton(c, icTrash, "回收站", ui.Color{}).Tooltip("回收站").KeepFocus().Clicked() {
				a.openTrash()
			}
			gear()
		})
		return
	}
	a.backupLine(c)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(4).Padding(0, 4, 0, 0).Shrink(0).Children(func() {
		a.trashRow(c)
		gear()
	})
}

// prefTabs are the settings' parts, with the labels that pick them.
var prefTabs = [][2]string{{"AI", "设置-AI"}, {"外观", "设置-外观"}, {"提醒与检查", "设置-提醒"}, {"备份", "设置-备份"}}

// The settings dialog keeps one size whichever part is shown.
const prefsWidth, prefsHeight = 900, 620

// prefAbout says under each part's title what it is for.
var prefAbout = map[string]string{
	"AI":    "是否使用 AI，以及使用哪些模型。",
	"外观":    "仅在本机生效，适用于所有作品。",
	"提醒与检查": "本作品的提醒规则，修改后立即生效。",
	"备份":    "为整部作品另存一份副本，以防设备故障。",
}

// prefName is what a part of the settings is called, and prefNote what
// it says it is for: with the AI off, 提醒与检查 has no checks, and the
// AI holds the switch alone.
func (a *App) prefName(tab string) string {
	if tab == "提醒与检查" && !a.aiOn() {
		return "提醒"
	}
	return tab
}

// prefTitle heads the part of the settings shown: its name, or the AI
// record's, which shows in the AI's part.
func (a *App) prefTitle(tab string) string {
	if tab == "AI" && a.showsLog() {
		return "AI 记录"
	}
	return a.prefName(tab)
}

func (a *App) prefNote(tab string) string {
	switch {
	case tab == "AI" && !a.aiOn():
		return "仅在本机生效，适用于所有作品。"
	case tab == "AI" && a.showsLog():
		return "本作品每次向模型发的请求记一行，失败、取消的也记；只记发了哪几部分、各多大，不存内容。"
	}
	return prefAbout[tab]
}

// showsLog is whether the AI's part of the settings shows the AI record.
func (a *App) showsLog() bool { return a.bk.log && a.aiOn() && a.Store != nil }

// openPrefs opens the settings at the part named, or the AI.
func (a *App) openPrefs(tab string) {
	if tab == "" {
		tab = "AI"
	}
	a.bk.prefs = true
	a.showPref(tab)
}

// showPref shows a part of the settings. The reminders count the book's
// foreshadowings from the tree and the marks the writing page reads as it
// is entered: shown from another page, they are read here. The AI's part
// reads the AI record, and shows what it adds up to.
func (a *App) showPref(tab string) {
	a.bk.tab, a.bk.log = tab, false
	if tab == "AI" && a.Store != nil {
		records, err := a.Store.Records()
		a.warn(err)
		a.bk.records = records
	}
	if tab == "提醒与检查" && a.Store != nil && !a.write.entered {
		a.warn(a.loadTree())
		a.scanMarks()
	}
}

// preferences is the settings dialog: the AI, the appearance, the
// reminders and the backups, over whatever page it was opened on; the
// shelf has the first two, which hold for every work.
func (a *App) preferences(c *ui.Context) {
	if !a.bk.prefs {
		return
	}
	t := a.pal.theme
	if a.bk.tab == "" || (a.bk.tab == "备份" || a.bk.tab == "提醒与检查") && a.Store == nil {
		a.bk.tab = "AI"
	}
	w, h := c.Size()
	ui.DialogBase(c, &a.bk.prefs, func(back, panel ui.Element) {
		// What is asked over the settings shades the window, them with it:
		// one shade, not two.
		if !a.bk.askAI && !(a.bk.ask && a.Store != nil) {
			back.Background(a.pal.scrim)
		}
		panel.Row().AlignItems(ui.Stretch).Width(min(prefsWidth, w-40)).Height(min(prefsHeight, h-40)).Radius(rDialog).Background(t.Background)
		a.pal.dialog.on(panel)
		ui.Column(c).Width(180).FillHeight().Shrink(0).Padding(20, 10).Gap(2).Radius(rDialog, 0, 0, rDialog).Background(a.pal.side).Children(func() {
			ui.Text(c, "设置").FontSize(fsSmall).FontWeight(600).TextColor(a.pal.faint).Padding(0, 10, 8, 10)
			for _, tl := range prefTabs {
				tab := tl[0]
				if (tab == "备份" || tab == "提醒与检查") && a.Store == nil {
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
				b.Children(func() { ui.Text(c, a.prefName(tab)).FontSize(fsBody).FontWeight(weight).TextColor(fg) })
				if b.Clicked() {
					a.showPref(tab)
				}
			}
		})
		ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
		ui.Column(c).Grow(1).Shrink(1).FillHeight().Children(func() {
			// The title stays where it is: what is under it scrolls up to
			// the line, not over it and the close button.
			ui.Column(c).FillWidth().Shrink(0).Gap(4).Padding(24, 56, 16, 32).Children(func() {
				ui.Text(c, a.prefTitle(a.bk.tab)).FontSize(fsDialog).FontWeight(700)
				ui.Text(c, a.prefNote(a.bk.tab)).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody)
			})
			a.pal.hairline(c)
			key := "prefs-" + a.bk.tab
			if a.showsLog() {
				key = "prefs-records"
			}
			ui.Scroll(c.Key(key)).Grow(1).Shrink(1).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(12, 56, 32, 32).Gap(12).Children(func() {
					switch a.bk.tab {
					case "外观":
						a.looksView(c)
					case "备份":
						a.backupSettingsView(c)
					case "提醒与检查":
						a.remindersView(c)
					default:
						a.aiView(c)
					}
				})
			})
		})
		if iconButton(c, icX, "关闭设置", ui.Color{}).Absolute().Top(12).Right(12).Clicked() {
			a.bk.prefs = false
		}
	})
	if a.bk.prefs {
		a.askAI(c) // over the settings
	} else {
		a.bk.askAI = false
	}
}

// prefsRow is a labeled row of the settings.
func (a *App) prefsRow(c *ui.Context, label string, fn func()) {
	ui.Row(c).FillWidth().MinHeight(hControl).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(fsBody).FontWeight(500).Width(110).Shrink(0)
		fn()
	})
}

// looksView is the settings' 外观.
func (a *App) looksView(c *ui.Context) {
	a.prefsRow(c, "颜色", func() { a.themeSegments(c, false) })
	on := !a.shelf.NoNames
	if ui.Checkbox(c, &on, "正文中标出设定的名字").Changed() {
		a.shelf.NoNames = !on
		a.saveShelf()
	}
	a.tickNote(c, "名字下面一道虚线，点击在右栏查看这条设定。")
}
