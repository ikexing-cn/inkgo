package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"

	"writer/locales"
)

// settingsView belongs to the settings window, independently of extension tasks.
// Closing keeps the selected tab; leaving the work resets it.
type settingsView struct {
	open  bool
	tab   string
	askAI bool
}

// The settings dialog keeps one size whichever part is shown.
const prefsWidth, prefsHeight = 900, 620

// prefsGap separates the rows of a part of the settings, an extension's too.
const prefsGap = 12

// prefAbout says under a part's title what the author must know of it;
// a part with nothing such says nothing there.
var prefAbout = map[string]func() string{
	"外观":    locales.SettingsLocalOnly,
	"提醒与检查": locales.SettingsBookOnly,
}

// prefName is what a part of the settings is called, and prefNote what
// it says it is for: with the AI off, 提醒 has no checks, and the AI
// holds the switch alone.
func (a *App) prefName(tab string) string {
	switch tab {
	case "外观":
		return locales.ViewLook()
	}
	for _, entry := range a.prefTabs {
		if entry.id == tab {
			return entry.title(a)
		}
	}
	return tab
}

// prefTitle heads the part of the settings shown: its name, or the AI
// record's, which shows in the AI's part.
func (a *App) prefTitle(tab string) string {
	if tab == "AI" && a.showsLog() {
		return locales.SettingsRecordsTitle()
	}
	return a.prefName(tab)
}

func (a *App) prefNote(tab string) string {
	switch {
	case tab == "AI" && !a.aiOn():
		return locales.SettingsLocalOnly()
	case tab == "AI" && a.showsLog():
		return locales.SettingsRecordsNote()
	}
	if say := prefAbout[tab]; say != nil {
		return say()
	}
	return ""
}

// showsLog is whether the AI's part of the settings shows the AI record.
func (a *App) showsLog() bool { return a.record.showing && a.aiOn() && a.Store != nil }

// openPrefs opens the settings at the part named, or the AI.
func (a *App) openPrefs(tab string) {
	if tab == "" {
		tab = "AI"
	}
	a.settings.open = true
	a.showPref(tab)
}

// closePrefs also drops an unanswered AI-switch question. It keeps this
// work's selected tab and service draft for reopening the settings.
func (a *App) closePrefs() {
	a.settings.open, a.settings.askAI = false, false
}

// resetPrefs releases state belonging to the work being left. Saved model
// settings and AI records stay on disk; only their views and drafts go.
func (a *App) resetPrefs() {
	a.settings = settingsView{}
	a.record.reset()
	a.serviceForm.close()
}

// showPref shows a part of the settings. The reminders count the book's
// foreshadowings from the tree and the marks Book read, from any page. The
// AI's part reads the AI record, and shows what it adds up to.
func (a *App) showPref(tab string) {
	a.settings.tab, a.record.showing = tab, false
	if tab == "AI" && a.Store != nil {
		records, err := a.Store.Records()
		a.warn(err)
		a.record.load(records)
	}
}

