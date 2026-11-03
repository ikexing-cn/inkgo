package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"

	"writer/locales"
)

// aiOn tells whether the AI is on on this machine. Off, nothing of it
// shows, and nothing is sent.
func (a *App) aiOn() bool { return !a.shelf.NoAI }

// loadAI reads the services configured, as the app starts with the AI on
// or the AI comes back on.
func (a *App) loadAI() {
	if a.ai.ConfigFile() != "" {
		if err := a.ai.Reload(); err != nil {
			a.SetError(err)
		}
	}
	if ps := a.ai.Providers(); len(ps) > 0 && a.serviceForm.providerID == "" {
		a.chooseProvider(ps[0].ID)
	}
}

// aiView is the settings' AI: the switch for every work on this machine,
// and with the AI on, the models.
func (a *App) aiView(c *ui.Context) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		off := !a.aiOn() // the box shows the switch; it changes once asked
		if ui.Checkbox(c, &off, locales.SettingsCloseAI()).Changed() {
			a.settings.askAI = true
		}
		a.pal.TickNote(c, locales.SettingsAIOffNote())
	})
	if a.aiOn() {
		a.modelsView(c)
	}
}

// askAI asks, over the settings, before the AI is turned off or on. Esc
// and the shade put the question away, the settings staying.
func (a *App) askAI(c *ui.Context) {
	off := a.aiOn() // what is asked is to turn it off
	title, about, verb := locales.SettingsAskOnTitle(), locales.SettingsAskOnAbout(), locales.SettingsTurnOn()
	if off {
		title, about, verb = locales.SettingsAskOffTitle(), locales.SettingsAskOffAbout(), locales.SettingsCloseAI()
	}
	turn := false
	a.pal.Modal(c, &a.settings.askAI, func() {
		ui.Column(c).Width(kit.DialogAsk).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.pal.DialogHead(c, title, about)
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if a.pal.Secondary(c, locales.CommonCancel()).Clicked() {
					a.settings.askAI = false
				}
				if a.pal.Primary(c, verb).Label(map[bool]string{true: "确定关闭 AI", false: "确定开启 AI"}[off]).Clicked() {
					turn = true
				}
			})
		})
	})
	if turn {
		a.settings.askAI = false
		a.turnAI(!off)
	}
}

// turnAI turns the AI on or off on this machine, and keeps that in the
// shelf. Off, what the AI is doing stops: what it proposed in the prose
// and was not taken is let go, a reply is let go as 取消请求 lets it go, the
// author's words kept, and the checks waiting are dropped; the pages and the parts of
// the AI give way to the prose and the setting, and the toasts go, most
// of them the AI's. Nothing it left is deleted: on again, it is all back,
// the talk beside the prose too when it was there, and the talk or check
// page it was turned off on while the prose still shows.
func (a *App) turnAI(on bool) {
	a.shelf.NoAI = !on
	a.saveShelf()
	if on {
		a.loadAI()
		if a.offAway && a.page == pageWrite {
			a.page = a.offFrom
		}
		a.offAway = false
	} else {
		for _, off := range a.aiOff {
			off()
		}
		if a.Store != nil && a.pages[a.page].ai {
			a.offFrom, a.offAway, a.page = a.page, true, pageWrite
		}
	}
	for _, ed := range a.write.editors() {
		ed.SetNoAI(!on)
	}
}