// preferences is the settings dialog: the AI, the appearance, the
// reminders and extension pages, over whatever page it was opened on; the
// shelf has the first two, which hold for every work.
func (a *App) preferences(c *ui.Context) {
	a.syncExtensions()
	if !a.settings.open {
		a.closePrefs()
		return
	}
	t := a.pal.Theme
	a.settings.tab = a.prefTabs.shown(a, a.settings.tab)
	w, h := c.Size()
	ui.DialogBase(c, &a.settings.open, func(back, panel ui.Element) {
		scrim := ui.Color{}
		// What is asked over the settings shades the window, them with it:
		// one shade, not two.
		if !a.settings.askAI && !a.extensionModalOpen() {
			scrim = a.pal.Scrim
		}
		kit.DialogIn(back, panel, scrim)
		panel.Row().AlignItems(ui.Stretch).Width(min(prefsWidth, w-40)).Height(min(prefsHeight, h-40)).Radius(kit.RDialog).Background(t.Background)
		a.pal.Dialog.On(panel)
		ui.Column(c).Width(180).FillHeight().Shrink(0).Padding(20, 10).Gap(2).Radius(kit.RDialog, 0, 0, kit.RDialog).Background(a.pal.Canvas).Children(func() {
			ui.Text(c, locales.SettingsTitle()).FontSize(kit.FsSmall).FontWeight(600).TextColor(a.pal.Faint).Padding(0, 10, 8, 10)
			for _, entry := range a.prefTabs {
				tab := entry.id
				if !entry.available(a) {
					continue
				}
				b := ui.ButtonBase(c).Row().FillWidth().Height(kit.HControl).Justify(ui.Start).AlignItems(ui.Center).Padding(0, 10).Radius(kit.RControl).Label(entry.label)
				on := a.settings.tab == tab
				fg, weight := t.TextMuted, 400
				if on {
					b.Background(t.SurfacePressed)
					fg, weight = t.Text, 600
				} else if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				kit.PressRow(c, b)
				b.Children(func() {
					ui.Text(c, a.prefName(tab)).FontSize(kit.FsBody).FontWeight(weight).TextColor(fg)
					if n := entry.number(); n > 0 {
						ui.Textf(c, "%d", n).FontSize(kit.FsNote).TextColor(a.pal.Faint)
					}
				})
				if b.Clicked() {
					a.showPref(tab)
				}
			}
		})
		ui.Box(c).Width(1).FillHeight().Background(a.pal.Line)
		ui.Column(c).Grow(1).Shrink(1).FillHeight().Children(func() {
			// The title stays where it is: what is under it scrolls up to
			// the line, not over it and the close button.
			ui.Column(c).FillWidth().Shrink(0).Gap(4).Padding(24, 56, 16, 32).Children(func() {
				ui.Text(c, a.prefTitle(a.settings.tab)).FontSize(kit.FsDialog).FontWeight(700)
				if note := a.prefNote(a.settings.tab); note != "" {
					ui.Text(c, note).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
				}
			})
			a.pal.Hairline(c)
			if a.showsLog() {
				a.recordsView(c)
			} else {
				ui.Scroll(c.Key("prefs-" + a.settings.tab)).Grow(1).Shrink(1).FillWidth().Children(func() {
					ui.Column(c).FillWidth().Padding(12, 56, 32, 32).Gap(prefsGap).Children(func() {
						switch a.settings.tab {
						case "外观":
							a.looksView(c)
						case "提醒与检查":
							a.remindersView(c)
						case "AI":
							a.aiView(c)
						default:
							a.extensionSettings(c, a.settings.tab)
						}
					})
				})
			}
		})
		if kit.IconButton(c, kit.IcX, locales.SettingsClose(), ui.Color{}).Absolute().Top(12).Right(12).Clicked() {
			a.closePrefs()
		}
	})
	if a.settings.open {
		a.askAI(c) // over the settings
	} else {
		a.closePrefs()
	}
}

// prefsRow is a labeled row of the settings.
func (a *App) prefsRow(c *ui.Context, label string, fn func()) {
	ui.Row(c).FillWidth().MinHeight(kit.HControl).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(kit.FsBody).FontWeight(500).Width(120).Shrink(0)
		fn()
	})
}

// looksView is the settings' 外观.
func (a *App) looksView(c *ui.Context) {
	a.prefsRow(c, locales.SettingsColor(), func() { a.themeSegments(c, false) })
	on := !a.shelf.NoNames
	if ui.Checkbox(c, &on, locales.SettingsMarkNames()).Changed() {
		a.shelf.NoNames = !on
		a.saveShelf()
	}
	if offerNoGPU {
		a.gpuSwitch(c)
	}
}

// offerNoGPU offers 禁用 GPU 加速 and draws by it. It was off until mygo
// v0.6.4: drawing without the GPU on Windows mostly left the window white,
// and whoever ticked it could not see to untick it. Off, the option hides
// and every machine draws on the GPU again.
var offerNoGPU = true

// gpuSwitch is 禁用 GPU 加速. The window is drawn as the software opened: a
// change waits for it to open again, which the link does at once.
func (a *App) gpuSwitch(c *ui.Context) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		if ui.Checkbox(c, &a.shelf.NoGPU, locales.SettingsDisableGPU()).Changed() {
			a.saveShelf()
		}
		if a.shelf.NoGPU == a.ranNoGPU {
			return
		}
		link := locales.SettingsRestartNow()
		if a.Relaunch == nil {
			link = ""
		}
		if a.pal.TickNoteLink(c, locales.SettingsGPUNext(), link) {
			a.relaunch()
		}
	})
}

// NoGPU reports whether this machine draws the window without the GPU;
// cmd/inkgo asks before the window draws.
func (a *App) NoGPU() bool { return offerNoGPU && a.shelf.NoGPU }

// relaunch opens the software again, on the same work, from the
// settings, which close first: what the AI proposed and is not decided
// on is asked about over the page, as closing the window asks.
func (a *App) relaunch() {
	a.closePrefs()
	if !a.Leave(a.Relaunch) {
		a.Relaunch()
	}
}
